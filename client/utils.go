package client

import "github.com/simple-rules/harmony-benchmark/crypto/pki"

var AddressToIntPriKeyMap map[[20]byte]int // For convenience, we use int as the secret seed for generating private key

func init() {
	AddressToIntPriKeyMap := make(map[[20]byte]int)
	for i := 0; i < 10000; i++ {
		AddressToIntPriKeyMap[pki.GetAddressFromInt(i)] = i
	}
}
