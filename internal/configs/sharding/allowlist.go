package shardingconfig

import (
	"fmt"

	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

type Allowlist struct {
	MaxLimitPerShard int
	BLSPublicKeys    []bls_cosi.PublicKeyWrapper
}

func _BLS(pubkeys []string) []bls_cosi.PublicKeyWrapper {
	blsPubkeys := make([]bls_cosi.PublicKeyWrapper, len(pubkeys))
	for i := range pubkeys {
		if key, err := bls_cosi.WrapperPublicKeyFromString(pubkeys[i]); err != nil {
			panic(fmt.Sprintf("invalid bls key: %d:%s error:%s", i, pubkeys[i], err.Error()))
		} else {
			blsPubkeys[i] = *key
		}
	}
	return blsPubkeys
}

var mainnetAllowlist = Allowlist{
	MaxLimitPerShard: 0,
	BLSPublicKeys:    _BLS([]string{}),
}

var testnetAllowlist = Allowlist{
	MaxLimitPerShard: 4,
	BLSPublicKeys: _BLS([]string{
		"7915b9cbae9d675af510cb252362b80ae6d68a3684bbea203bc30d2f5fda25ffcedfa3cf2a6c1d3051469379920a418d",
	}),
}

var localnetAllowlist = Allowlist{
	MaxLimitPerShard: 0,
	BLSPublicKeys:    _BLS([]string{}),
}

var emptyAllowlist = Allowlist{}
