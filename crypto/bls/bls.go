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

// WrapperFromPrivateKey makes a PrivateKeyWrapper from bls secret key
func WrapperFromPrivateKey(pri *bls.SecretKey) PrivateKeyWrapper {
	pub := pri.GetPublicKey()
	pubBytes := FromLibBLSPublicKeyUnsafe(pub)
	return PrivateKeyWrapper{
		Pri: pri,
		Pub: &PublicKeyWrapper{
			Bytes:  *pubBytes,
			Object: pub,
		},
	}
}

// WrapperPublicKeyFromString makes a PublicKeyWrapper from public key hex string
func WrapperPublicKeyFromString(pubkey string) (*PublicKeyWrapper, error) {
	pub := &bls.PublicKey{}
	if err := pub.DeserializeHexStr(pubkey); err != nil {
		return nil, err
	}
	pubBytes := FromLibBLSPublicKeyUnsafe(pub)
	return &PublicKeyWrapper{
		Bytes:  *pubBytes,
		Object: pub,
	}, nil
}

// SerializedPublicKey defines the serialized bls public key
type SerializedPublicKey [PublicKeySizeInBytes]byte

// SerializedSignature defines the bls signature
type SerializedSignature [BLSSignatureSizeInBytes]byte

// Bytes returns the byte array of bls signature
func (pk SerializedPublicKey) Bytes() []byte {
	return pk[:]
}

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

// SeparateSigAndMask parse the commig signature data into signature and bitmap.
func SeparateSigAndMask(commitSigs []byte) ([]byte, []byte, error) {
	if len(commitSigs) < BLSSignatureSizeInBytes {
		return nil, nil, errors.Errorf("no mask data found in commit sigs: %x", commitSigs)
	}
	//#### Read payload data from committed msg
	aggSig := make([]byte, BLSSignatureSizeInBytes)
	bitmap := make([]byte, len(commitSigs)-BLSSignatureSizeInBytes)
	offset := 0
	copy(aggSig[:], commitSigs[offset:offset+BLSSignatureSizeInBytes])
	offset += BLSSignatureSizeInBytes
	copy(bitmap[:], commitSigs[offset:])
	//#### END Read payload data from committed msg
	return aggSig, bitmap, nil
}
