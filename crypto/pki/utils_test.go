package pki

import (
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
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

func TestGetAddressFromPrivateKeyBytes(test *testing.T) {
	t := time.Now().UnixNano()
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(t))
	address1 := GetAddressFromPrivateKeyBytes(priKey)
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])
	address2 := GetAddressFromPublicKey(privateKey.GetPublicKey())
	if !reflect.DeepEqual(address1, address2) {
		test.Error("two public address should be equal")
	}
}

func TestGetAddressFromInt(test *testing.T) {
	t := time.Now().UnixNano()
	address1 := GetAddressFromInt(int(t))
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(t))
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])
	address2 := GetAddressFromPublicKey(privateKey.GetPublicKey())
	if !reflect.DeepEqual(address1, address2) {
		test.Error("two public address should be equal")
	}
}

func TestGetBLSPrivateKeyFromInt(test *testing.T) {
	t := time.Now().UnixNano()
	privateKey1 := GetBLSPrivateKeyFromInt(int(t))
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(t))
	var privateKey2 bls.SecretKey
	privateKey2.SetLittleEndian(priKey[:])
	address1 := GetAddressFromPrivateKey(privateKey1)
	address2 := GetAddressFromPrivateKey(&privateKey2)
	if !reflect.DeepEqual(address1, address2) {
		test.Error("two public address should be equal")
	}
}
