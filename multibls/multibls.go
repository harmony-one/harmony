package multibls

import (
	"strings"

	"github.com/harmony-one/harmony/crypto/bls"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
)

// PrivateKeys stores the bls secret keys that belongs to the node
type PrivateKeys []bls.PrivateKeyWrapper

// PublicKeys stores the bls public keys that belongs to the node
type PublicKeys []bls.PublicKeyWrapper

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
func (multiKey PublicKeys) Contains(pubKey *bls_core.PublicKey) bool {
	for _, key := range multiKey {
		if key.Object.IsEqual(pubKey) {
			return true
		}
	}
	return false
}

// GetPublicKeys wrapper
func (multiKey PrivateKeys) GetPublicKeys() PublicKeys {
	pubKeys := make([]bls.PublicKeyWrapper, len(multiKey))
	for i, key := range multiKey {
		pubKeys[i] = *key.Pub
	}

	return pubKeys
}

// GetPrivateKeys creates a multibls PrivateKeys using bls.SecretKey
func GetPrivateKeys(key *bls_core.SecretKey) PrivateKeys {
	pub := key.GetPublicKey()
	pubWrapper := bls.PublicKeyWrapper{Object: pub}
	pubWrapper.Bytes.FromLibBLSPublicKey(pub)
	return PrivateKeys{bls.PrivateKeyWrapper{Pri: key, Pub: &pubWrapper}}
}
