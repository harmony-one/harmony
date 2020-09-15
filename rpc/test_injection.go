package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
)

// PublicTestInjectionService provides an API to do injection test
// It offers only methods that operate on public data that is freely available to anyone.
type PublicTestInjectionService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicTestInjectionAPI creates a new API for the RPC interface
func NewPublicTestInjectionAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicTestInjectionService{hmy, version},
		Public:    true,
	}
}

// KillNode kills the current node
func (s *PublicTestInjectionService) KillNode(
	ctx context.Context,
	safe bool,
) string {
	return s.hmy.KillNode(safe)
}

// KillLeader kills the current node if it is leader
func (s *PublicTestInjectionService) KillLeader(
	ctx context.Context,
	safe bool,
) string {
	return s.hmy.KillLeader(safe)
}

// KillNodeCond kills the node based on the condition
func (s *PublicTestInjectionService) KillNodeCond(
	ctx context.Context,
	safe bool,
	phase string,
	mode string,
	block uint64,
	timeout uint,
) string {
	return s.hmy.KillNodeCond(safe, phase, mode, block, timeout)
}
