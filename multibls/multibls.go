package multibls

import (
	"strings"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// SecretKeys stores the bls secret keys that belongs to the node
type SecretKeys []bls.SecretKey

// PublicKeys stores the bls public keys that belongs to the node
type PublicKeys []bls.PublicKey

// SerializeToHexStr wrapper
func (multiKey PublicKeys) SerializeToHexStr() string {
	if multiKey == nil {
		return ""
	}
	var builder strings.Builder
	for _, pubKey := range multiKey {
		builder.WriteString(pubKey.ToHex() + ";")
	}
	return builder.String()
}

// Contains wrapper
func (multiKey PublicKeys) Contains(pubKey bls.PublicKey) bool {
	for _, key := range multiKey {
		if key.Equal(pubKey) {
			return true
		}
	}
	return false
}

// GetPublicKeys wrapper
func (multiKey SecretKeys) GetPublicKeys() PublicKeys {
	pubKeys := make([]bls.PublicKey, len(multiKey))
	for i, key := range multiKey {
		pubKeys[i] = key.PublicKey()
	}
	return pubKeys
}

// Dedup will return a new list of dedupped private keys.
// This func won't modify the original slice.
func (multiKey SecretKeys) Dedup() SecretKeys {
	uniqueKeys := make(map[bls.SerializedPublicKey]struct{})
	deduped := make(SecretKeys, 0, len(multiKey))
	for _, priKey := range multiKey {
		if _, ok := uniqueKeys[priKey.PublicKey().Serialized()]; ok {
			utils.Logger().Warn().Str("PubKey", priKey.PublicKey().ToHex()).Msg("Duplicate private key ignored!")
			continue
		}
		uniqueKeys[priKey.PublicKey().Serialized()] = struct{}{}
		deduped = append(deduped, priKey)
	}
	return deduped
}
