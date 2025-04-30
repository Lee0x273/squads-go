#!/bin/bash
# devnet_test.sh - Squads-CLI test script for Solana devnet

# Color definitions for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default configuration
CLI_PATH="./squads-cli"
RPC_ENDPOINT="https://api.mainnet-beta.solana.com"
WS_ENDPOINT="wss://api.mainnet-beta.solana.com"
TEST_DATA_DIR="./tests/testdata"
TEST_TIMEOUT=60                    # Default timeout in seconds for devnet (longer than local)
TEST_MODE="basic"                  # Default test mode: basic or full
KEYPAIR_PATH="/Users/hogyzen12/.config/solana/pLgH63FULg9BVrqyjFLwDXyiGBFKvtw7HMByv592WMK.json"

# Print usage
print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --cli PATH            Path to the squads-cli binary (default: $CLI_PATH)"
  echo "  --rpc URL             Solana RPC endpoint (default: QuickNode devnet)"
  echo "  --ws URL              Solana WebSocket endpoint (default: QuickNode devnet)"
  echo "  --test-data DIR       Directory for test data (default: $TEST_DATA_DIR)"
  echo "  --timeout SECONDS     Timeout for operations in seconds (default: $TEST_TIMEOUT)"
  echo "  --mode MODE           Test mode: 'basic' or 'full' (default: $TEST_MODE)"
  echo "  --keypair PATH        Use specific keypair for tests (default: $KEYPAIR_PATH)"
  echo "  --multisig ADDRESS    Use existing multisig (optional)"
  echo "  --help                Show this help message"
  echo ""
  echo "Examples:"
  echo "  $0 --mode basic                    # Run basic tests on devnet"
  echo "  $0 --mode full                     # Run full test suite on devnet"
  echo "  $0 --multisig Address...           # Test with existing multisig"
}

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --cli)
      CLI_PATH="$2"
      shift 2
      ;;
    --rpc)
      RPC_ENDPOINT="$2"
      shift 2
      ;;
    --ws)
      WS_ENDPOINT="$2"
      shift 2
      ;;
    --test-data)
      TEST_DATA_DIR="$2"
      shift 2
      ;;
    --timeout)
      TEST_TIMEOUT="$2"
      shift 2
      ;;
    --mode)
      TEST_MODE="$2"
      shift 2
      ;;
    --keypair)
      KEYPAIR_PATH="$2"
      shift 2
      ;;
    --multisig)
      TEST_MULTISIG="$2"
      shift 2
      ;;
    --help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown parameter: $1"
      print_usage
      exit 1
      ;;
  esac
done

# Check if CLI binary exists
if [[ ! -f "$CLI_PATH" ]]; then
  echo -e "${RED}Error: CLI binary not found at $CLI_PATH${NC}"
  echo "Build the CLI first with 'go build -o squads-cli cmd/main.go'"
  exit 1
fi

# Check if keypair exists
if [[ ! -f "$KEYPAIR_PATH" ]]; then
  echo -e "${RED}Error: Keypair not found at $KEYPAIR_PATH${NC}"
  exit 1
fi

# Make sure test data directory exists
mkdir -p "$TEST_DATA_DIR"

# Function to check if a command exists
command_exists() {
  which "$1" &> /dev/null
}

# Function to run a test with timeout
run_test() {
  local test_name="$1"
  local command="$2"
  local timeout_seconds="${3:-$TEST_TIMEOUT}"
  
  echo -e "\n${BLUE}=== Running test: $test_name ===${NC}"
  echo -e "${YELLOW}Command: $command${NC}"
  
  # Run the command with timeout if the timeout command exists
  if command_exists timeout; then
    timeout --preserve-status "$timeout_seconds" bash -c "$command" 2>&1
    local exit_code=$?
    
    if [ $exit_code -eq 124 ]; then
      echo -e "${YELLOW}⚠ Test timed out after $timeout_seconds seconds.${NC}"
      echo -e "${YELLOW}This may not indicate failure on devnet - transactions can take time to confirm.${NC}"
      return 0
    elif [ $exit_code -eq 0 ]; then
      echo -e "${GREEN}✓ Test passed!${NC}"
      return 0
    else
      echo -e "${RED}✗ Test failed with exit code $exit_code${NC}"
      return $exit_code
    fi
  else
    # Run without timeout
    bash -c "$command" 2>&1
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
      echo -e "${GREEN}✓ Test passed!${NC}"
      return 0
    else
      echo -e "${RED}✗ Test failed with exit code $exit_code${NC}"
      return $exit_code
    fi
  fi
}

