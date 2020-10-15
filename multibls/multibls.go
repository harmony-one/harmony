package multibls

import (
	"strings"

	"github.com/harmony-one/harmony/internal/utils"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
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

// Dedup will return a new list of dedupped private keys.
// This func won't modify the original slice.
func (multiKey PrivateKeys) Dedup() PrivateKeys {
	uniqueKeys := make(map[bls.SerializedPublicKey]struct{})
	deduped := make(PrivateKeys, 0, len(multiKey))
	for _, priKey := range multiKey {
		if _, ok := uniqueKeys[priKey.Pub.Bytes]; ok {
			utils.Logger().Warn().Str("PubKey", priKey.Pub.Bytes.Hex()).Msg("Duplicate private key ignored!")
			continue
		}
		uniqueKeys[priKey.Pub.Bytes] = struct{}{}
		deduped = append(deduped, priKey)
	}
	return deduped
}

// GetPrivateKeys creates a multibls PrivateKeys using bls.SecretKey
func GetPrivateKeys(secretKeys ...*bls_core.SecretKey) PrivateKeys {
	keys := make(PrivateKeys, 0, len(secretKeys))
	for _, secretKey := range secretKeys {
		key := bls.WrapperFromPrivateKey(secretKey)
		keys = append(keys, key)
	}
	return keys
}
