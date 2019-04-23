package hmyapi

import (
	"github.com/harmony-one/harmony/rpc"
)

const namespace = "hmy"

// GetAPIs returns all the APIs.
func GetAPIs(b *rpc.HmyAPIBackend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: namespace,
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(b),
			Public:    true,
		}, {
			Namespace: namespace,
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(b, nonceLock),
			Public:    true,
		},
	}
}
