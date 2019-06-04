package pki

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/harmony-one/bls/ffi/go/bls"
)

func TestGetBLSPrivateKeyFromInt(test *testing.T) {
	t := time.Now().UnixNano()
	privateKey1 := GetBLSPrivateKeyFromInt(int(t))
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(t))
	var privateKey2 bls.SecretKey
	privateKey2.SetLittleEndian(priKey[:])
	if !privateKey1.IsEqual(&privateKey2) {
		test.Error("two public address should be equal")
	}
}
