package transaction

import (
	"context"
	"fmt"
	"time"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"squads-go/generated/squads_multisig_program"
)

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

// fetchProposalAccount fetches and decodes a proposal account
func fetchProposalAccount(client *rpc.Client, proposalPDA solana.PublicKey) (*squads_multisig_program.Proposal, error) {
	accountInfo, err := client.GetAccountInfo(context.Background(), proposalPDA)
	if err != nil {
		return nil, fmt.Errorf("failed to get proposal account: %w", err)
	}

	var proposalAccount squads_multisig_program.Proposal
	decoder := ag_binary.NewBorshDecoder(accountInfo.Value.Data.GetBinary())
	err = proposalAccount.UnmarshalWithDecoder(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to decode proposal account: %w", err)
	}

	return &proposalAccount, nil
}

// getProposalStatusString returns a human-readable string for a proposal status
func getProposalStatusString(status squads_multisig_program.ProposalStatus) string {
	switch status.(type) {
	case *squads_multisig_program.ProposalStatusDraft:
		draft := status.(*squads_multisig_program.ProposalStatusDraft)
		return fmt.Sprintf("Draft (created %s)", formatUnixTimestamp(draft.Timestamp))
	case *squads_multisig_program.ProposalStatusActive:
		active := status.(*squads_multisig_program.ProposalStatusActive)
		return fmt.Sprintf("Active (since %s)", formatUnixTimestamp(active.Timestamp))
	case *squads_multisig_program.ProposalStatusRejected:
		rejected := status.(*squads_multisig_program.ProposalStatusRejected)
		return fmt.Sprintf("Rejected (at %s)", formatUnixTimestamp(rejected.Timestamp))
	case *squads_multisig_program.ProposalStatusApproved:
		approved := status.(*squads_multisig_program.ProposalStatusApproved)
		return fmt.Sprintf("Approved (at %s)", formatUnixTimestamp(approved.Timestamp))
	case *squads_multisig_program.ProposalStatusExecuting:
		return "Executing"
	case *squads_multisig_program.ProposalStatusExecuted:
		executed := status.(*squads_multisig_program.ProposalStatusExecuted)
		return fmt.Sprintf("Executed (at %s)", formatUnixTimestamp(executed.Timestamp))
	case *squads_multisig_program.ProposalStatusCancelled:
		cancelled := status.(*squads_multisig_program.ProposalStatusCancelled)
		return fmt.Sprintf("Cancelled (at %s)", formatUnixTimestamp(cancelled.Timestamp))
	default:
		return "Unknown Status"
	}
}

// formatUnixTimestamp returns a formatted time string from a Unix timestamp
func formatUnixTimestamp(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02 15:04:05")
}
