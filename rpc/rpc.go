package rpc

import (
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
)

// Version enum
const (
	V1 Version = iota
	V2
)
const (
	// APIVersion used for DApp's, bumped after RPC refactor (7/2020)
	APIVersion = "1.1"
)

// Version ..
type Version int

// Namespace ..
func (n Version) Namespace() string {
	return [...]string{"hmy", "hmyv2"}[n]
}

// GetAPIs returns all the API methods for the RPC interface
func GetAPIs(hmy *hmy.Harmony) []rpc.API {
	return []rpc.API{
		// Public methods
		NewPublicHarmonyAPI(hmy, V1),
		NewPublicHarmonyAPI(hmy, V2),
		NewPublicBlockChainAPI(hmy, V1),
		NewPublicBlockChainAPI(hmy, V2),
		NewPublicTransactionPoolAPI(hmy, V1),
		NewPublicTransactionPoolAPI(hmy, V2),
		// Private methods
		NewPrivateDebugAPI(hmy, V1),
		NewPrivateDebugAPI(hmy, V2),
	}
}
