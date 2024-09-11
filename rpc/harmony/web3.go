package rpc

import (
	"context"

	"github.com/harmony-one/harmony/eth/rpc"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

// PublicWeb3Service offers web3 related RPC methods
type PublicWeb3Service struct{}

// NewPublicWeb3API creates a new web3 API instance.
func NewPublicWeb3API() rpc.API {
	return rpc.API{
		Namespace: web3Namespace,
		Version:   APIVersion,
		Service:   &PublicWeb3Service{},
		Public:    true,
	}
}

// ClientVersion - returns the current client version of the running node
func (s *PublicWeb3Service) ClientVersion(ctx context.Context) interface{} {
	timer := DoMetricRPCRequest(ClientVersion)
	defer DoRPCRequestDuration(ClientVersion, timer)
	return nodeconfig.GetVersion()
}
