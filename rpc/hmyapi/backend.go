package hmyapi

import (
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/rpc"
)

const namespace = "hmy"

// GetAPIs returns all the APIs.
func GetAPIs(b *core.BlockChain) []rpc.API {
	return []rpc.API{
		{
			Namespace: namespace,
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(b),
			Public:    true,
		},
	}
}
