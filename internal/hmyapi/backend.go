package hmyapi

import (
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv1"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv2"
)

// GetAPIs returns all the APIs.
func GetAPIs(hmy *hmy.Harmony) []rpc.API {
	nonceLock := new(apiv1.AddrLocker)
	nonceLockV2 := new(apiv2.AddrLocker)
	return []rpc.API{
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   apiv1.NewPublicHarmonyAPI(hmy),
			Public:    true,
		},
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   apiv1.NewPublicBlockChainAPI(hmy),
			Public:    true,
		},
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   apiv1.NewPublicTransactionPoolAPI(hmy, nonceLock),
			Public:    true,
		},
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   apiv1.NewDebugAPI(hmy),
			Public:    false,
		},
		{
			Namespace: "hmyv2",
			Version:   "1.0",
			Service:   apiv2.NewPublicHarmonyAPI(hmy),
			Public:    true,
		},
		{
			Namespace: "hmyv2",
			Version:   "1.0",
			Service:   apiv2.NewPublicBlockChainAPI(hmy),
			Public:    true,
		},
		{
			Namespace: "hmyv2",
			Version:   "1.0",
			Service:   apiv2.NewPublicTransactionPoolAPI(hmy, nonceLockV2),
			Public:    true,
		},
		{
			Namespace: "hmyv2",
			Version:   "1.0",
			Service:   apiv2.NewDebugAPI(hmy),
			Public:    false,
		},
	}
}
