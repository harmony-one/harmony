package multibls

import (
	"strings"

	"github.com/harmony-one/harmony/shard"

	"github.com/harmony-one/bls/ffi/go/bls"
)

// PrivateKey stores the bls secret keys that belongs to the node
type PrivateKey struct {
	PrivateKey []*bls.SecretKey
}

// PublicKey stores the bls public keys that belongs to the node
type PublicKey struct {
	PublicKey      []*bls.PublicKey
	PublicKeyBytes []shard.BLSPublicKey
}

// SerializeToHexStr wrapper
func (multiKey *PublicKey) SerializeToHexStr() string {
	if multiKey == nil {
		return ""
	}
	var builder strings.Builder
	for _, pubKey := range multiKey.PublicKey {
		builder.WriteString(pubKey.SerializeToHexStr() + ";")
	}
	return builder.String()
}

// Contains wrapper
func (multiKey PublicKey) Contains(pubKey *bls.PublicKey) bool {
	for _, key := range multiKey.PublicKey {
		if key.IsEqual(pubKey) {
			return true
		}
	}
	return false
}

// GetPublicKey wrapper
func (multiKey PrivateKey) GetPublicKey() *PublicKey {
	pubKeys := make([]*bls.PublicKey, len(multiKey.PrivateKey))
	pubKeysBytes := make([]shard.BLSPublicKey, len(multiKey.PrivateKey))
	for i, key := range multiKey.PrivateKey {
		pubKeys[i] = key.GetPublicKey()
		pubKeysBytes[i].FromLibBLSPublicKey(pubKeys[i])
	}

	return &PublicKey{PublicKey: pubKeys, PublicKeyBytes: pubKeysBytes}
}

// GetPrivateKey creates a multibls PrivateKey using bls.SecretKey
func GetPrivateKey(key *bls.SecretKey) *PrivateKey {
	return &PrivateKey{PrivateKey: []*bls.SecretKey{key}}
}

// GetPublicKey creates a multibls PublicKey using bls.PublicKey
func GetPublicKey(key *bls.PublicKey) *PublicKey {
	return &PublicKey{PublicKey: []*bls.PublicKey{key}}
}

// AppendPubKey appends a PublicKey to multibls PublicKey
func AppendPubKey(multiKey *PublicKey, key *bls.PublicKey) {
	if multiKey != nil {
		multiKey.PublicKey = append(multiKey.PublicKey, key)
	} else {
		multiKey = &PublicKey{PublicKey: []*bls.PublicKey{key}}
	}
}

// AppendPriKey appends a SecretKey to multibls PrivateKey
func AppendPriKey(multiKey *PrivateKey, key *bls.SecretKey) {
	if multiKey != nil {
		multiKey.PrivateKey = append(multiKey.PrivateKey, key)
	} else {
		multiKey = &PrivateKey{PrivateKey: []*bls.SecretKey{key}}
	}
}
