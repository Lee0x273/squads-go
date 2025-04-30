package transaction

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"squads-go/generated/squads_multisig_program"
	"squads-go/pkg/multisig"
)

// ProposalVoteInput defines input parameters for voting on a proposal
type ProposalVoteInput struct {
	// Required inputs
	Multisig         solana.PublicKey
	TransactionIndex uint64
	Voter            solana.PrivateKey

	// Optional inputs
	Memo     string
	Action   string // "approve", "reject", or "cancel"
	Client   *rpc.Client
	WsClient *ws.Client
}

// ProposalVoteOutput defines return values from voting on a proposal
type ProposalVoteOutput struct {
	Signature     string
	ProposalPDA   solana.PublicKey
	Action        string
	CurrentStatus string
	Approvals     int
	Rejections    int
	Cancelled     int
	Threshold     uint16

	// If approved and at threshold, shows when execution is possible
	ExecutableAfter *time.Time
}

// VoteOnProposal votes on a proposal with the specified action (approve, reject, or cancel)
// This version submits the transaction but doesn't wait for confirmation
func VoteOnProposal(ctx context.Context, input ProposalVoteInput) (*ProposalVoteOutput, error) {
	// Validate action
	action := input.Action
	if action == "" {
		action = "approve" // Default to approve
	}

	if action != "approve" && action != "reject" && action != "cancel" {
		return nil, fmt.Errorf("invalid action: %s. Must be 'approve', 'reject', or 'cancel'", action)
	}

	log.Printf("%sing proposal for transaction %d...", action, input.TransactionIndex)

	// Create clients if not provided
	if input.Client == nil {
		// Use a default endpoint that works with our project instead of MainnetRPCEndpoint
		input.Client = rpc.New("https://api.mainnet-beta.solana.com")
	}

	var wsClientCreated bool
	if input.WsClient == nil {
		// Use a default WS endpoint that works with our project
		wsClient, err := ws.Connect(ctx, "wss://api.mainnet-beta.solana.com")
		if err != nil {
			return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		input.WsClient = wsClient
		wsClientCreated = true
		defer func() {
			if wsClientCreated {
				input.WsClient.Close()
			}
		}()
	}

	// Calculate proposal PDA
	proposalPDA, _ := multisig.GetProposalPDA(input.Multisig, input.TransactionIndex)

	// Validate that the multisig and proposal accounts exist
	// Check if the multisig account exists
	multisigInfo, err := input.Client.GetAccountInfo(ctx, input.Multisig)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch multisig account: %w", err)
	}
	if multisigInfo.Value == nil || len(multisigInfo.Value.Data.GetBinary()) == 0 {
		return nil, fmt.Errorf("multisig account not found or not initialized: %s", input.Multisig)
	}

	// Check if the proposal account exists
	proposalInfo, err := input.Client.GetAccountInfo(ctx, proposalPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch proposal account: %w", err)
	}
	if proposalInfo.Value == nil || len(proposalInfo.Value.Data.GetBinary()) == 0 {
		return nil, fmt.Errorf("proposal account not found or not initialized: %s", proposalPDA)
	}

	// Build proposal vote arguments
	proposalVoteArgs := squads_multisig_program.ProposalVoteArgs{}
	if input.Memo != "" {
		proposalVoteArgs.Memo = &input.Memo
	}

	// Select the appropriate instruction based on the action
	var votingIx solana.Instruction

	switch action {
	case "approve":
		votingIx = squads_multisig_program.NewProposalApproveInstruction(
			proposalVoteArgs,
			input.Multisig,
			input.Voter.PublicKey(),
			proposalPDA,
		).Build()
	case "reject":
		votingIx = squads_multisig_program.NewProposalRejectInstruction(
			proposalVoteArgs,
			input.Multisig,
			input.Voter.PublicKey(),
			proposalPDA,
		).Build()
	case "cancel":
		// Use ProposalCancelV2 for better compatibility
		votingIx = squads_multisig_program.NewProposalCancelV2Instruction(
			proposalVoteArgs,
			input.Multisig,
			input.Voter.PublicKey(),
			proposalPDA,
			solana.SystemProgramID,
		).Build()
	}

	// Get latest blockhash
	hash, err := input.Client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	// Create transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{votingIx},
		hash.Value.Blockhash,
		solana.TransactionPayer(input.Voter.PublicKey()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create voting transaction: %w", err)
	}

	// Sign transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(input.Voter.PublicKey()) {
				return &input.Voter
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Log transaction details for debugging
	log.Printf("%sing proposal for transaction %d with voter %s",
		action, input.TransactionIndex, input.Voter.PublicKey())
	log.Printf("Proposal PDA: %s", proposalPDA)

	// Submit transaction WITHOUT waiting for confirmation
	// This will prevent the CLI from hanging
	sig, err := input.Client.SendTransaction(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send voting transaction: %w", err)
	}

	// Create output with minimal information
	output := &ProposalVoteOutput{
		Signature:   sig.String(),
		ProposalPDA: proposalPDA,
		Action:      action,
	}

	log.Printf("✓ Successfully submitted %s transaction: %s", action, sig)
	log.Printf("Transaction may take a few seconds to confirm.")

	return output, nil
}
