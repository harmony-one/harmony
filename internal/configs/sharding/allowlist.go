package shardingconfig

import (
	"fmt"

	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

type Allowlist struct {
	MaxLimitPerShard int
	BLSPublicKeys    []bls_cosi.PublicKeyWrapper
}

func BLS(pubkeys []string) []bls_cosi.PublicKeyWrapper {
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

// each time to update the allowlist, it requires a hardfork.
// keep same version of mainnet Instance
var mainnetAllowlisV3_TBD = Allowlist{
	MaxLimitPerShard: 0,
	BLSPublicKeys:    BLS([]string{}),
}

// keep same version of testnet Instance
var testnetAllowlistV3_3 = Allowlist{
	MaxLimitPerShard: 4,
	BLSPublicKeys: BLS([]string{
		"7915b9cbae9d675af510cb252362b80ae6d68a3684bbea203bc30d2f5fda25ffcedfa3cf2a6c1d3051469379920a418d",
		"cf8dad79f5da460462b190a4996f1701589139aa0d8a2202bdd10004ad3b0d9299165278f8002bcb49599444b3607802",
		"a7b563a180629a121a3f78d2864b3d5c5b76d1672a3f9de9349fde9c7a3dad0922bf9a4e68cb38d033d6a9ddef754709",
		"0a62ca435c5e48983b6124768b383d3f0b2d358326604aec692189c7833f4a52dff865c1515c32f315eb8e78eaceec11",
	}),
}

var localnetAllowlist = Allowlist{
	MaxLimitPerShard: 0,
	BLSPublicKeys:    BLS([]string{}),
}

var emptyAllowlist = Allowlist{}
