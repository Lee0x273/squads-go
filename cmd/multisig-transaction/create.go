package multisigtransaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	sendAndConfirmTransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/spf13/cobra"

	"squads-go/generated/squads_multisig_program"
	"squads-go/pkg/multisig"

	ag_binary "github.com/gagliardetto/binary"
)

// uint64ToLEBytes converts a uint64 to a little-endian byte slice
func uint64ToLEBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	bytes[0] = byte(value)
	bytes[1] = byte(value >> 8)
	bytes[2] = byte(value >> 16)
	bytes[3] = byte(value >> 24)
	bytes[4] = byte(value >> 32)
	bytes[5] = byte(value >> 40)
	bytes[6] = byte(value >> 48)
	bytes[7] = byte(value >> 56)
	return bytes
}

// createTransactionMessageBytes creates a byte array representing a transaction message for a transfer
func createTransactionMessageBytes(ix solana.Instruction, vaultPDA, recipientPubkey solana.PublicKey, lamports uint64) []byte {
	// Convert instruction accounts to a slice of PublicKeys
	accountKeys := []solana.PublicKey{
		vaultPDA,               // First account is the vault (signer, writable)
		recipientPubkey,        // Second account is the recipient (writable)
		solana.SystemProgramID, // Third account is the system program
	}

	// Create a VaultTransactionMessage
	txMessage := squads_multisig_program.VaultTransactionMessage{
		NumSigners:            1, // Only vault is a signer
		NumWritableSigners:    1, // Vault is writable
		NumWritableNonSigners: 1, // Recipient is writable non-signer
		AccountKeys:           accountKeys,
		Instructions: []squads_multisig_program.MultisigCompiledInstruction{
			{
				ProgramIdIndex: 2,                                                        // Index of SystemProgram in accountKeys
				AccountIndexes: []byte{0, 1},                                             // Indexes of Vault and Recipient
				Data:           append([]byte{2, 0, 0, 0}, uint64ToLEBytes(lamports)...), // Transfer instruction
			},
		},
		AddressTableLookups: []squads_multisig_program.MultisigMessageAddressTableLookup{}, // No lookups
	}

	// Serialize the message to bytes using borsh encoder
	buf := new(bytes.Buffer)
	err := ag_binary.NewBorshEncoder(buf).Encode(txMessage)
	if err != nil {
		log.Fatalf("Failed to encode transaction message: %v", err)
	}

	return buf.Bytes()
}

// fetchMultisigAccount fetches and decodes a multisig account
func fetchMultisigAccount(client *rpc.Client, multisigPDA solana.PublicKey) (*squads_multisig_program.Multisig, error) {
	accountInfo, err := client.GetAccountInfo(context.Background(), multisigPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to get multisig account: %w", err)
	}

	var multisigAccount squads_multisig_program.Multisig
	decoder := ag_binary.NewBorshDecoder(accountInfo.Value.Data.GetBinary())
	err = multisigAccount.UnmarshalWithDecoder(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to decode multisig account: %w", err)
	}

	return &multisigAccount, nil
}