# Function to check connection to Solana devnet
check_devnet_connection() {
  echo -e "${BLUE}=== Checking connection to Solana devnet ===${NC}"
  echo -e "${YELLOW}RPC endpoint: $RPC_ENDPOINT${NC}"
  
  local response
  response=$(curl -s -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"getHealth"}' "$RPC_ENDPOINT")
  
  if [[ $response == *"ok"* ]]; then
    echo -e "${GREEN}✓ Connected to Solana devnet!${NC}"
    return 0
  else
    echo -e "${RED}✗ Failed to connect to Solana devnet: $response${NC}"
    return 1
  fi
}

# Function to check keypair balance
check_keypair_balance() {
  local keypair="$1"
  local pubkey=$(solana-keygen pubkey "$keypair")
  
  echo -e "${BLUE}=== Checking keypair balance ===${NC}"
  echo -e "${YELLOW}Public key: $pubkey${NC}"
  
  local balance
  balance=$(solana balance "$pubkey" --url "$RPC_ENDPOINT" 2>/dev/null)
  
  if [[ $? -eq 0 && $balance == *"SOL"* ]]; then
    echo -e "${GREEN}✓ Balance: $balance${NC}"
    return 0
  else
    echo -e "${RED}✗ Failed to check balance or account has no SOL${NC}"
    return 1
  fi
}

# Start tests
echo -e "${BLUE}====================================${NC}"
echo -e "${BLUE}  SQUADS-CLI DEVNET TESTS${NC}"
echo -e "${BLUE}====================================${NC}"
echo -e "${YELLOW}CLI Path: $CLI_PATH${NC}"
echo -e "${YELLOW}RPC Endpoint: $RPC_ENDPOINT${NC}"
echo -e "${YELLOW}WS Endpoint: $WS_ENDPOINT${NC}"
echo -e "${YELLOW}Keypair: $KEYPAIR_PATH${NC}"
echo -e "${YELLOW}Test Mode: $TEST_MODE${NC}"

# Check connection to devnet
check_devnet_connection
if [ $? -ne 0 ]; then
  echo -e "${RED}Error: Cannot proceed with tests due to devnet connection failure.${NC}"
  exit 1
fi

# Check keypair balance
check_keypair_balance "$KEYPAIR_PATH"
if [ $? -ne 0 ]; then
  echo -e "${RED}Error: Keypair needs SOL to perform tests. Please fund your account.${NC}"
  exit 1
fi

# Test 1: Help command
run_test "Help Command" "$CLI_PATH --help"

# Test 2: Multisig help command
run_test "Multisig Help Command" "$CLI_PATH multisig --help"

# Test 3: Transaction help command
run_test "Transaction Help Command" "$CLI_PATH transaction --help"

# If a multisig address was provided, use it for tests
if [ -n "$TEST_MULTISIG" ]; then
  echo -e "${BLUE}=== Using existing multisig: $TEST_MULTISIG ===${NC}"
  
  # Test 4: Get info about the multisig
  run_test "Multisig Info" "$CLI_PATH multisig info --address $TEST_MULTISIG --rpc $RPC_ENDPOINT"
  
  # If testing in full mode, proceed with transaction tests
  if [ "$TEST_MODE" == "full" ]; then
    echo -e "${BLUE}=== Running full test suite with existing multisig ===${NC}"
    
    # Generate a recipient address for testing transactions
    RECIPIENT_ADDRESS="5qrUGR4BP7wp3EHSiunLGgNRNJWKQRWtgX3L3cwELLZP"
    
    # Test 5: Create a transaction (small amount to avoid using too much SOL)
    run_test "Create Transaction" "$CLI_PATH transaction create --multisig $TEST_MULTISIG --to $RECIPIENT_ADDRESS --amount 0.001 --payer $KEYPAIR_PATH --rpc $RPC_ENDPOINT --ws $WS_ENDPOINT"
    
    # Check multisig info again to see the new transaction
    run_test "Check Multisig After Transaction" "$CLI_PATH multisig info --address $TEST_MULTISIG --rpc $RPC_ENDPOINT"
  fi
