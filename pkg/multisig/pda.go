package multisig

import (
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

var (
	seedPrefix        = []byte("multisig")
	seedProgramConfig = []byte("program_config")
	seedMultisig      = []byte("multisig")
)

func GetProgramConfigPDA(programID ...solana.PublicKey) (solana.PublicKey, uint8) {
	pid := solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	if len(programID) > 0 {
		pid = programID[0]
	}

	seeds := [][]byte{
		seedPrefix,
		seedProgramConfig,
	}

	pda, bump, err := solana.FindProgramAddress(seeds, pid)
	if err != nil {
		panic(fmt.Sprintf("Failed to find program config PDA: %v", err))
	}

	return pda, bump
}

func GetMultisigPDA(createKey solana.PublicKey, programID ...solana.PublicKey) (solana.PublicKey, uint8) {
	pid := solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	if len(programID) > 0 {
		pid = programID[0]
	}

	seeds := [][]byte{
		seedPrefix,
		seedMultisig,
		createKey.Bytes(),
	}

	pda, bump, err := solana.FindProgramAddress(seeds, pid)
	if err != nil {
		panic(fmt.Sprintf("Failed to find multisig PDA: %v", err))
	}

	return pda, bump
}

func GetVaultPDA(
	multisigPDA solana.PublicKey,
	vaultIndex uint8,
	programID ...solana.PublicKey,
) (solana.PublicKey, uint8) {
	pid := solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	if len(programID) > 0 {
		pid = programID[0]
	}

	seeds := [][]byte{
		seedPrefix,
		multisigPDA.Bytes(),
		[]byte("vault"),
		{vaultIndex},
	}

	pda, bump, err := solana.FindProgramAddress(seeds, pid)
	if err != nil {
		panic(fmt.Sprintf("Failed to find vault PDA: %v", err))
	}

	return pda, bump
}

func GetTransactionPDA(
	multisigPDA solana.PublicKey,
	transactionIndex uint64,
	programID ...solana.PublicKey,
) (solana.PublicKey, uint8) {
	pid := solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	if len(programID) > 0 {
		pid = programID[0]
	}

	seeds := [][]byte{
		seedPrefix,
		multisigPDA.Bytes(),
		[]byte("transaction"),
		uint64ToBytes(transactionIndex),
	}

	pda, bump, err := solana.FindProgramAddress(seeds, pid)
	if err != nil {
		panic(fmt.Sprintf("Failed to find transaction PDA: %v", err))
	}

	return pda, bump
}

func GetProposalPDA(
	multisigPDA solana.PublicKey,
	transactionIndex uint64,
	programID ...solana.PublicKey,
) (solana.PublicKey, uint8) {
	pid := solana.MustPublicKeyFromBase58("SQDS4ep65T869zMMBKyuUq6aD6EgTu8psMjkvj52pCf")
	if len(programID) > 0 {
		pid = programID[0]
	}

	seeds := [][]byte{
		seedPrefix,
		multisigPDA.Bytes(),
		[]byte("transaction"),
		uint64ToBytes(transactionIndex),
		[]byte("proposal"),
	}

	pda, bump, err := solana.FindProgramAddress(seeds, pid)
	if err != nil {
		panic(fmt.Sprintf("Failed to find proposal PDA: %v", err))
	}

	return pda, bump
}

// Helper function to convert uint64 to byte slice
func uint64ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, value)
	return bytes
}
