package bls

import (
	"bytes"
	"encoding/hex"
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"
)

var (
	emptyBLSPubKey = SerializedPublicKey{}
)

// PublicKeySizeInBytes ..
const (
	PublicKeySizeInBytes    = 48
	BLSSignatureSizeInBytes = 96
)

// PrivateKeyWrapper combines the bls private key and the corresponding public key
type PrivateKeyWrapper struct {
	Pri *bls.SecretKey
	Pub *PublicKeyWrapper
}

// PublicKeyWrapper defines the bls public key in both serialized and
// deserialized form.
type PublicKeyWrapper struct {
	Bytes  SerializedPublicKey
	Object *bls.PublicKey
}

// SerializedPublicKey defines the serialized bls public key
type SerializedPublicKey [PublicKeySizeInBytes]byte

// SerializedSignature defines the bls signature
type SerializedSignature [BLSSignatureSizeInBytes]byte

// Big ..
func (pk SerializedPublicKey) Big() *big.Int {
	return new(big.Int).SetBytes(pk[:])
}

// IsEmpty returns whether the bls public key is empty 0 bytes
func (pk SerializedPublicKey) IsEmpty() bool {
	return bytes.Equal(pk[:], emptyBLSPubKey[:])
}

// Hex returns the hex string of bls public key
func (pk SerializedPublicKey) Hex() string {
	return hex.EncodeToString(pk[:])
}

// MarshalText so that we can use this as JSON printable when used as
// key in a map
func (pk SerializedPublicKey) MarshalText() (text []byte, err error) {
	text = make([]byte, BLSSignatureSizeInBytes)
	hex.Encode(text, pk[:])
	return text, nil
}

// FromLibBLSPublicKeyUnsafe could give back nil, use only in cases when
// have invariant that return value won't be nil
func FromLibBLSPublicKeyUnsafe(key *bls.PublicKey) *SerializedPublicKey {
	result := &SerializedPublicKey{}
	if err := result.FromLibBLSPublicKey(key); err != nil {
		return nil
	}
	return result
}

// FromLibBLSPublicKey replaces the key contents with the given key,
func (pk *SerializedPublicKey) FromLibBLSPublicKey(key *bls.PublicKey) error {
	bytes := key.Serialize()
	if len(bytes) != len(pk) {
		return errors.Errorf(
			"key size (BLS) size mismatch, expected %d have %d", len(pk), len(bytes),
		)
	}
	copy(pk[:], bytes)
	return nil
}

// ToLibBLSPublicKey copies the key contents into the given key.
func (pk *SerializedPublicKey) ToLibBLSPublicKey(key *bls.PublicKey) error {
	return key.Deserialize(pk[:])
}
