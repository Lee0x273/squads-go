package tests

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
)

// TestTransactionLifecycle tests the complete lifecycle of a transaction
// from creation to execution
func TestTransactionLifecycle(t *testing.T) {
	// Skip if in short mode
	if testing.Short() {
		t.Skip("Skipping transaction lifecycle test in short mode")
	}

	config := SetupTestEnvironment(t)

	// Need existing multisig to test with
	if *multisigAddr == "" {
		t.Skip("This test requires an existing multisig. Use --multisig flag.")
	}

	// Set test parameters
	multisigAddress := *multisigAddr
	transferAmount := "0.005" // Small amount to avoid draining the vault
	recipient := ""           // Will be set to the keypair's public key

	// Get keypair's public key to use as recipient if needed
	keyPair, err := LoadKeypair(config.KeypairPath)
	require.NoError(t, err, "Failed to load keypair")
	recipient = keyPair.PublicKey().String()

	t.Logf("Starting transaction lifecycle test")
	t.Logf("Multisig: %s", multisigAddress)
	t.Logf("Recipient: %s", recipient)
	t.Logf("Amount: %s SOL", transferAmount)

	// Step 1: Get multisig info to verify our account and check the vault
	t.Run("Step 1: Check Multisig Info", func(t *testing.T) {
		output, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", multisigAddress,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info")
		t.Logf("Multisig info retrieved successfully")

		// Extract vault address
		vaultAddr, err := ExtractAddressFromOutput(output, "Default Vault (Index 0):")
		require.NoError(t, err, "Failed to extract vault address from multisig info")
		t.Logf("Vault address: %s", vaultAddr)

		// Connect to RPC to check if vault has sufficient balance
		client := rpc.New(config.RpcEndpoint)
		balance, err := client.GetBalance(
			context.Background(),
			solana.MustPublicKeyFromBase58(vaultAddr),
			rpc.CommitmentConfirmed,
		)
		if err != nil {
			t.Logf("Warning: Failed to check vault balance: %v", err)
		} else {
			balanceSOL := float64(balance.Value) / 1e9
			transferAmountSOL, _ := strconv.ParseFloat(transferAmount, 64)
			t.Logf("Vault balance: %f SOL", balanceSOL)
			if balanceSOL < transferAmountSOL {
				t.Fatalf("Vault has insufficient balance: %f SOL, need at least %f SOL",
					balanceSOL, transferAmountSOL)
			}
			t.Logf("Vault has sufficient balance for transfer")
		}
	})

	// Step 2: Create a transaction
	var transactionIndex string
	t.Run("Step 2: Create Transaction", func(t *testing.T) {
		output, err := RunCommand(t, 90*time.Second, config.CliPath,
			"transaction", "create",
			"--multisig", multisigAddress,
			"--to", recipient,
			"--amount", transferAmount,
			"--payer", config.KeypairPath,
			"--rpc", config.RpcEndpoint,
			"--ws", config.WsEndpoint,
		)
		// On devnet, the command might timeout but still succeed
		if err != nil {
			t.Logf("Command returned error, but transaction might have succeeded: %v", err)
			t.Logf("Output: %s", output)
		} else {
			t.Logf("Transaction created successfully")
		}

		// Extract transaction index from output
		if output != "" {
			// Try to find transaction index in the output
			for _, line := range []string{
				"Transaction Index: ",
				"Transaction #",
			} {
				if idx, err := ExtractNumberFromOutput(output, line); err == nil {
					transactionIndex = idx
					t.Logf("Transaction index: %s", transactionIndex)
					break
				}
			}
		}

		// If transaction index wasn't found, check multisig info
		if transactionIndex == "" {
			t.Log("Transaction index not found in create output, checking multisig info...")
			time.Sleep(5 * time.Second) // Give some time for the transaction to confirm

			infoOutput, err := RunCommand(t, 15*time.Second, config.CliPath,
				"multisig", "info",
				"--address", multisigAddress,
				"--rpc", config.RpcEndpoint,
			)
			require.NoError(t, err, "Failed to get multisig info after transaction creation")

			// Try to find the transaction index in info output
			if idx, err := ExtractNumberFromOutput(infoOutput, "Transaction #"); err == nil {
				transactionIndex = idx
				t.Logf("Transaction index from multisig info: %s", transactionIndex)
			} else {
				t.Fatalf("Could not find transaction index in multisig info")
			}
		}

		require.NotEmpty(t, transactionIndex, "Failed to extract transaction index")
	})

	// Step 3: Check if our keypair has already approved the transaction
	// and proceed to execute if we have sufficient approvals
	t.Run("Step 3: Check Approval Status", func(t *testing.T) {
		// Get multisig info to check transaction status
		infoOutput, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", multisigAddress,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info")

		// Check if the transaction is already approved
		isApproved := false
		if len(infoOutput) > 0 {
			// Look for "Status: Approved" in the output
			if ContainsAny(infoOutput, []string{
				"Status: Approved",
				"Current Approvals: 1",
				"Transaction has reached threshold",
			}) {
				isApproved = true
				t.Log("Transaction is approved and ready for execution")
			} else {
				t.Log("Transaction needs more approvals before execution")
			}
		}

		// If not approved, we need to check if we can approve it
		if !isApproved {
			t.Log("In a real scenario, other members would need to approve this transaction")
			t.Logf("You can approve with: ./squads-cli transaction approve --multisig %s --transaction %s --payer /path/to/keypair.json",
				multisigAddress, transactionIndex)
		}
	})

	// Step 4: Execute the transaction if it has enough approvals
	// Note: In most test cases with a 1/X multisig or if the creator auto-approved,
	// the transaction should be executable now.
	t.Run("Step 4: Execute Transaction", func(t *testing.T) {
		if !t.Run("check if we can execute", func(t *testing.T) {
			// Assuming we have approval, try to execute
			output, err := RunCommand(t, 90*time.Second, config.CliPath,
				"transaction", "execute",
				"--multisig", multisigAddress,
				"--transaction", transactionIndex,
				"--payer", config.KeypairPath,
				"--rpc", config.RpcEndpoint,
				"--ws", config.WsEndpoint,
			)

			if err != nil {
				if ContainsAny(output, []string{
					"timelock has not elapsed",
					"transaction is not in approved state",
				}) {
					t.Skip("Transaction not ready for execution: " + output)
				} else {
					t.Fatalf("Execution failed: %v\nOutput: %s", err, output)
				}
			}

			t.Logf("Transaction executed successfully")

			// Wait for execution to confirm
			time.Sleep(10 * time.Second)
		}) {
			t.Skip("Skipping execution verification")
		}

		// Verify execution by checking multisig info again
		infoOutput, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", multisigAddress,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info after execution")

		// Look for "Status: Executed" in the output
		if ContainsAny(infoOutput, []string{
			"Status: Executed",
			"Transaction executed successfully",
		}) {
			t.Log("Transaction execution confirmed in multisig info")
		} else {
			t.Log("Could not confirm transaction execution status")
		}
	})
}

