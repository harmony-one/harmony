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

func GetAddressFromPrivateKey(priKey kyber.Scalar) [20]byte {
	return GetAddressFromPublicKey(GetPublicKeyFromScalar(priKey))
}

// Temporary helper function for benchmark use
func GetAddressFromInt(value int) [20]byte {
	return GetAddressFromPublicKey(GetPublicKeyFromScalar(GetPrivateKeyFromInt(value)))
}

func GetPrivateKeyFromInt(value int) kyber.Scalar {
	return crypto.Ed25519Curve.Scalar().SetInt64(int64(value))
}

func GetPublicKeyFromPrivateKey(priKey [32]byte) kyber.Point {
	suite := crypto.Ed25519Curve
	scalar := suite.Scalar()
	scalar.UnmarshalBinary(priKey[:])
	return suite.Point().Mul(scalar, nil)
}

// Same as GetPublicKeyFromPrivateKey, but it directly works on kyber.Scalar object.
func GetPublicKeyFromScalar(priKey kyber.Scalar) kyber.Point {
	return crypto.Ed25519Curve.Point().Mul(priKey, nil)
}
