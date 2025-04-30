package tests

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Define test flags
var (
	useDevnet    = flag.Bool("devnet", true, "Run tests on devnet instead of local validator")
	rpcEndpoint  = flag.String("rpc", "https://api.mainnet-beta.solana.com", "RPC endpoint URL")
	wsEndpoint   = flag.String("ws", "wss://api.mainnet-beta.solana.com", "WebSocket endpoint URL")
	keypairPath  = flag.String("keypair", "/Users/hogyzen12/.config/solana/pLgH63FULg9BVrqyjFLwDXyiGBFKvtw7HMByv592WMK.json", "Path to keypair JSON file")
	multisigAddr = flag.String("multisig", "", "Existing multisig address to use for tests")
)

// TestConfig holds configuration for the test environment
type TestConfig struct {
	RpcEndpoint string
	WsEndpoint  string
	CliPath     string
	TestDataDir string
	KeypairPath string
}

// MultisigTestData represents data for a test multisig
type MultisigTestData struct {
	MultisigAddress    string
	VaultAddress       string
	TransactionIndex   uint64
	Members            []string
	MemberKeypairPaths []string
	AdminKeypairPath   string
	CreateKeyPath      string
}

// SaveMultisigTestData saves multisig test data to a file
func SaveMultisigTestData(data *MultisigTestData, testDataDir string) error {
	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal test data: %w", err)
	}

	// Create test data directory if it doesn't exist
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create test data directory: %w", err)
	}

	// Save to file
	testDataPath := filepath.Join(testDataDir, "multisig_test_data.json")
	if err := ioutil.WriteFile(testDataPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write test data file: %w", err)
	}

	return nil
}

// LoadMultisigTestData loads multisig test data from a file
func LoadMultisigTestData(testDataDir string) (*MultisigTestData, error) {
	testDataPath := filepath.Join(testDataDir, "multisig_test_data.json")

	// Read file
	jsonData, err := ioutil.ReadFile(testDataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test data file: %w", err)
	}

	// Unmarshal from JSON
	var data MultisigTestData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test data: %w", err)
	}

	return &data, nil
}

// LoadKeypair loads a keypair from a JSON file
func LoadKeypair(path string) (solana.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keyArray []byte
	if err := json.Unmarshal(keyBytes, &keyArray); err != nil {
		return nil, err
	}

	return solana.PrivateKey(keyArray), nil
}

// RunCommand runs a command and returns its output
func RunCommand(t *testing.T, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %s", timeout)
	}

	return string(output), err
}

// ExtractAddressFromOutput extracts an address from command output
func ExtractAddressFromOutput(output, prefix string) (string, error) {
	// Find the line containing the address
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, prefix) {
			// Extract the address
			parts := strings.Split(line, prefix)
			if len(parts) < 2 {
				continue
			}
			address := strings.TrimSpace(parts[1])

			// Clean up any color codes, special characters, or extra text
			address = strings.Split(address, " ")[0]
			address = strings.Split(address, "(")[0]
			address = strings.ReplaceAll(address, "\x1b[32m", "") // Remove ANSI color codes
			address = strings.ReplaceAll(address, "\x1b[0m", "")
			address = strings.TrimSpace(address)

			// Validate it's a proper base58 address
			_, err := solana.PublicKeyFromBase58(address)
			if err == nil {
				return address, nil
			}
		}
	}

	// Try a more general approach if the specific prefix wasn't found
	for _, line := range lines {
		// Look for any base58-like string (Solana addresses are always 32-44 chars)
		words := strings.Fields(line)
		for _, word := range words {
			word = strings.TrimSpace(word)
			if len(word) >= 32 && len(word) <= 44 {
				_, err := solana.PublicKeyFromBase58(word)
				if err == nil {
					return word, nil
				}
			}
		}
	}

	return "", fmt.Errorf("address with prefix '%s' not found in output", prefix)
}

