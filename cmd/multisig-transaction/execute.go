package multisigtransaction

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/spf13/cobra"

	"squads-go/pkg/multisig"
	"squads-go/pkg/transaction"
)

// NewExecuteCommand creates the command for executing an approved transaction
func NewExecuteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute an approved transaction for a Squads Multisig",
		Long: `Execute an approved transaction for a Squads Multisig.

This command allows a multisig member with execute permission to execute
a transaction that has been approved by the required number of members
and passed its timelock period (if any).

Examples:
# Execute a transaction
squads-cli transaction execute \
--multisig MULTISIG_ADDRESS \
--transaction TRANSACTION_INDEX \
--payer /path/to/payer.json
`,
		Run: runExecuteTransaction,
	}

	cmd.Flags().StringP("multisig", "m", "", "Multisig PDA address (REQUIRED)")
	cmd.Flags().Uint64P("transaction", "t", 0, "Transaction index to execute (REQUIRED)")
	cmd.Flags().StringP("payer", "p", "", "Member keypair path for execution (REQUIRED)")
	cmd.Flags().Uint32P("timeout", "", 120, "Transaction confirmation timeout in seconds (default 120)")

	cmd.MarkFlagRequired("multisig")
	cmd.MarkFlagRequired("transaction")
	cmd.MarkFlagRequired("payer")

	return cmd
}

func runExecuteTransaction(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Load RPC endpoints
	rpcEndpoint, _ := cmd.Parent().Parent().Flags().GetString("rpc")
	wsEndpoint, _ := cmd.Parent().Parent().Flags().GetString("ws")

	// Get flags
	multisigStr, _ := cmd.Flags().GetString("multisig")
	transactionIndex, _ := cmd.Flags().GetUint64("transaction")
	payerPath, _ := cmd.Flags().GetString("payer")
	timeoutSecs, _ := cmd.Flags().GetUint32("timeout")

	// Parse multisig address
	multisigPDA, err := solana.PublicKeyFromBase58(multisigStr)
	if err != nil {
		log.Fatalf("Invalid multisig address: %v", err)
	}

	// Load payer keypair
	executor, err := transaction.LoadKeypair(payerPath)
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

	// Calculate transaction and proposal PDAs for logging
	txPDA, _ := multisig.GetTransactionPDA(multisigPDA, transactionIndex)
	proposalPDA, _ := multisig.GetProposalPDA(multisigPDA, transactionIndex)

	// Log starting execution
	log.Printf("Executing transaction #%d on multisig %s...", transactionIndex, multisigPDA)
	log.Printf("Transaction PDA: %s", txPDA)
	log.Printf("Proposal PDA: %s", proposalPDA)
	log.Printf("Executor: %s", executor.PublicKey())

	// Set context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	// Execute the transaction
	output, err := transaction.ExecuteProposal(ctxWithTimeout, multisigPDA, transactionIndex, executor, client, wsClient)
	if err != nil {
		log.Fatalf("Failed to execute transaction: %v", err)
	}

	// Display successful result
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("      TRANSACTION EXECUTED SUCCESSFULLY")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Transaction Signature: %s\n", output.Signature)
	fmt.Printf("Transaction PDA: %s\n", output.TransactionPDA)
	fmt.Printf("Proposal PDA: %s\n", output.ProposalPDA)
	fmt.Printf("Transaction Index: %d\n", output.TransactionIndex)
	fmt.Println("\nYou can view this transaction on Solana Explorer:")

	// Check network type to determine explorer URL
	if strings.Contains(rpcEndpoint, "devnet") {
		fmt.Printf("https://explorer.solana.com/tx/%s?cluster=devnet\n", output.Signature)
	} else if strings.Contains(rpcEndpoint, "testnet") {
		fmt.Printf("https://explorer.solana.com/tx/%s?cluster=testnet\n", output.Signature)
	} else {
		fmt.Printf("https://explorer.solana.com/tx/%s\n", output.Signature)
	}
}
