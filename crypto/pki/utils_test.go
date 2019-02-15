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
