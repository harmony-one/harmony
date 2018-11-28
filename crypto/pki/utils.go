package pki

import (
	"crypto/sha256"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/log"
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

func GetAddressFromPrivateKeyBytes(priKey [32]byte) [20]byte {
	return GetAddressFromPublicKey(GetPublicKeyFromScalar(crypto.Ed25519Curve.Scalar().SetBytes(priKey[:])))
}

// Temporary helper function for benchmark use
func GetAddressFromInt(value int) [20]byte {
	return GetAddressFromPublicKey(GetPublicKeyFromScalar(GetPrivateKeyScalarFromInt(value)))
}

func GetPrivateKeyScalarFromInt(value int) kyber.Scalar {
	return crypto.Ed25519Curve.Scalar().SetInt64(int64(value))
}

func GetPrivateKeyFromInt(value int) [32]byte {
	priKey, err := crypto.Ed25519Curve.Scalar().SetInt64(int64(value)).MarshalBinary()
	priKeyBytes := [32]byte{}
	if err == nil {
		copy(priKeyBytes[:], priKey[:])
	}
	return priKeyBytes
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

// Converts public key point to bytes
func GetBytesFromPublicKey(pubKey kyber.Point) [32]byte {
	bytes, err := pubKey.MarshalBinary()
	result := [32]byte{}
	if err == nil {
		copy(result[:], bytes)
	}
	return result
}