// getAccountBalance fetches an account's SOL balance
func getAccountBalance(client *rpc.Client, pubkey solana.PublicKey) (uint64, error) {
	balance, err := client.GetBalance(context.Background(), pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return balance.Value, nil
}

// LoadKeypair loads a keypair from a JSON file
func LoadKeypair(path string) (solana.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keyArray []byte
	if err := json.Unmarshal(keyBytes, &keyArray); err != nil {
		return nil, err
	}
	return solana.PrivateKey(keyArray), nil
}

// runCreateTransaction handles the creation of a transaction for a Squads Multisig
func runCreateTransaction(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load RPC endpoints
	rpcEndpoint, _ := cmd.Parent().Parent().Flags().GetString("rpc")
	wsEndpoint, _ := cmd.Parent().Parent().Flags().GetString("ws")

	// Get flags
	multisigStr, _ := cmd.Flags().GetString("multisig")
	toStr, _ := cmd.Flags().GetString("to")
	amount, _ := cmd.Flags().GetFloat64("amount")
	payerPath, _ := cmd.Flags().GetString("payer")
	vaultIndex, _ := cmd.Flags().GetUint8("vault-index")
	memo, _ := cmd.Flags().GetString("memo")
	autoApprove, _ := cmd.Flags().GetBool("approve")
	timeoutSecs, _ := cmd.Flags().GetUint32("timeout")
	_ = timeoutSecs // Explicitly mark as used to satisfy compiler

	// Parse addresses
	multisigPDA, err := solana.PublicKeyFromBase58(multisigStr)
	if err != nil {
		log.Fatalf("Invalid multisig address: %v", err)
	}

	recipientPubkey, err := solana.PublicKeyFromBase58(toStr)
	if err != nil {
		log.Fatalf("Invalid recipient address: %v", err)
	}

	// Load payer keypair
	payer, err := LoadKeypair(payerPath)
	if err != nil {
		log.Fatalf("Failed to load payer keypair: %v", err)
	}

	// Set up RPC and WebSocket clients
	client := rpc.New(rpcEndpoint)
	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer wsClient.Close()

	// Get Vault PDA
	vaultPDA, _ := multisig.GetVaultPDA(multisigPDA, vaultIndex)

	// Convert SOL to lamports
	lamports := uint64(math.Round(amount * 1_000_000_000))

	// Fetch multisig account to get current transaction index
	multisigAccount, err := fetchMultisigAccount(client, multisigPDA)
	if err != nil {
		log.Fatalf("Failed to fetch multisig account: %v", err)
	}

	// Check if payer is a member of the multisig
	isMember := false
	for _, member := range multisigAccount.Members {
		if member.Key.Equals(payer.PublicKey()) {
			// Also check if member has permission to propose
			if member.Permissions.Mask&1 != 0 { // 1 is the permission to propose
				isMember = true
				break
			}
		}
	}

	if !isMember {
		log.Fatalf("Error: The payer %s is not a member of this multisig or doesn't have proposal permission",
			payer.PublicKey())
	}

	// Check the vault balance
	vaultBalance, err := getAccountBalance(client, vaultPDA)
	if err != nil {
		log.Printf("Warning: Unable to fetch vault balance: %v", err)
	} else if vaultBalance < lamports {
		log.Fatalf("Error: Vault balance is insufficient: %f SOL, trying to send %f SOL",
			float64(vaultBalance)/1e9, amount)
	}

	// Create the transfer instruction - use system program's Transfer instruction directly
	transferIx := solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{
			solana.Meta(vaultPDA).WRITE(),
			solana.Meta(recipientPubkey).WRITE(),
		},
		// The data layout for System Program's Transfer instruction:
		// [0] - instruction index (2 = Transfer)
		// [1-8] - lamports (uint64 LE)
		append(
			[]byte{2, 0, 0, 0}, // instruction index 2 = Transfer
			uint64ToLEBytes(lamports)...,
		),
	)

	// Prepare transaction message bytes for the vault transaction
	txMessageBytes := createTransactionMessageBytes(transferIx, vaultPDA, recipientPubkey, lamports)

	// Prepare transaction index for the new transaction
	transactionIndex := multisigAccount.TransactionIndex + 1

	// Calculate all PDAs needed
	txPDA, _ := multisig.GetTransactionPDA(multisigPDA, transactionIndex)
	proposalPDA, _ := multisig.GetProposalPDA(multisigPDA, transactionIndex)

	// Build vault transaction create instruction
	vaultTxCreateArgs := squads_multisig_program.VaultTransactionCreateArgs{
		VaultIndex:         vaultIndex,
		EphemeralSigners:   0, // No ephemeral signers for a simple transfer
		TransactionMessage: txMessageBytes,
	}

	if memo != "" {
		vaultTxCreateArgs.Memo = &memo
	}

	vaultTxCreateIx := squads_multisig_program.NewVaultTransactionCreateInstruction(
		vaultTxCreateArgs,
		multisigPDA,
		txPDA,
		payer.PublicKey(),
		payer.PublicKey(),
		solana.SystemProgramID,
	).Build()

	// Build proposal create instruction
	proposalCreateArgs := squads_multisig_program.ProposalCreateArgs{
		TransactionIndex: transactionIndex,
		Draft:            false,
	}

	proposalCreateIx := squads_multisig_program.NewProposalCreateInstruction(
		proposalCreateArgs,
		multisigPDA,
		proposalPDA,
		payer.PublicKey(),
		payer.PublicKey(),
		solana.SystemProgramID,
	).Build()

	// Create instructions array
	instructions := []solana.Instruction{vaultTxCreateIx, proposalCreateIx}

	// If auto-approve, add approval instruction
	if autoApprove {
		proposalVoteArgs := squads_multisig_program.ProposalVoteArgs{}
		if memo != "" {
			proposalVoteArgs.Memo = &memo
		}

		approveIx := squads_multisig_program.NewProposalApproveInstruction(
			proposalVoteArgs,
			multisigPDA,
			payer.PublicKey(),
			proposalPDA,
		).Build()

		instructions = append(instructions, approveIx)
	}

	// Get latest blockhash
	hash, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("Failed to get latest blockhash: %v", err)
	}

	// Create transaction
	tx, err := solana.NewTransaction(
		instructions,
		hash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		log.Fatalf("Failed to create transaction: %v", err)
	}

	// Sign transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(payer.PublicKey()) {
				return &payer
			}
			return nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	// Prepare logging output
	log.Printf("Creating transaction to transfer %f SOL to %s", amount, recipientPubkey)
	log.Printf("  Multisig: %s", multisigPDA)
	log.Printf("  Vault PDA: %s", vaultPDA)
	log.Printf("  Transaction: %s", txPDA)
	log.Printf("  Proposal: %s", proposalPDA)
	log.Printf("  Transaction Index: %d", transactionIndex)

	if memo != "" {
		log.Printf("  Memo: %s", memo)
	}

	// Send transaction
	sig, err := sendAndConfirmTransaction.SendAndConfirmTransaction(
		ctx,
		client,
		wsClient,
		tx,
	)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	log.Printf("Transaction submitted: %s", sig)

	// Get transaction status
	sigStr := sig.String()
	// Get transaction status
	sigStatuses, err := client.GetSignatureStatuses(
		ctx,
		true, // search transaction history
		solana.MustSignatureFromBase58(sig.String()),
	)
	if err != nil {
		log.Printf("Could not fetch transaction status: %v", err)
	} else if len(sigStatuses.Value) > 0 && sigStatuses.Value[0] != nil {
		status := sigStatuses.Value[0]
		if status.Err != nil {
			log.Printf("Transaction failed with error: %v", status.Err)
		} else {
			log.Printf("Transaction confirmed successfully")
		}
	}

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("       TRANSACTION CREATED SUCCESSFULLY")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Transaction Signature: %s\n", sigStr)
	fmt.Printf("Transaction PDA: %s\n", txPDA)
	fmt.Printf("Proposal PDA: %s\n", proposalPDA)
	fmt.Printf("Transfer Amount: %f SOL\n", amount)
	fmt.Printf("Recipient: %s\n", recipientPubkey)

	if autoApprove {
		fmt.Println("\nTransaction was automatically approved by the creator.")
		fmt.Printf("Required Approvals: %d/%d\n", 1, multisigAccount.Threshold)
		fmt.Printf("Current Approvals: 1 (%s)\n", payer.PublicKey())

		if multisigAccount.Threshold > 1 {
			fmt.Printf("\nWaiting for %d more approvals before execution is possible.\n",
				multisigAccount.Threshold-1)
		} else {
			fmt.Printf("\nTransaction has reached threshold and can be executed after timelock of %d seconds.\n",
				multisigAccount.TimeLock)

			if multisigAccount.TimeLock > 0 {
				unlockTime := time.Now().Add(time.Duration(multisigAccount.TimeLock) * time.Second)
				fmt.Printf("Executable after: %s\n", unlockTime.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Println("Executable now (no timelock).")
			}
		}
	} else {
		fmt.Println("\nTransaction requires explicit approval. Use the following command to approve:")
		fmt.Printf("  squads-cli transaction approve --multisig %s --transaction %d --payer /path/to/keypair.json\n",
			multisigPDA, transactionIndex)
	}
}