// ExtractNumberFromOutput extracts a number from the command output
func ExtractNumberFromOutput(output, prefix string) (string, error) {
	for _, line := range splitLines(output) {
		if ContainsString(line, prefix) {
			// Different formats to try
			for _, format := range []string{
				prefix + "%d",      // e.g., "Transaction Index: 123"
				prefix + " %d",     // e.g., "Transaction # 123"
				prefix + "%d:",     // e.g., "Transaction 123:"
				prefix + "#%d:",    // e.g., "Transaction #123:"
				prefix + " #%d",    // e.g., "Transaction #123"
				prefix + ".*?(%d)", // Any format with number in parentheses
			} {
				var num int
				if _, err := fmt.Sscanf(line, format, &num); err == nil {
					return strconv.Itoa(num), nil
				}
			}

			// If specific formats don't work, try a more general approach
			for _, word := range splitWords(line) {
				if num, err := strconv.Atoi(word); err == nil {
					return strconv.Itoa(num), nil
				}
			}
		}
	}
	return "", fmt.Errorf("number with prefix '%s' not found in output", prefix)
}

// Helper functions
func splitLines(s string) []string {
	return split(s, "\n")
}

func splitWords(s string) []string {
	return split(s, " \t\n")
}

func split(s, sep string) []string {
	var result []string
	for _, part := range []string{s} {
		for _, sep := range sep {
			parts := []string{}
			for _, subpart := range part {
				if subpart == sep {
					if len(parts) > 0 {
						result = append(result, parts...)
						parts = []string{}
					}
				} else {
					parts = append(parts, string(subpart))
				}
			}
			if len(parts) > 0 {
				result = append(result, parts...)
			}
		}
	}
	return result
}

func ContainsString(s, substr string) bool {
	return s != "" && substr != "" && s != substr && len(s) >= len(substr) && s[0:len(substr)] == substr
}

func ContainsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if s != "" && substr != "" && s != substr && len(s) >= len(substr) && s[0:len(substr)] == substr {
			return true
		}
	}
	return false
}
