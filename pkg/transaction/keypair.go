package transaction

import (
	"encoding/json"
	"os"

	"github.com/gagliardetto/solana-go"
)

// LoadKeypair loads a keypair from a JSON file
func LoadKeypair(path string) (solana.PrivateKey, error) {
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
