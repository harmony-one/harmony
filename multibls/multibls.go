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

// PublicKeys stores the bls public keys that belongs to the node
type PublicKeys []shard.BLSPublicKeyWrapper

// SerializeToHexStr wrapper
func (multiKey PublicKeys) SerializeToHexStr() string {
	if multiKey == nil {
		return ""
	}
	var builder strings.Builder
	for _, pubKey := range multiKey {
		builder.WriteString(pubKey.Bytes.Hex() + ";")
	}
	return builder.String()
}

// Contains wrapper
func (multiKey PublicKeys) Contains(pubKey *bls.PublicKey) bool {
	for _, key := range multiKey {
		if key.Object.IsEqual(pubKey) {
			return true
		}
	}
	return false
}

// GetPublicKey wrapper
func (multiKey PrivateKey) GetPublicKey() PublicKeys {
	pubKeys := make([]shard.BLSPublicKeyWrapper, len(multiKey.PrivateKey))
	for i, key := range multiKey.PrivateKey {
		wrapper := shard.BLSPublicKeyWrapper{Object: key.GetPublicKey()}
		wrapper.Bytes.FromLibBLSPublicKey(wrapper.Object)
		pubKeys[i] = wrapper
	}

	return pubKeys
}

// GetPrivateKey creates a multibls PrivateKey using bls.SecretKey
func GetPrivateKey(key *bls.SecretKey) *PrivateKey {
	return &PrivateKey{PrivateKey: []*bls.SecretKey{key}}
}

// GetPublicKey creates a multibls PublicKeys using bls.PublicKeys
func GetPublicKey(key *bls.PublicKey) PublicKeys {
	keyBytes := shard.BLSPublicKey{}
	keyBytes.FromLibBLSPublicKey(key)
	return PublicKeys{shard.BLSPublicKeyWrapper{Object: key, Bytes: keyBytes}}
}

// AppendPriKey appends a SecretKey to multibls PrivateKey
func AppendPriKey(multiKey *PrivateKey, key *bls.SecretKey) {
	if multiKey != nil {
		multiKey.PrivateKey = append(multiKey.PrivateKey, key)
	} else {
		multiKey = &PrivateKey{PrivateKey: []*bls.SecretKey{key}}
	}
}
