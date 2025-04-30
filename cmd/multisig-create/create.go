package multisigcreate

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/spf13/cobra"

	"squads-go/generated/squads_multisig_program"
	"squads-go/pkg/multisig"
)

// Define permission masks
const (
	PermissionPropose uint8 = 1 << 0                                                 // 1 - Can create proposals
	PermissionVote    uint8 = 1 << 1                                                 // 2 - Can vote on proposals
	PermissionExecute uint8 = 1 << 2                                                 // 4 - Can execute proposals
	PermissionFull    uint8 = PermissionPropose | PermissionVote | PermissionExecute // 7 - Full permissions
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Squads Multisig",
		Long: `Create a new Squads Multisig with custom member permissions and threshold.

Permissions Breakdown:
  - 1 (Propose): Can create new proposals
  - 2 (Vote): Can vote on proposals
  - 4 (Execute): Can execute approved proposals
  - 7 (Full): Can propose, vote, and execute

Threshold Requirement:
The threshold MUST be less than or equal to the number of members with VOTE permission.

Examples:
  # 3-of-3 multisig with all members having full permissions
  squads-cli create --payer /path/to/payer.json \
    --members member1,member2,member3 \
    --permissions 7,7,7 \
    --threshold 3

  # 2-of-3 multisig with mixed permissions
  squads-cli create --payer /path/to/payer.json \
    --members member1,member2,member3 \
    --permissions 7,2,2 \  # First member full, others vote-only
    --threshold 2

  # INVALID: Threshold (3) exceeds voting members (2)
  squads-cli create --payer /path/to/payer.json \
    --members member1,member2,member3 \
    --permissions 1,1,1 \  # No voting members
    --threshold 3  # THIS WILL FAIL
`,
		Run: runCreate,
	}

	cmd.Flags().StringP("payer", "p", "", "Path to payer keypair JSON (REQUIRED)")
	cmd.Flags().Uint16P("threshold", "t", 2, "Multisig signature threshold")
	cmd.Flags().Uint32P("timelock", "l", 0, "Timelock duration in seconds")
	cmd.Flags().StringSliceP("members", "m", []string{
		"6tBou5MHL5aWpDy6cgf3wiwGGK2mR8qs68ujtpaoWrf2",
		"Hy5oibb1cYdmjyPJ2fiypDtKYvp1uZTuPkmFzVy7TL8c",
	}, "Member public keys")
	cmd.Flags().IntSliceP("permissions", "P", []int{7, 5}, "Permissions for each member (1=Propose, 2=Vote, 4=Execute, 7=Full)")
	cmd.MarkFlagRequired("payer")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) {
	// Get RPC endpoints
	rpcEndpoint, _ := cmd.Parent().Flags().GetString("rpc")
	wsEndpoint, _ := cmd.Parent().Flags().GetString("ws")

	// Load payer keypair
	payerPath, _ := cmd.Flags().GetString("payer")
	payer, err := loadKeypair(payerPath)
	if err != nil {
		log.Fatalf("Failed to load payer keypair: %v", err)
	}

	// Get threshold and timelock
	threshold, _ := cmd.Flags().GetUint16("threshold")
	timeLock, _ := cmd.Flags().GetUint32("timelock")

	// Get member keys and permissions
	memberKeys, _ := cmd.Flags().GetStringSlice("members")
	memberPermissions, _ := cmd.Flags().GetIntSlice("permissions")

	// Validate input
	if len(memberKeys) != len(memberPermissions) {
		log.Fatalf("Number of members (%d) must match number of permissions (%d)",
			len(memberKeys), len(memberPermissions))
	}

	// Prepare members with their permissions
	members := make([]squads_multisig_program.Member, len(memberKeys))
	// Validate threshold against voting members
	votingMemberCount := 0
	for _, permission := range memberPermissions {
		if uint8(permission)&PermissionVote != 0 {
			votingMemberCount++
		}
	}

	if uint16(votingMemberCount) < threshold {
		// Use the new explanation function
		errorMessage := explainThresholdError(memberKeys, memberPermissions, threshold)
		log.Fatalf("\n%s", errorMessage)
	}

	for i, keyStr := range memberKeys {
		memberKey, err := solana.PublicKeyFromBase58(keyStr)
		if err != nil {
			log.Fatalf("Invalid member public key %s: %v", keyStr, err)
		}

		// Validate permissions
		if memberPermissions[i] < 0 || memberPermissions[i] > 7 {
			log.Fatalf("Invalid permission value %d for member %s. Must be between 0-7.",
				memberPermissions[i], keyStr)
		}

		// Count voting members
		if uint8(memberPermissions[i])&PermissionVote != 0 {
			votingMemberCount++
		}

		members[i] = squads_multisig_program.Member{
			Key: memberKey,
			Permissions: squads_multisig_program.Permissions{
				Mask: uint8(memberPermissions[i]),
			},
		}
	}

	// Validate threshold against voting members
	if uint16(votingMemberCount) < threshold {
		log.Fatalf(
			"Invalid threshold: %d. Must be less than or equal to number of voting members (%d)",
			threshold,
			votingMemberCount,
		)
	}

	// Set up RPC and WebSocket clients
	client := rpc.New(rpcEndpoint)
	wsClient, err := ws.Connect(cmd.Context(), wsEndpoint)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer wsClient.Close()

	// Generate a create key
	createKey := solana.NewWallet().PrivateKey

	// Call multisig creation
	sig, multisigPDA, err := multisig.CreateMultisig(
		client,
		wsClient,
		payer,
		createKey,
		members,
		threshold,
		timeLock,
		solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf"),
	)
	if err != nil {
		log.Fatalf("Failed to create multisig: %v", err)
	}

	// Output results
	log.Printf("Multisig created successfully!")
	log.Printf("Create Key: %s", createKey.PublicKey().String())
	log.Printf("Multisig Address: %s", multisigPDA.String())
	log.Printf("Transaction Signature: %s", sig)

	// Print detailed member information
	log.Println("\nMultisig Configuration:")
	log.Printf("Threshold: %d voting members required", threshold)
	log.Println("\nMultisig Members:")
	for _, member := range members {
		permissionDesc := describePermissions(member.Permissions.Mask)
		log.Printf("- %s (Permissions: %s)", member.Key.String(), permissionDesc)
	}
}

