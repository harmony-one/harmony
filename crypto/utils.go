package crypto

import (
	"crypto/sha256"
	"github.com/dedis/kyber"
)

func Hash(message string) [32]byte {
	return sha256.Sum256([]byte(message))
}

func GetPublicKeyFromPrivateKey(suite Suite, priKey [32]byte) kyber.Point {
	scalar := suite.Scalar()
	scalar.UnmarshalBinary(priKey[:])
	return suite.Point().Mul(scalar, nil)
}

// Same as GetPublicKeyFromPrivateKey, but it directly works on kyber.Scalar object.
func GetPublicKeyFromScalar(suite Suite, priKey kyber.Scalar) kyber.Point {
	return suite.Point().Mul(priKey, nil)
}
