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
	D.SetString("65737420636f6e7472616374206b657920737472696e672073747265616d2074", 16)
	X := &big.Int{}
	X.SetString("3d4fc035410b035f72da12df457ce17e255cede9fc9e72796ac3868ceb2ed226", 16)
	Y := &big.Int{}
	Y.SetString("808248810e75eb32859e46cdd8e541ed9f323bde6076d2ca6fd75d6cf0ffd089", 16)

	ContractDeployerKey = &ecdsa.PrivateKey{
		D: D,
		PublicKey: ecdsa.PublicKey{
			Curve: crypto.S256(),
			X:     X,
			Y:     Y,
		},
	}
}
