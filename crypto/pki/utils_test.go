package pki

import (
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto"
)

func TestGetAddressFromPublicKey(test *testing.T) {
	t := time.Now().UnixNano()
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(t))
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])
	addr1 := GetAddressFromPublicKey(privateKey.GetPublicKey())
	addr2 := GetAddressFromPrivateKey(&privateKey)
	if !reflect.DeepEqual(addr1, addr2) {
		test.Error("two public address should be equal")
	}
}

func TestGetPublicKeyFromPrivateKey(test *testing.T) {
	suite := crypto.Ed25519Curve
	t := time.Now().UnixNano()
	scalar := suite.Scalar().SetInt64(t)
	pubKey1 := GetPublicKeyFromScalar(scalar)
	bytes := [32]byte{}
	tmp, err := scalar.MarshalBinary()
	copy(bytes[:], tmp)
	if err != nil {
		test.Error("unable to marshal private key to binary")
	}
	pubKey2 := GetPublicKeyFromPrivateKey(bytes)
	if !reflect.DeepEqual(pubKey1, pubKey2) {
		test.Error("two public keys should be equal")
	}
}

func TestGetPrivateKeyFromInt(test *testing.T) {
	t := int(time.Now().UnixNano())
	priKey1 := GetPrivateKeyFromInt(t)
	priKeyScalar := GetPrivateKeyScalarFromInt(t)
	tmp, err := priKeyScalar.MarshalBinary()
	priKey2 := [32]byte{}
	copy(priKey2[:], tmp)

	if err != nil {
		test.Error("unable to marshal private key to binary")
	}
	if !reflect.DeepEqual(priKey1, priKey2) {
		test.Error("two private keys should be equal")
	}

}
