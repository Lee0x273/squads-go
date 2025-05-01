package multisig

import (
	"context"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/hogyzen12/squads-go/generated/squads_multisig_program"
)

// MultisigInfo is returned to callers (GUI, etc.).
type MultisigInfo struct {
	Address               solana.PublicKey
	Threshold             uint16
	TimeLock              uint32
	Members               []squads_multisig_program.Member
	DefaultVault          solana.PublicKey
	TransactionIndex      uint64
	StaleTransactionIndex uint64
}

// FetchMultisigInfo loads the on-chain account and decodes it.
func FetchMultisigInfo(
	ctx context.Context,
	rpcURL string,
	addr solana.PublicKey,
) (*MultisigInfo, error) {

	client := rpc.New(rpcURL)

	acc, err := client.GetAccountInfo(ctx, addr)
	if err != nil {
		return nil, err
	}

	// Decode just like cmd/multisig-info does.
	var ms squads_multisig_program.Multisig
	dec := ag_binary.NewBorshDecoder(acc.Value.Data.GetBinary())
	if err := ms.UnmarshalWithDecoder(dec); err != nil {
		return nil, err
	}

	// Derive the default vault (index 0) for convenience.
	vault0, _ := GetVaultPDA(addr, 0)

	info := &MultisigInfo{
		Address:               addr,
		Threshold:             ms.Threshold,
		TimeLock:              ms.TimeLock,
		Members:               ms.Members,
		DefaultVault:          vault0,
		TransactionIndex:      ms.TransactionIndex,
		StaleTransactionIndex: ms.StaleTransactionIndex,
	}
	return info, nil
}
