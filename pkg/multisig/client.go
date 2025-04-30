package multisig

import (
	"context"
	"fmt"

	"github.com/hogyzen12/squads-go/generated/squads_multisig_program"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// Instruction is an alias for solana.Instruction to avoid unused variable warning
type Instruction = solana.Instruction

// FetchProgramConfig fetches and decodes the program config from the blockchain
func FetchProgramConfig(client *rpc.Client, programConfigPDA solana.PublicKey) (*squads_multisig_program.ProgramConfig, error) {
	accountInfo, err := client.GetAccountInfo(context.Background(), programConfigPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch program config: %w", err)
	}

	if accountInfo.Value == nil {
		return nil, fmt.Errorf("program config account does not exist")
	}

	// Skip the 8-byte discriminator
	data := accountInfo.Value.Data.GetBinary()
	if len(data) <= 8 {
		return nil, fmt.Errorf("account data too short")
	}

	var programConfig squads_multisig_program.ProgramConfig
	decoder := ag_binary.NewBorshDecoder(data)
	err = programConfig.UnmarshalWithDecoder(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to decode program config: %w", err)
	}

	return &programConfig, nil
}

// CreateMultisig creates a new multisig on the Solana blockchain
func CreateMultisig(
	client *rpc.Client,
	wsClient *ws.Client,
	payer solana.PrivateKey,
	createKey solana.PrivateKey,
	members []squads_multisig_program.Member,
	threshold uint16,
	timeLock uint32,
	programID solana.PublicKey,
) (string, solana.PublicKey, error) {
	ctx := context.Background()

	// Get PDAs
	multisigPDA, _ := GetMultisigPDA(createKey.PublicKey(), programID)
	programConfigPDA, _ := GetProgramConfigPDA(programID)

	// Fetch the program config to get the treasury
	programConfig, err := FetchProgramConfig(client, programConfigPDA)
	if err != nil {
		// If the program config doesn't exist, we need to initialize it
		// This is a rare case and usually only happens in testing environments
		// In production, the program config should already be initialized
		return "", solana.PublicKey{}, fmt.Errorf("failed to fetch program config: %w", err)
	}

	treasury := programConfig.Treasury

	// Prepare instruction arguments
	args := squads_multisig_program.MultisigCreateArgsV2{
		ConfigAuthority: nil, // None
		Threshold:       threshold,
		Members:         members,
		TimeLock:        timeLock,
		RentCollector:   nil, // None
		Memo:            nil, // None
	}

	// Build the instruction using the generated method
	instruction := squads_multisig_program.NewMultisigCreateV2Instruction(
		args,
		programConfigPDA,
		treasury,
		multisigPDA,
		createKey.PublicKey(),
		payer.PublicKey(),
		solana.SystemProgramID,
	).Build()

	// Get latest blockhash
	hash, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	// Create transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		hash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Sign transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(payer.PublicKey()) {
				return &payer
			}
			if key.Equals(createKey.PublicKey()) {
				return &createKey
			}
			return nil
		},
	)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	sig, err := confirm.SendAndConfirmTransaction(
		ctx,
		client,
		wsClient,
		tx,
	)
	if err != nil {
		return "", solana.PublicKey{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return sig.String(), multisigPDA, nil
}