// NewCreateCommand creates the command for creating a new transaction
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new transaction proposal for a Squads Multisig",
		Long: `Create a transaction proposal for a Squads Multisig.

This command allows you to create a transaction proposal with various types of instructions.
Currently supports SOL transfer transactions.

Examples:
# Transfer SOL from multisig vault
squads-cli transaction create \
--multisig MULTISIG_ADDRESS \
--to RECIPIENT_ADDRESS \
--amount 0.1 \
--payer /path/to/payer.json
`,
		Run: runCreateTransaction,
	}

	cmd.Flags().StringP("multisig", "m", "", "Multisig PDA address (REQUIRED)")
	cmd.Flags().StringP("to", "t", "", "Recipient address (REQUIRED)")
	cmd.Flags().Float64P("amount", "a", 0, "Amount of SOL to transfer (REQUIRED)")
	cmd.Flags().StringP("payer", "p", "", "Payer keypair path (REQUIRED)")
	cmd.Flags().Uint8P("vault-index", "v", 0, "Vault index (default 0)")
	cmd.Flags().StringP("memo", "", "", "Transaction memo (optional)")
	cmd.Flags().BoolP("approve", "", true, "Auto-approve the transaction (default true)")
	cmd.Flags().Uint32P("timeout", "", 60, "Transaction confirmation timeout in seconds (default 60)")

	cmd.MarkFlagRequired("multisig")
	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("amount")
	cmd.MarkFlagRequired("payer")

	return cmd
}
