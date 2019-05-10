package hmyapi

import (
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
)

// GetAPIs returns all the APIs.
func GetAPIs(b *core.HmyAPIBackend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(b),
			Public:    true,
		}, {
			Namespace: "hmy",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(b, nonceLock),
			Public:    true,
		},
	}
}
