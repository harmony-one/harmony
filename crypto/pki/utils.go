package pki

import (
	"crypto/sha256"
	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/log"
)

func GetAddressFromPublicKey(pubKey kyber.Point) [20]byte {
	bytes, err := pubKey.MarshalBinary()
	if err != nil {
		log.Error("Failed to serialize challenge")
	}
	address := [20]byte{}
	hash := sha256.Sum256(bytes)
	copy(address[:], hash[12:])
	return address
}

// Temporary helper function for benchmark use
func GetAddressFromInt(value int) [20]byte {
	return GetAddressFromPublicKey(crypto.GetPublicKeyFromScalar(crypto.Ed25519Curve, getPrivateKeyFromInt(value)))
}

func getPrivateKeyFromInt(value int) kyber.Scalar {
	return crypto.Ed25519Curve.Scalar().SetInt64(int64(value))
}