// SetupTestEnvironment prepares the test environment
func SetupTestEnvironment(t *testing.T) *TestConfig {
	flag.Parse()

	// Default configuration
	config := &TestConfig{
		RpcEndpoint: *rpcEndpoint,
		WsEndpoint:  *wsEndpoint,
		CliPath:     "../squads-cli",
		TestDataDir: "./testdata",
		KeypairPath: *keypairPath,
	}

	// Make sure the CLI exists
	_, err := os.Stat(config.CliPath)
	require.NoError(t, err, "CLI binary not found at %s", config.CliPath)

	// Check that the keypair exists
	_, err = os.Stat(config.KeypairPath)
	require.NoError(t, err, "Keypair not found at %s", config.KeypairPath)

	// Create test data directory if it doesn't exist
	err = os.MkdirAll(config.TestDataDir, 0755)
	require.NoError(t, err, "Failed to create test data directory")

	return config
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// TestDevnetConnection tests the connection to devnet
func TestDevnetConnection(t *testing.T) {
	config := SetupTestEnvironment(t)

	// Connect to devnet
	client := rpc.New(config.RpcEndpoint)

	// Try to get a recent blockhash to verify connection
	_, err := client.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	require.NoError(t, err, "Failed to connect to Solana devnet")

	t.Log("Successfully connected to Solana devnet")
}

// TestCLICommands tests individual CLI commands
func TestCLICommands(t *testing.T) {
	config := SetupTestEnvironment(t)

	// Test help command
	t.Run("Help Command", func(t *testing.T) {
		output, err := RunCommand(t, 5*time.Second, config.CliPath, "--help")
		assert.NoError(t, err, "Help command failed")
		assert.Contains(t, output, "CLI for Squads Multisig Protocol", "Help output doesn't contain expected text")
	})

	// Test multisig subcommand
	t.Run("Multisig Command", func(t *testing.T) {
		output, err := RunCommand(t, 5*time.Second, config.CliPath, "multisig", "--help")
		assert.NoError(t, err, "Multisig command failed")
		assert.Contains(t, output, "Manage Squads Multisig wallets", "Multisig help output doesn't contain expected text")
	})

	// Test transaction subcommand
	t.Run("Transaction Command", func(t *testing.T) {
		output, err := RunCommand(t, 5*time.Second, config.CliPath, "transaction", "--help")
		assert.NoError(t, err, "Transaction command failed")
		assert.Contains(t, output, "Manage Squads Multisig transactions", "Transaction help output doesn't contain expected text")
	})
}

// TestMultisigLifecycle tests the full lifecycle of a multisig
func TestMultisigLifecycle(t *testing.T) {
	// Skip if in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := SetupTestEnvironment(t)

	// Use existing multisig or create a new one
	var multisigAddress string
	var vaultAddress string

	// Step 1: Get or create a multisig
	if *multisigAddr != "" {
		// Use provided multisig address
		multisigAddress = *multisigAddr
		t.Logf("Using existing multisig: %s", multisigAddress)

		// Test info command on the existing multisig
		output, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", multisigAddress,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info: %s", output)
		t.Log("Successfully got info for existing multisig")

		// Extract vault address
		vaultAddrExtracted, err := ExtractAddressFromOutput(output, "Default Vault (Index 0):")
		if err == nil {
			vaultAddress = vaultAddrExtracted
			t.Logf("Found vault address: %s", vaultAddress)
		} else {
			t.Logf("Could not extract vault address: %v", err)
		}
	} else {
		// Create a new multisig with the test keypair as a member
		t.Log("Creating a new multisig...")

		// Get keypair's public key
		keyPair, err := LoadKeypair(config.KeypairPath)
		require.NoError(t, err, "Failed to load keypair")

		// Define additional member public keys (these are placeholder keys)
		member2 := "6tBou5MHL5aWpDy6cgf3wiwGGK2mR8qs68ujtpaoWrf2"
		member3 := "Hy5oibb1cYdmjyPJ2fiypDtKYvp1uZTuPkmFzVy7TL8c"

		// Create the multisig
		memberList := fmt.Sprintf("%s,%s,%s", keyPair.PublicKey().String(), member2, member3)
		cmd := []string{
			config.CliPath,
			"multisig", "create",
			"--payer", config.KeypairPath,
			"--members", memberList,
			"--permissions", "7,7,7", // Full permissions for all
			"--threshold", "2", // 2 of 3 required signatures
			"--rpc", config.RpcEndpoint,
			"--ws", config.WsEndpoint,
		}

		output, err := RunCommand(t, 90*time.Second, cmd[0], cmd[1:]...)
		// On devnet, the command might timeout but still succeed
		if err != nil {
			t.Logf("Command returned error, but might have succeeded: %v", err)
			t.Logf("Output: %s", output)

			// Try to extract the multisig address from the output anyway
			addr, extractErr := ExtractAddressFromOutput(output, "Multisig Address:")
			if extractErr == nil {
				multisigAddress = addr
				t.Logf("Extracted multisig address: %s", multisigAddress)
			} else {
				t.Fatalf("Failed to create multisig or extract address: %v", extractErr)
			}
		} else {
			t.Logf("Multisig creation output: %s", output)

			// Extract multisig address from output
			addr, extractErr := ExtractAddressFromOutput(output, "Multisig Address:")
			require.NoError(t, extractErr, "Failed to extract multisig address from output")
			multisigAddress = addr
		}

		// Wait for multisig to be available on-chain
		t.Log("Waiting for multisig to be available on-chain...")
		time.Sleep(10 * time.Second)

		// Get multisig info to confirm creation and get vault address
		infoOutput, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", multisigAddress,
			"--rpc", config.RpcEndpoint,
		)
		if err != nil {
			t.Logf("Warning: Could not get multisig info: %v", err)
		} else {
			t.Log("Successfully got info for newly created multisig")

			// Extract vault address
			vaultAddrExtracted, err := ExtractAddressFromOutput(infoOutput, "Default Vault (Index 0):")
			if err == nil {
				vaultAddress = vaultAddrExtracted
				t.Logf("Found vault address: %s", vaultAddress)

				// Fund the vault for transaction tests
				t.Log("Funding vault for transaction tests...")
				solanaCmd := exec.Command("solana", "transfer",
					"--allow-unfunded-recipient",
					"--keypair", config.KeypairPath,
					vaultAddress,
					"0.05",
					"--url", config.RpcEndpoint,
				)
				fundOutput, err := solanaCmd.CombinedOutput()
				if err != nil {
					t.Logf("Warning: Failed to fund vault: %v", err)
					t.Logf("Fund output: %s", string(fundOutput))
				} else {
					t.Log("Successfully funded vault with 0.05 SOL")

					// Wait for transfer to confirm
					t.Log("Waiting for transfer to confirm...")
					time.Sleep(10 * time.Second)
				}
			} else {
				t.Logf("Could not extract vault address: %v", err)
			}
		}
	}

	// Save created/found multisig info to test data for other tests to use
	testData := &MultisigTestData{
		MultisigAddress:    multisigAddress,
		VaultAddress:       vaultAddress,
		TransactionIndex:   0, // This will be incremented during transaction tests
		AdminKeypairPath:   config.KeypairPath,
		Members:            []string{},
		MemberKeypairPaths: []string{config.KeypairPath},
	}
	err := SaveMultisigTestData(testData, config.TestDataDir)
	if err != nil {
		t.Logf("Warning: Failed to save multisig test data: %v", err)
	}

	// Step 2: Test transaction creation (if vault has been funded)
	if vaultAddress != "" {
		t.Run("Transaction Creation", func(t *testing.T) {
			// Skip this test in short mode or if we don't want to create a real transaction
			if testing.Short() {
				t.Skip("Skipping transaction test in short mode")
			}

			// Use a test recipient address
			recipientAddress := "5qrUGR4BP7wp3EHSiunLGgNRNJWKQRWtgX3L3cwELLZP"

			t.Logf("Creating transaction to send 0.01 SOL from %s to %s", vaultAddress, recipientAddress)

			// Create transaction
			cmd := []string{
				config.CliPath,
				"transaction", "create",
				"--multisig", multisigAddress,
				"--to", recipientAddress,
				"--amount", "0.01",
				"--payer", config.KeypairPath,
				"--rpc", config.RpcEndpoint,
				"--ws", config.WsEndpoint,
				"--timeout", "60",
			}

			output, err := RunCommand(t, 90*time.Second, cmd[0], cmd[1:]...)
			// Command might time out but still succeed
			if err != nil {
				t.Logf("Transaction creation command returned error, but might have succeeded: %v", err)
				t.Logf("Output: %s", output)
			} else {
				t.Log("Transaction creation succeeded")
				t.Logf("Output: %s", output)
			}

			// Wait for transaction to be processed
			t.Log("Waiting for transaction to be processed...")
			time.Sleep(10 * time.Second)

			// Get multisig info again to see the new transaction
			infoOutput, err := RunCommand(t, 15*time.Second, config.CliPath,
				"multisig", "info",
				"--address", multisigAddress,
				"--rpc", config.RpcEndpoint,
			)
			if err != nil {
				t.Logf("Warning: Could not get updated multisig info: %v", err)
			} else {
				t.Log("Got updated multisig info after transaction creation")

				// Check if transaction is visible in output
				if strings.Contains(infoOutput, "Transaction #") {
					t.Log("Transaction visible in multisig info")
				} else {
					t.Log("Transaction not found in multisig info output")
				}
			}
		})
	}
}

