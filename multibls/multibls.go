package multibls

import (
	"strings"

	"github.com/harmony-one/harmony/shard"

	"github.com/harmony-one/bls/ffi/go/bls"
)

// PrivateKeys stores the bls secret keys that belongs to the node
type PrivateKeys []shard.BLSPrivateKeyWrapper

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

// GetPublicKeys wrapper
func (multiKey PrivateKeys) GetPublicKeys() PublicKeys {
	pubKeys := make([]shard.BLSPublicKeyWrapper, len(multiKey))
	for i, key := range multiKey {
		pubKeys[i] = *key.Pub
	}

	return pubKeys
}

// GetPrivateKeys creates a multibls PrivateKeys using bls.SecretKey
func GetPrivateKeys(key *bls.SecretKey) PrivateKeys {
	pub := key.GetPublicKey()
	pubWrapper := shard.BLSPublicKeyWrapper{Object: pub}
	pubWrapper.Bytes.FromLibBLSPublicKey(pub)
	return PrivateKeys{shard.BLSPrivateKeyWrapper{Pri: key, Pub: &pubWrapper}}
}
