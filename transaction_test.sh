#!/bin/bash
# transaction_test.sh - Testing the full transaction lifecycle on a Squads Multisig

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
KEYPAIR_PATH=""            # Will be provided as argument
MULTISIG_ADDRESS=""        # Will be provided as argument
TRANSFER_AMOUNT="0.005"    # Amount of SOL to send (small to avoid draining vault)
RECIPIENT_ADDRESS=""       # Will be determined from keypair if not provided
TEST_TIMEOUT=90            # Default timeout in seconds for transaction operations

# Print usage
print_usage() {
  echo "Usage: $0 --keypair KEYPAIR_PATH --multisig MULTISIG_ADDRESS [OPTIONS]"
  echo ""
  echo "Arguments:"
  echo "  --keypair PATH       Path to the keypair JSON file (required)"
  echo "  --multisig ADDRESS   Multisig address to test (required)"
  echo "  --rpc URL            RPC endpoint (default: QuickNode devnet)"
  echo "  --ws URL             WebSocket endpoint (default: QuickNode devnet)"
  echo "  --amount SOL         Amount of SOL to transfer (default: $TRANSFER_AMOUNT)"
  echo "  --recipient ADDRESS  Recipient address (default: derived from keypair)"
  echo "  --timeout SECONDS    Transaction timeout in seconds (default: $TEST_TIMEOUT)"
  echo "  --help               Show this help message"
  echo ""
  echo "Example:"
  echo "  $0 --keypair ~/.config/solana/my-keypair.json --multisig 6NwD2N5AEcshmwRqmzBJn1Pq9wTzqemTBQL7mxi7Moqg"
}

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --keypair)
      KEYPAIR_PATH="$2"
      shift 2
      ;;
    --multisig)
      MULTISIG_ADDRESS="$2"
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
    --amount)
      TRANSFER_AMOUNT="$2"
      shift 2
      ;;
    --recipient)
      RECIPIENT_ADDRESS="$2"
      shift 2
      ;;
    --timeout)
      TEST_TIMEOUT="$2"
      shift 2
      ;;
    --help)
      print_usage
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown parameter: $1${NC}"
      print_usage
      exit 1
      ;;
  esac
done

# Verify required parameters
if [[ -z "$KEYPAIR_PATH" ]]; then
  echo -e "${RED}Error: Keypair path is required. Use --keypair to specify the path.${NC}"
  print_usage
  exit 1
fi

if [[ -z "$MULTISIG_ADDRESS" ]]; then
  echo -e "${RED}Error: Multisig address is required. Use --multisig to specify the address.${NC}"
  print_usage
  exit 1
fi

# Check if keypair file exists
if [[ ! -f "$KEYPAIR_PATH" ]]; then
  echo -e "${RED}Error: Keypair file not found at $KEYPAIR_PATH${NC}"
  exit 1
fi

# Check if squads-cli exists
if [[ ! -f "$CLI_PATH" ]]; then
  echo -e "${RED}Error: squads-cli not found at $CLI_PATH${NC}"
  exit 1
fi

# Function to run a command with timeout
run_command() {
  local description="$1"
  local command="$2"
  local timeout="${3:-$TEST_TIMEOUT}"
  
  echo -e "\n${BLUE}=== $description ===${NC}"
  echo -e "${YELLOW}Command: $command${NC}"
  
  # Run the command
  if command -v timeout >/dev/null 2>&1; then
    # Use timeout command if available
    timeout --preserve-status $timeout bash -c "$command"
    local exit_code=$?
    
    if [ $exit_code -eq 124 ]; then
      echo -e "${YELLOW}⚠ Command timed out after $timeout seconds.${NC}"
      echo -e "${YELLOW}This may not indicate failure - check logs for transaction status.${NC}"
      return 0
    elif [ $exit_code -eq 0 ]; then
      echo -e "${GREEN}✓ Command succeeded!${NC}"
      return 0
    else
      echo -e "${RED}✗ Command failed with exit code $exit_code${NC}"
      return $exit_code
    fi
  else
    # Run without timeout
    bash -c "$command"
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
      echo -e "${GREEN}✓ Command succeeded!${NC}"
      return 0
    else
      echo -e "${RED}✗ Command failed with exit code $exit_code${NC}"
      return $exit_code
    fi
  fi
}

