package transaction

import (
	"context"
	"fmt"
	"log"
	"time"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"github.com/hogyzen12/squads-go/generated/squads_multisig_program"
	"github.com/hogyzen12/squads-go/pkg/multisig"
)

// ProposalExecuteOutput defines return values from executing a proposal
type ProposalExecuteOutput struct {
	Signature        string
	TransactionPDA   solana.PublicKey
	ProposalPDA      solana.PublicKey
	TransactionIndex uint64
}

// ExecuteProposal executes an approved proposal that has passed its timelock
func ExecuteProposal(ctx context.Context,
	multisigPDA solana.PublicKey,
	transactionIndex uint64,
	executor solana.PrivateKey,
	client *rpc.Client,
	wsClient *ws.Client) (*ProposalExecuteOutput, error) {

	log.Println("Executing approved proposal...")

	// Create clients if not provided
	if client == nil {
		client = rpc.New("https://api.mainnet-beta.solana.com")
	}

	var wsClientCreated bool
	if wsClient == nil {
		var err error
		wsClient, err = ws.Connect(ctx, "wss://api.mainnet-beta.solana.com")
		if err != nil {
			return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		wsClientCreated = true
		defer func() {
			if wsClientCreated {
				wsClient.Close()
			}
		}()
	}

	// Calculate transaction and proposal PDAs
	txPDA, _ := multisig.GetTransactionPDA(multisigPDA, transactionIndex)
	proposalPDA, _ := multisig.GetProposalPDA(multisigPDA, transactionIndex)

	// Fetch the multisig account
	multisigAccount, err := fetchMultisigAccount(client, multisigPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch multisig account: %w", err)
	}

	// Fetch the proposal account
	proposal, err := fetchProposalAccount(client, proposalPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch proposal: %w", err)
	}

	// Check if the proposal is approved
	_, isApproved := proposal.Status.(*squads_multisig_program.ProposalStatusApproved)
	if !isApproved {
		return nil, fmt.Errorf("proposal is not in approved state, current status: %s",
			getProposalStatusString(proposal.Status))
	}

	// Check if timelock has elapsed
	approvedStatus := proposal.Status.(*squads_multisig_program.ProposalStatusApproved)
	approvalTime := time.Unix(approvedStatus.Timestamp, 0)
	timelockEnd := approvalTime.Add(time.Duration(multisigAccount.TimeLock) * time.Second)

	if time.Now().Before(timelockEnd) && multisigAccount.TimeLock > 0 {
		return nil, fmt.Errorf("timelock has not elapsed yet. Executable after: %s",
			timelockEnd.Format("2006-01-02 15:04:05"))
	}

	// Check if the executor has execute permission
	hasExecutePermission := false
	for _, member := range multisigAccount.Members {
		if member.Key.Equals(executor.PublicKey()) {
			if member.Permissions.Mask&4 != 0 { // 4 is the permission to execute
				hasExecutePermission = true
				break
			}
		}
	}

	if !hasExecutePermission {
		return nil, fmt.Errorf("executor %s does not have execute permission", executor.PublicKey())
	}

	// Fetch the transaction account
	txAccountInfo, err := client.GetAccountInfo(ctx, txPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction account: %w", err)
	}
	if txAccountInfo.Value == nil || len(txAccountInfo.Value.Data.GetBinary()) < 8 {
		return nil, fmt.Errorf("transaction account not found or has invalid data: %s", txPDA)
	}

	// Decode the vault transaction
	var vaultTx squads_multisig_program.VaultTransaction
	decoder := ag_binary.NewBorshDecoder(txAccountInfo.Value.Data.GetBinary())
	err = vaultTx.UnmarshalWithDecoder(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vault transaction: %w", err)
	}

	// Log the transaction message for debugging
	log.Printf("Vault Transaction Message: %+v", vaultTx.Message)

	// Check if there are instructions
	if len(vaultTx.Message.Instructions) == 0 {
		return nil, fmt.Errorf("transaction has no instructions and cannot be executed")
	}

	// Extract additional accounts dynamically
	additionalAccounts := vaultTx.Message.AccountKeys
	log.Printf("Additional accounts required: %v", additionalAccounts)

	// Build the VaultTransactionExecute instruction with base accounts
	executeInstruction := squads_multisig_program.NewVaultTransactionExecuteInstructionBuilder().
		SetMultisigAccount(multisigPDA).
		SetProposalAccount(proposalPDA).
		SetTransactionAccount(txPDA).
		SetMemberAccount(executor.PublicKey())

	// Append additional accounts with dynamic properties
	for i, accountKey := range additionalAccounts {
		isWritable := false
		// Determine if the account is writable based on the message structure
		if i < int(vaultTx.Message.NumWritableSigners) {
			isWritable = true // Writable signer
		} else if (i - int(vaultTx.Message.NumSigners)) < int(vaultTx.Message.NumWritableNonSigners) {
			isWritable = true // Writable non-signer
		}
		// Additional accounts are typically not signers in multisig execution
		executeInstruction.AccountMetaSlice = append(executeInstruction.AccountMetaSlice,
			solana.NewAccountMeta(accountKey, isWritable, false))
	}

	executeIx := executeInstruction.Build()

	// Log transaction details
	log.Printf("Executing vault transaction #%d on multisig %s with %d additional accounts",
		transactionIndex, multisigPDA, len(additionalAccounts))
	log.Printf("Transaction PDA: %s", txPDA)
	log.Printf("Proposal PDA: %s", proposalPDA)

	// Get latest blockhash
	hash, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	// Create transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{executeIx},
		hash.Value.Blockhash,
		solana.TransactionPayer(executor.PublicKey()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution transaction: %w", err)
	}

	// Sign transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(executor.PublicKey()) {
				return &executor
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	sig, err := client.SendTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transaction: %w", err)
	}

	output := &ProposalExecuteOutput{
		Signature:        sig.String(),
		TransactionPDA:   txPDA,
		ProposalPDA:      proposalPDA,
		TransactionIndex: transactionIndex,
	}

	log.Printf("✓ Successfully submitted execution transaction: %s", sig)
	log.Printf("Transaction may take a few seconds to confirm.")

	return output, nil
}
