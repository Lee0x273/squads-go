package multisiginfo

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/spf13/cobra"

	"squads-go/generated/squads_multisig_program"
	"squads-go/pkg/multisig"
)

// Define permission masks
const (
	PermissionPropose uint8 = 1 << 0
	PermissionVote    uint8 = 1 << 1
	PermissionExecute uint8 = 1 << 2
	PermissionFull    uint8 = PermissionPropose | PermissionVote | PermissionExecute
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display information about a Squads Multisig",
		Long: `Display detailed information about a Squads Multisig, including:
- Multisig configuration (threshold, timelock)
- Member list with permissions
- Vault addresses
- Transaction counts
- Proposal status

Example:
  squads-cli multisig info --address BXWqvwmYKZV8UKLCCL7TDwDWWYRmfi5RuusX44zESaWA
`,
		Run: runInfoCommand,
	}

	cmd.Flags().StringP("address", "a", "", "Multisig address (REQUIRED)")
	cmd.MarkFlagRequired("address")

	return cmd
}

func runInfoCommand(cmd *cobra.Command, args []string) {
	// Get RPC endpoint
	rpcEndpoint, _ := cmd.Parent().Flags().GetString("rpc")

	// Get multisig address
	multisigStr, _ := cmd.Flags().GetString("address")
	multisigAddr, err := solana.PublicKeyFromBase58(multisigStr)
	if err != nil {
		log.Fatalf("Invalid multisig address: %v", err)
	}

	// Set up RPC client
	client := rpc.New(rpcEndpoint)

	// Fetch multisig account
	multisigAccount, err := fetchMultisigAccount(client, multisigAddr)
	if err != nil {
		log.Fatalf("Failed to fetch multisig account: %v", err)
	}

	// Display multisig information
	displayMultisigInfo(multisigAddr, multisigAccount)

	// Get vault PDA (default vault index 0)
	vaultPDA, vaultBump := multisig.GetVaultPDA(multisigAddr, 0)
	fmt.Printf("\nMultisig Vaults:\n")
	fmt.Printf("  Default Vault (Index 0): %s (Bump: %d)\n", vaultPDA, vaultBump)

	// Get balance of the vault
	balance, err := getAccountBalance(client, vaultPDA)
	if err != nil {
		fmt.Printf("  Balance: Unable to fetch balance\n")
	} else {
		fmt.Printf("  Balance: %f SOL\n", float64(balance)/1e9)
	}

	// Show a list of the last 5 transactions if any exist
	if multisigAccount.TransactionIndex > 0 {
		fmt.Printf("\nRecent Transactions:\n")
		// Show up to the last 5 transactions
		startIdx := multisigAccount.TransactionIndex
		if startIdx > 5 {
			startIdx = 5
		}

		for i := multisigAccount.TransactionIndex; i > multisigAccount.TransactionIndex-startIdx; i-- {
			txPDA, _ := multisig.GetTransactionPDA(multisigAddr, i)
			proposalPDA, _ := multisig.GetProposalPDA(multisigAddr, i)

			// Try to fetch the proposal to get its status
			proposal, err := fetchProposalAccount(client, proposalPDA)
			if err != nil {
				fmt.Printf("  Transaction #%d: %s (Proposal: %s) - Unable to fetch status\n",
					i, txPDA.String(), proposalPDA.String())
				continue
			}

			status := getProposalStatusString(proposal.Status)
			fmt.Printf("  Transaction #%d: %s - Status: %s\n", i, txPDA.String(), status)

			// Show approval count if in active or approved state
			if strings.Contains(status, "Active") || strings.Contains(status, "Approved") {
				fmt.Printf("    Approvals: %d, Rejections: %d, Cancellations: %d\n",
					len(proposal.Approved), len(proposal.Rejected), len(proposal.Cancelled))
			}
		}
	} else {
		fmt.Println("\nNo transactions created yet.")
	}
}

func fetchMultisigAccount(
	client *rpc.Client,
	multisigPDA solana.PublicKey,
) (*squads_multisig_program.Multisig, error) {
	accountInfo, err := client.GetAccountInfo(
		context.Background(),
		multisigPDA,
	)
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

func fetchProposalAccount(
	client *rpc.Client,
	proposalPDA solana.PublicKey,
) (*squads_multisig_program.Proposal, error) {
	accountInfo, err := client.GetAccountInfo(
		context.Background(),
		proposalPDA,
	)
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

func getAccountBalance(client *rpc.Client, pubkey solana.PublicKey) (uint64, error) {
	balance, err := client.GetBalance(
		context.Background(),
		pubkey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return 0, err
	}
	return balance.Value, nil
}

func displayMultisigInfo(address solana.PublicKey, multisig *squads_multisig_program.Multisig) {
	fmt.Println("═════════════════════════════════════════")
	fmt.Println("           MULTISIG DETAILS              ")
	fmt.Println("═════════════════════════════════════════")
	fmt.Printf("Address: %s\n", address.String())
	fmt.Printf("Create Key: %s\n", multisig.CreateKey.String())
	fmt.Printf("Threshold: %d/%d\n", multisig.Threshold, countVotingMembers(multisig.Members))
	fmt.Printf("Time Lock: %d seconds\n", multisig.TimeLock)

	// Show config authority
	if multisig.ConfigAuthority.IsZero() {
		fmt.Println("Config Authority: None (Autonomous)")
	} else {
		fmt.Printf("Config Authority: %s\n", multisig.ConfigAuthority.String())
	}

	// Show rent collector if set
	if multisig.RentCollector != nil {
		fmt.Printf("Rent Collector: %s\n", multisig.RentCollector.String())
	} else {
		fmt.Println("Rent Collector: None")
	}

	// Transaction indices
	fmt.Printf("Transaction Count: %d\n", multisig.TransactionIndex)
	fmt.Printf("Stale Transaction Index: %d\n", multisig.StaleTransactionIndex)

	// Member information
	fmt.Println("\nMembers:")
	for i, member := range multisig.Members {
		permStr := describePermissions(member.Permissions.Mask)
		fmt.Printf("  %d. %s\n     Permissions: %s\n",
			i+1, member.Key.String(), permStr)
	}
}

func countVotingMembers(members []squads_multisig_program.Member) int {
	count := 0
	for _, member := range members {
		if member.Permissions.Mask&PermissionVote != 0 {
			count++
		}
	}
	return count
}

func describePermissions(mask uint8) string {
	var desc []string
	if mask&PermissionPropose != 0 {
		desc = append(desc, "Propose")
	}
	if mask&PermissionVote != 0 {
		desc = append(desc, "Vote")
	}
	if mask&PermissionExecute != 0 {
		desc = append(desc, "Execute")
	}
	if len(desc) == 0 {
		return "No Permissions"
	}
	return strings.Join(desc, ", ")
}

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

func formatUnixTimestamp(timestamp int64) string {
	t := time.Unix(timestamp, 0)
	return t.Format("2006-01-02 15:04:05")
}