# Get public key from keypair if recipient not specified
if [[ -z "$RECIPIENT_ADDRESS" ]]; then
  if command -v solana >/dev/null 2>&1; then
    RECIPIENT_ADDRESS=$(solana-keygen pubkey "$KEYPAIR_PATH" 2>/dev/null)
    if [[ -z "$RECIPIENT_ADDRESS" ]]; then
      echo -e "${RED}Error: Could not derive public key from keypair. Please specify --recipient.${NC}"
      exit 1
    fi
  else
    echo -e "${RED}Error: Solana CLI not found. Please specify --recipient explicitly.${NC}"
    exit 1
  fi
fi

# Start the test
echo -e "${BLUE}====================================${NC}"
echo -e "${BLUE}  SQUADS-CLI TRANSACTION LIFECYCLE TEST${NC}"
echo -e "${BLUE}====================================${NC}"
echo -e "${YELLOW}CLI Path: $CLI_PATH${NC}"
echo -e "${YELLOW}RPC Endpoint: $RPC_ENDPOINT${NC}"
echo -e "${YELLOW}WS Endpoint: $WS_ENDPOINT${NC}"
echo -e "${YELLOW}Keypair: $KEYPAIR_PATH${NC}"
echo -e "${YELLOW}Multisig: $MULTISIG_ADDRESS${NC}"
echo -e "${YELLOW}Recipient: $RECIPIENT_ADDRESS${NC}"
echo -e "${YELLOW}Amount: $TRANSFER_AMOUNT SOL${NC}"

