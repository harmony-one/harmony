package pki

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/harmony-one/bls/ffi/go/bls"
)

func init() {
	bls.Init(bls.BLS12_381)
}

// GetAddressFromPublicKey returns address given a public key.
func GetAddressFromPublicKey(pubKey *bls.PublicKey) [20]byte {
	bytes := pubKey.Serialize()
	address := [20]byte{}
	hash := sha256.Sum256(bytes)
	copy(address[:], hash[12:])
	return address
}

// GetAddressFromPrivateKey returns address given a private key.
func GetAddressFromPrivateKey(priKey *bls.SecretKey) [20]byte {
	return GetAddressFromPublicKey(priKey.GetPublicKey())
}

// GetAddressFromPrivateKeyBytes returns address from private key in bytes.
func GetAddressFromPrivateKeyBytes(priKey [32]byte) [20]byte {
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])

	return GetAddressFromPublicKey(privateKey.GetPublicKey())
}

// GetAddressFromInt is the temporary helper function for benchmark use
func GetAddressFromInt(value int) [20]byte {
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(value))
	return GetAddressFromPrivateKeyBytes(priKey)
}

// GetBLSPrivateKeyFromInt returns bls private key
func GetBLSPrivateKeyFromInt(value int) *bls.SecretKey {
	priKey := [32]byte{}
	binary.LittleEndian.PutUint32(priKey[:], uint32(value))
	var privateKey bls.SecretKey
	privateKey.SetLittleEndian(priKey[:])
	return &privateKey
}
