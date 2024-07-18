//go:build go1.20
// +build go1.20

package genesis

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

var ContractDeployerKey *ecdsa.PrivateKey

func init() {
	D := &big.Int{}
	D.SetString("7472616374206b65e45ffeb29e933944f5027ef139f124f430641487e70ea9a1", 16)
	X := &big.Int{}
	X.SetString("9b6b422147204489291d40321b154e5f3a07d341e8fa4db42a0eccb51c5cf768", 16)
	Y := &big.Int{}
	Y.SetString("308741f739674f61424fe8245ce2d9bf4914832deabddc2ba22de0f2c397a3ed", 16)

	ContractDeployerKey = &ecdsa.PrivateKey{
		D: D,
		PublicKey: ecdsa.PublicKey{
			Curve: crypto.S256(),
			X:     X,
			Y:     Y,
		},
	}
}