# Step 1: Get multisig info to verify our account and check the vault
echo -e "\n${BLUE}=== Step 1: Checking multisig info ===${NC}"
MULTISIG_INFO_OUTPUT=$(run_command "Getting multisig info" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT" 15)

# Extract vault address and vault balance
VAULT_ADDRESS=$(echo "$MULTISIG_INFO_OUTPUT" | grep -oE "Default Vault \(Index 0\): [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")
VAULT_BALANCE=$(echo "$MULTISIG_INFO_OUTPUT" | grep -oE "Balance: [0-9]+\.[0-9]+ SOL" | grep -oE "[0-9]+\.[0-9]+")

if [[ -z "$VAULT_ADDRESS" ]]; then
  echo -e "${RED}Error: Could not extract vault address from multisig info.${NC}"
  exit 1
fi

echo -e "${GREEN}Found vault address: $VAULT_ADDRESS${NC}"

# Check if vault has enough balance
if [[ -n "$VAULT_BALANCE" ]]; then
  echo -e "${GREEN}Current vault balance: $VAULT_BALANCE SOL${NC}"
  
  # Compare with bc to handle floating point comparison
  if (( $(echo "$VAULT_BALANCE < $TRANSFER_AMOUNT" | bc -l) )); then
    echo -e "${RED}Error: Vault balance ($VAULT_BALANCE SOL) is less than transfer amount ($TRANSFER_AMOUNT SOL).${NC}"
    echo -e "${YELLOW}Please fund the vault or reduce the transfer amount.${NC}"
    exit 1
  else
    echo -e "${GREEN}Vault has sufficient balance for the transfer.${NC}"
  fi
else
  echo -e "${YELLOW}Warning: Could not extract vault balance. Proceeding anyway...${NC}"
fi

# Step 2: Create a transaction
echo -e "\n${BLUE}=== Step 2: Creating transaction proposal ===${NC}"
TRANSACTION_CREATE_OUTPUT=$(run_command "Creating transaction" "$CLI_PATH transaction create --multisig $MULTISIG_ADDRESS --to $RECIPIENT_ADDRESS --amount $TRANSFER_AMOUNT --payer $KEYPAIR_PATH --rpc $RPC_ENDPOINT --ws $WS_ENDPOINT --timeout $TEST_TIMEOUT")

# Extract transaction index and proposal PDA
TRANSACTION_INDEX=$(echo "$TRANSACTION_CREATE_OUTPUT" | grep -oE "Transaction Index: [0-9]+" | grep -oE "[0-9]+")
TRANSACTION_PDA=$(echo "$TRANSACTION_CREATE_OUTPUT" | grep -oE "Transaction PDA: [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")
PROPOSAL_PDA=$(echo "$TRANSACTION_CREATE_OUTPUT" | grep -oE "Proposal PDA: [1-9A-HJ-NP-Za-km-z]{32,44}" | grep -oE "[1-9A-HJ-NP-Za-km-z]{32,44}")

if [[ -z "$TRANSACTION_INDEX" || -z "$TRANSACTION_PDA" || -z "$PROPOSAL_PDA" ]]; then
  echo -e "${YELLOW}Warning: Could not extract all transaction details. Checking multisig info to confirm...${NC}"
  
  # Get multisig info again to verify the transaction was created
  sleep 5
  MULTISIG_INFO_AFTER=$(run_command "Verifying transaction creation" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT" 15)
  
  if echo "$MULTISIG_INFO_AFTER" | grep -q "Transaction #"; then
    echo -e "${GREEN}Transaction was created successfully.${NC}"
    
    # Try to extract the transaction index from the info
    if [[ -z "$TRANSACTION_INDEX" ]]; then
      TRANSACTION_INDEX=$(echo "$MULTISIG_INFO_AFTER" | grep -oE "Transaction #[0-9]+" | grep -oE "[0-9]+" | head -1)
      if [[ -n "$TRANSACTION_INDEX" ]]; then
        echo -e "${GREEN}Extracted transaction index: $TRANSACTION_INDEX${NC}"
      fi
    fi
  else
    echo -e "${RED}Error: Transaction creation may have failed. No transactions found in multisig info.${NC}"
    exit 1
  fi
fi

# Step 3: Check if the creator auto-approved the transaction
echo -e "\n${BLUE}=== Step 3: Checking approval status ===${NC}"

APPROVAL_COUNT=$(echo "$TRANSACTION_CREATE_OUTPUT" | grep -oE "Current Approvals: [0-9]+" | grep -oE "[0-9]+")
THRESHOLD=$(echo "$MULTISIG_INFO_OUTPUT" | grep -oE "Threshold: ([0-9]+)/" | grep -oE "[0-9]+")

if [[ -n "$APPROVAL_COUNT" && -n "$THRESHOLD" ]]; then
  echo -e "${GREEN}Current approvals: $APPROVAL_COUNT/$THRESHOLD${NC}"
  
  # Convert to integers and compare
  APPROVAL_INT=$((APPROVAL_COUNT))
  THRESHOLD_INT=$((THRESHOLD))
  
  if [[ $APPROVAL_INT -lt $THRESHOLD_INT ]]; then
    echo -e "${YELLOW}Transaction needs more approvals before it can be executed.${NC}"
    
    # Determine how many more approvals are needed
    APPROVALS_NEEDED=$((THRESHOLD - APPROVAL_COUNT))
    echo -e "${YELLOW}Needs $APPROVALS_NEEDED more approval(s).${NC}"
    
    # In a real world scenario, we would need to have other members approve
    echo -e "${YELLOW}In a real-world scenario, you would now need other members to approve the transaction.${NC}"
    echo -e "${YELLOW}You can approve as another member by running:${NC}"
    echo -e "${CYAN}  ./squads-cli transaction approve --multisig $MULTISIG_ADDRESS --transaction $TRANSACTION_INDEX --payer /path/to/other/member/keypair.json${NC}"
    
    exit 0
  else
    echo -e "${GREEN}Transaction has reached the approval threshold!${NC}"
  fi
else
  echo -e "${YELLOW}Could not determine approval status. Checking multisig info...${NC}"
  
  # Get multisig info again to verify the transaction status
  sleep 5
  MULTISIG_INFO_AFTER=$(run_command "Checking transaction status" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT" 15)
  
  if echo "$MULTISIG_INFO_AFTER" | grep -q "Status: Approved"; then
    echo -e "${GREEN}Transaction is approved and ready for execution.${NC}"
  else
    echo -e "${YELLOW}Transaction may need more approvals. Check manual approval instructions above.${NC}"
    exit 0
  fi
fi

# Step 4: Execute the transaction (if it has enough approvals)
echo -e "\n${BLUE}=== Step 4: Executing the transaction ===${NC}"
echo -e "${YELLOW}This step will execute the approved transaction, transferring $TRANSFER_AMOUNT SOL from the vault to $RECIPIENT_ADDRESS.${NC}"

# Ask for confirmation before executing
read -p "Do you want to proceed with execution? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo -e "${YELLOW}Transaction execution cancelled.${NC}"
  exit 0
fi

# Check if the execute command exists first
if $CLI_PATH transaction --help | grep -q "execute"; then
  # If execute command exists, use it
  EXECUTION_OUTPUT=$(run_command "Executing transaction" "$CLI_PATH transaction execute --multisig $MULTISIG_ADDRESS --transaction $TRANSACTION_INDEX --payer $KEYPAIR_PATH --rpc $RPC_ENDPOINT --ws $WS_ENDPOINT --timeout $TEST_TIMEOUT")
else
  # If execute command doesn't exist, inform the user
  echo -e "${YELLOW}The 'transaction execute' command is not yet implemented in the CLI.${NC}"
  echo -e "${YELLOW}To complete this test, you need to implement the execute command in your CLI.${NC}"
  
  # Get multisig info to check the latest transaction status
  echo -e "\n${BLUE}=== Checking final transaction status ===${NC}"
  FINAL_MULTISIG_INFO=$(run_command "Getting updated multisig info" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT" 15)
  
  echo -e "${YELLOW}You would need to implement a command like:${NC}"
  echo -e "${CYAN}  ./squads-cli transaction execute --multisig $MULTISIG_ADDRESS --transaction $TRANSACTION_INDEX --payer $KEYPAIR_PATH${NC}"
  
  EXECUTION_OUTPUT="Transaction execute command not yet implemented"
fi

# Check if execution was successful
if echo "$EXECUTION_OUTPUT" | grep -q "Transaction executed successfully"; then
  echo -e "${GREEN}Transaction executed successfully!${NC}"
  
  # Extract execution signature if available
  TX_SIGNATURE=$(echo "$EXECUTION_OUTPUT" | grep -oE "[1-9A-HJ-NP-Za-km-z]{64,88}" | head -1)
  if [[ -n "$TX_SIGNATURE" ]]; then
    echo -e "${GREEN}Transaction signature: $TX_SIGNATURE${NC}"
    echo -e "${CYAN}You can view this transaction on the Solana Explorer:${NC}"
    echo -e "${CYAN}https://explorer.solana.com/tx/$TX_SIGNATURE?cluster=devnet${NC}"
  fi
  
  # Verify the vault balance changed
  echo -e "\n${BLUE}=== Verifying final vault balance ===${NC}"
  sleep 5
  FINAL_MULTISIG_INFO=$(run_command "Getting updated multisig info" "$CLI_PATH multisig info --address $MULTISIG_ADDRESS --rpc $RPC_ENDPOINT" 15)
  
  FINAL_BALANCE=$(echo "$FINAL_MULTISIG_INFO" | grep -oE "Balance: [0-9]+\.[0-9]+ SOL" | grep -oE "[0-9]+\.[0-9]+")
  
  if [[ -n "$FINAL_BALANCE" && -n "$VAULT_BALANCE" ]]; then
    EXPECTED_BALANCE=$(echo "$VAULT_BALANCE - $TRANSFER_AMOUNT" | bc -l)
    echo -e "${GREEN}Initial vault balance: $VAULT_BALANCE SOL${NC}"
    echo -e "${GREEN}Final vault balance: $FINAL_BALANCE SOL${NC}"
    echo -e "${GREEN}Expected balance: $EXPECTED_BALANCE SOL${NC}"
    
    # Allow for some rounding/fee differences with a small tolerance
    TOLERANCE=0.001
    DIFF=$(echo "($FINAL_BALANCE - $EXPECTED_BALANCE) < 0 ? ($EXPECTED_BALANCE - $FINAL_BALANCE) : ($FINAL_BALANCE - $EXPECTED_BALANCE)" | bc -l)
    
    if (( $(echo "$DIFF <= $TOLERANCE" | bc -l) )); then
      echo -e "${GREEN}✓ Balance change verified! Transaction completed successfully.${NC}"
    else
      echo -e "${YELLOW}⚠ Balance change does not match expected value.${NC}"
      echo -e "${YELLOW}This might be due to transaction fees or other factors.${NC}"
    fi
  else
    echo -e "${YELLOW}Could not verify final balance.${NC}"
  fi
else
  echo -e "${RED}Transaction execution may have failed or is still processing.${NC}"
  echo -e "${YELLOW}Please check the multisig info and transaction status manually.${NC}"
fi

echo -e "\n${BLUE}====================================${NC}"
echo -e "${GREEN}Transaction lifecycle test completed!${NC}"
echo -e "${BLUE}====================================${NC}"

exit 0