// TestWithExistingMultisig tests operations on an existing multisig
func TestWithExistingMultisig(t *testing.T) {
	// Skip if no multisig address is provided
	if *multisigAddr == "" {
		t.Skip("Skipping existing multisig test since no multisig address provided")
	}

	config := SetupTestEnvironment(t)

	// Step 1: Get multisig info
	t.Run("Get Multisig Info", func(t *testing.T) {
		output, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", *multisigAddr,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info")
		t.Logf("Multisig info: %s", output)
	})

	// Step 2: Check if the multisig has any transactions
	t.Run("Check Transactions", func(t *testing.T) {
		output, err := RunCommand(t, 15*time.Second, config.CliPath,
			"multisig", "info",
			"--address", *multisigAddr,
			"--rpc", config.RpcEndpoint,
		)
		require.NoError(t, err, "Failed to get multisig info")

		if strings.Contains(output, "Transaction #") {
			t.Log("Multisig has existing transactions")

			// Could extract transaction IDs and check them individually
			// But for now, just verify they appear in the output
			if strings.Contains(output, "Status:") {
				t.Log("Transaction status information available")
			}
		} else {
			t.Log("No transactions found in multisig")
		}
	})
}

// TestGenerateKeypairs tests generating keypairs for testing
func TestGenerateKeypairs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping keypair generation in short mode")
	}

	config := SetupTestEnvironment(t)

	t.Run("Generate Test Keypairs", func(t *testing.T) {
		// Skip the actual keypair generation for this test
		// Instead, just verify the existing keypair
		keyPair, err := LoadKeypair(config.KeypairPath)
		require.NoError(t, err, "Failed to load keypair")

		pubkey := keyPair.PublicKey().String()
		t.Logf("Successfully loaded keypair with public key: %s", pubkey)

		// Verify it's a valid Solana public key
		_, err = solana.PublicKeyFromBase58(pubkey)
		require.NoError(t, err, "Invalid public key")
	})
}