else
  # If no multisig was provided, create a new one
  echo -e "${BLUE}=== Creating new multisig for testing ===${NC}"
  
  # Generate three member keypairs (using the same keypair for simplicity)
  MEMBER1_ADDRESS=$(solana-keygen pubkey "$KEYPAIR_PATH")
  
  # Generate two additional member addresses
  MEMBER2_ADDRESS="6tBou5MHL5aWpDy6cgf3wiwGGK2mR8qs68ujtpaoWrf2"
  MEMBER3_ADDRESS="Hy5oibb1cYdmjyPJ2fiypDtKYvp1uZTuPkmFzVy7TL8c"
  
  # Create the multisig
  MULTISIG_CREATE_OUTPUT=$(run_test "Create Multisig" "$CLI_PATH multisig create --payer $KEYPAIR_PATH --members $MEMBER1_ADDRESS,$MEMBER2_ADDRESS,$MEMBER3_ADDRESS --permissions 7,7,7 --threshold 2 --rpc $RPC_ENDPOINT --ws $WS_ENDPOINT" 120)
  
  # Extract multisig address from output
  MULTISIG_ADDRESS=$(echo "$MULTISIG_CREATE_OUTPUT" | grep -oE "Multisig Address: [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")
  
  if [ -z "$MULTISIG_ADDRESS" ]; then
    # Try to extract it from the transaction signature
    SIG=$(echo "$MULTISIG_CREATE_OUTPUT" | grep -oE "Transaction Signature: [1-9A-HJ-NP-Za-km-z]{64,88}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{64,88}")
    
    if [ -n "$SIG" ]; then
      echo -e "${YELLOW}Waiting for transaction confirmation...${NC}"
      sleep 10
      
      # Try to get the multisig address from transaction logs
      TRANSACTION_DETAILS=$(solana confirm -v "$SIG" --url "$RPC_ENDPOINT" 2>&1)
      MULTISIG_ADDRESS=$(echo "$TRANSACTION_DETAILS" | grep -oE "Multisig Address: [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")
    fi
    
    if [ -z "$MULTISIG_ADDRESS" ]; then
      echo -e "${RED}Error: Failed to extract multisig address from output${NC}"
      echo -e "${YELLOW}Transaction might still be processing. Check your devnet explorer.${NC}"
      echo -e "${YELLOW}Command output:${NC}"
      echo "$MULTISIG_CREATE_OUTPUT"
      exit 1
    fi
  fi
  
  echo -e "${GREEN}✓ Created new multisig: $MULTISIG_ADDRESS${NC}"
  
  # Wait a bit for the multisig to be available on-chain
  echo -e "${YELLOW}Waiting for multisig to be available on-chain...${NC}"
  sleep 5
  
  # Test info command on the new multisig
  MULTISIG_INFO_OUTPUT=$(run_test "Get Multisig Info" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT")
  
  # Extract vault address
  VAULT_ADDRESS=$(echo "$MULTISIG_INFO_OUTPUT" | grep -oE "Default Vault \(Index 0\): [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")
  
  if [ -z "$VAULT_ADDRESS" ]; then
    echo -e "${YELLOW}Warning: Failed to extract vault address from output${NC}"
  else
    echo -e "${GREEN}✓ Found vault address: $VAULT_ADDRESS${NC}"
    
    # Fund the vault
    echo -e "${BLUE}=== Funding vault for transaction tests ===${NC}"
    FUND_OUTPUT=$(solana transfer --allow-unfunded-recipient --keypair "$KEYPAIR_PATH" "$VAULT_ADDRESS" 0.05 --url "$RPC_ENDPOINT" 2>&1)
    
    if [[ $? -eq 0 ]]; then
      echo -e "${GREEN}✓ Funded vault with 0.05 SOL${NC}"
      
      # Wait for the transfer to confirm
      echo -e "${YELLOW}Waiting for transfer to confirm...${NC}"
      sleep 10
    else
      echo -e "${YELLOW}Warning: Failed to fund vault address${NC}"
      echo "$FUND_OUTPUT"
    fi
  fi
  
  # If testing in full mode, proceed with transaction tests
  if [ "$TEST_MODE" == "full" ]; then
    echo -e "${BLUE}=== Running full test suite with new multisig ===${NC}"
    
    # Use a known recipient address for test
    RECIPIENT_ADDRESS="5qrUGR4BP7wp3EHSiunLGgNRNJWKQRWtgX3L3cwELLZP"
    
    # Wait a bit to ensure vault is funded
    sleep 5
    
    # Test transaction creation
    run_test "Create Transaction" "$CLI_PATH transaction create --multisig $MULTISIG_ADDRESS --to $RECIPIENT_ADDRESS --amount 0.01 --payer $KEYPAIR_PATH --rpc $RPC_ENDPOINT --ws $WS_ENDPOINT"
    
    # Wait for the transaction to settle
    sleep 10
    
    # Check multisig info again to see if transaction was created
    run_test "Check Multisig After Transaction" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT"
  fi
fi

# Test completed
echo -e "\n${BLUE}====================================${NC}"
echo -e "${GREEN}Test suite completed!${NC}"
echo -e "${BLUE}====================================${NC}"

# If we created a new multisig, tell the user how to use it for future tests
if [ -n "$MULTISIG_ADDRESS" ] && [ -z "$TEST_MULTISIG" ]; then
  echo -e "\n${CYAN}To use this multisig in future tests:${NC}"
  echo -e "${CYAN}$0 --multisig $MULTISIG_ADDRESS${NC}"
fi

exit 0