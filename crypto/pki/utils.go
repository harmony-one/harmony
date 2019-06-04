package pki

import (
	"encoding/binary"

	"github.com/harmony-one/bls/ffi/go/bls"
)

func init() {
	bls.Init(bls.BLS12_381)
}

// GetBLSPrivateKeyFromInt returns bls private key
func GetBLSPrivateKeyFromInt(value int) *bls.SecretKey {
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(value))
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])
	return &privateKey
}
