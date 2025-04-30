package multisigtransaction

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/spf13/cobra"

	"squads-go/pkg/transaction"
)

// NewApproveCommand creates the command for approving a transaction proposal
func NewApproveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve a transaction proposal for a Squads Multisig",
		Long: `Approve a transaction proposal for a Squads Multisig.

This command allows a multisig member to approve a transaction that has been
proposed but not yet executed. Each member with "Vote" permission can approve
a transaction once. When the approval threshold is reached, the transaction
becomes eligible for execution.

Examples:
# Approve a transaction
squads-cli transaction approve \
--multisig MULTISIG_ADDRESS \
--transaction TRANSACTION_INDEX \
--payer /path/to/payer.json
`,
		Run: runApproveTransaction,
	}

	cmd.Flags().StringP("multisig", "m", "", "Multisig PDA address (REQUIRED)")
	cmd.Flags().Uint64P("transaction", "t", 0, "Transaction index to approve (REQUIRED)")
	cmd.Flags().StringP("payer", "p", "", "Member keypair path for approval (REQUIRED)")
	cmd.Flags().StringP("memo", "", "", "Optional memo for the approval")
	cmd.Flags().Uint32P("timeout", "", 60, "Transaction confirmation timeout in seconds (default 60)")

	cmd.MarkFlagRequired("multisig")
	cmd.MarkFlagRequired("transaction")
	cmd.MarkFlagRequired("payer")

	return cmd
}

func runApproveTransaction(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load RPC endpoints
	rpcEndpoint, _ := cmd.Parent().Parent().Flags().GetString("rpc")
	wsEndpoint, _ := cmd.Parent().Parent().Flags().GetString("ws")

	// Get flags
	multisigStr, _ := cmd.Flags().GetString("multisig")
	transactionIndex, _ := cmd.Flags().GetUint64("transaction")
	payerPath, _ := cmd.Flags().GetString("payer")
	memo, _ := cmd.Flags().GetString("memo")
	timeoutSecs, _ := cmd.Flags().GetUint32("timeout")

	// Parse multisig address
	multisigPDA, err := solana.PublicKeyFromBase58(multisigStr)
	if err != nil {
		log.Fatalf("Invalid multisig address: %v", err)
	}

	// Load payer keypair
	payer, err := transaction.LoadKeypair(payerPath)
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

	// Prepare approval input
	input := transaction.ProposalVoteInput{
		Multisig:         multisigPDA,
		TransactionIndex: transactionIndex,
		Voter:            payer,
		Memo:             memo,
		Action:           "approve", // Specifically for approval
		Client:           client,
		WsClient:         wsClient,
	}

	// Start approval
	log.Printf("Approving transaction #%d on multisig %s...", transactionIndex, multisigPDA)

	// Set context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	// Vote on the proposal (approve)
	output, err := transaction.VoteOnProposal(ctxWithTimeout, input)
	if err != nil {
		log.Fatalf("Failed to approve transaction: %v", err)
	}

	// Display successful result
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("      TRANSACTION APPROVED SUCCESSFULLY")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Transaction Signature: %s\n", output.Signature)
	fmt.Printf("Transaction Status: %s\n", output.CurrentStatus)
	fmt.Printf("Approvals: %d/%d\n", output.Approvals, output.Threshold)

	// If threshold reached, show execution information
	if output.Approvals >= int(output.Threshold) {
		fmt.Println("\nTransaction has reached approval threshold! 🎉")

		if output.ExecutableAfter != nil && output.ExecutableAfter.After(time.Now()) {
			fmt.Printf("Due to timelock, it will be executable after: %s\n",
				output.ExecutableAfter.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Println("Transaction is ready for execution!")
			fmt.Printf("\nTo execute this transaction, run:\n")
			fmt.Printf("  squads-cli transaction execute --multisig %s --transaction %d --payer %s\n",
				multisigPDA, transactionIndex, payerPath)
		}
	} else {
		// Show how many more approvals are needed
		remainingApprovals := int(output.Threshold) - output.Approvals
		fmt.Printf("\nTransaction needs %d more approval(s) to reach threshold.\n", remainingApprovals)
	}
}