// describePermissions converts the permission mask to a human-readable string
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

func loadKeypair(path string) (solana.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keyArray []byte
	if err := json.Unmarshal(keyBytes, &keyArray); err != nil {
		return nil, err
	}
	return solana.PrivateKey(keyArray), nil
}

func explainThresholdError(memberKeys []string, memberPermissions []int, threshold uint16) string {
	votingMembers := make([]string, 0)
	nonVotingMembers := make([]string, 0)

	for i, permission := range memberPermissions {
		if uint8(permission)&PermissionVote != 0 {
			votingMembers = append(votingMembers, memberKeys[i])
		} else {
			nonVotingMembers = append(nonVotingMembers, memberKeys[i])
		}
	}

	var explanation strings.Builder
	explanation.WriteString("Threshold Configuration Error:\n")
	explanation.WriteString(fmt.Sprintf("  Requested Threshold: %d\n", threshold))
	explanation.WriteString(fmt.Sprintf("  Voting Members Count: %d\n\n", len(votingMembers)))

	explanation.WriteString("Voting Members:\n")
	for _, member := range votingMembers {
		explanation.WriteString(fmt.Sprintf("  - %s\n", member))
	}

	explanation.WriteString("\nNon-Voting Members:\n")
	for _, member := range nonVotingMembers {
		explanation.WriteString(fmt.Sprintf("  - %s\n", member))
	}

	explanation.WriteString("\nTo resolve this issue, you have two options:\n")
	explanation.WriteString("1. Reduce the threshold to match the number of voting members\n")
	explanation.WriteString("2. Modify member permissions to include more voting members\n\n")

	explanation.WriteString("Permissions Explanation:\n")
	explanation.WriteString("  - 1 (Propose): Can create proposals\n")
	explanation.WriteString("  - 2 (Vote): Can vote on proposals ✓ COUNTS TOWARDS THRESHOLD\n")
	explanation.WriteString("  - 4 (Execute): Can execute proposals\n")
	explanation.WriteString("  - 7 (Full): Can propose, vote, and execute ✓ COUNTS TOWARDS THRESHOLD\n")

	return explanation.String()
}
