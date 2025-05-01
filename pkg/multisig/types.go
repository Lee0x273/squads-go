package multisig

import "github.com/gagliardetto/solana-go"

// -------------------------------------------------------------------
// Permission bits – one canonical definition for the whole package.
// -------------------------------------------------------------------
const (
	PermissionPropose uint8 = 1 << 0
	PermissionVote    uint8 = 1 << 1
	PermissionExecute uint8 = 1 << 2
	PermissionFull          = PermissionPropose | PermissionVote | PermissionExecute
)

// -------------------------------------------------------------------
// Re-usable data holders
// -------------------------------------------------------------------
type Member struct {
	Key         solana.PublicKey
	Permissions uint8
}

type CreateParams struct {
	RPCURL    string
	WSURL     string
	Payer     solana.PrivateKey
	Members   []Member
	Threshold uint16
	TimeLock  uint32
	ProgramID solana.PublicKey
}
