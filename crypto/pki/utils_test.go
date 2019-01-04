package pki

import (
	"github.com/harmony-one/harmony/crypto"
	"reflect"
	"testing"
	"time"
)

func TestGetPublicKeyFromPrivateKey(test *testing.T) {
	suite := crypto.Ed25519Curve
	t := time.Now().UnixNano()
	scalar := suite.Scalar().SetInt64(t)
	pubKey := GetPublicKeyFromScalar(scalar)
	bytes := [32]byte{}
	tmp, err := scalar.MarshalBinary()
	copy(bytes[:], tmp)
	if err != nil {
		test.Error("unable to marshal private key to binary")
	}
	pubKey1 := GetPublicKeyFromPrivateKey(bytes)
	if !reflect.DeepEqual(pubKey, pubKey1) {
		test.Error("two public keys should be equal")
	}
}
