package multisig

import (
	"context"
	"errors"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/hogyzen12/squads-go/generated/squads_multisig_program"
)

// (the Permission* constants and data-types stay exactly the same)

// ---------------------------------------------------------------
// New high-level wrapper — note the new name
// ---------------------------------------------------------------
func CreateMultisigWithParams(
	ctx context.Context,
	p CreateParams,
) (sig solana.Signature,
	multisigPDA solana.PublicKey,
	createKey solana.PrivateKey,
	err error) {

	// 1. validation
	if len(p.Members) == 0 {
		err = errors.New("must supply at least one member")
		return
	}
	voters := 0
	for _, m := range p.Members {
		if m.Permissions&PermissionVote != 0 {
			voters++
		}
		if m.Permissions > PermissionFull {
			err = errors.New("permissions must be 0-7")
			return
		}
	}
	if p.Threshold == 0 || int(p.Threshold) > voters {
		err = errors.New("threshold must be ≤ #voting members")
		return
	}
	if p.ProgramID.IsZero() {
		p.ProgramID = solana.MustPublicKeyFromBase58(
			"SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	}

	// 2. clients
	rpcClient := rpc.New(p.RPCURL)
	wsClient, errWS := ws.Connect(ctx, p.WSURL)
	if errWS != nil {
		err = errWS
		return
	}
	defer wsClient.Close()

	// 3. translate members
	gen := make([]squads_multisig_program.Member, len(p.Members))
	for i, m := range p.Members {
		gen[i] = squads_multisig_program.Member{
			Key: m.Key,
			Permissions: squads_multisig_program.Permissions{
				Mask: m.Permissions,
			},
		}
	}

	// 4. delegate to the original low-level helper
	createKey = solana.NewWallet().PrivateKey
	sigStr, multisigPDA, errRaw := CreateMultisig(
		rpcClient,
		wsClient,
		p.Payer,
		createKey,
		gen,
		p.Threshold,
		p.TimeLock,
		p.ProgramID,
	)
	if errRaw != nil {
		err = errRaw
		return
	}
	sig, err = solana.SignatureFromBase58(sigStr)
	return
}
