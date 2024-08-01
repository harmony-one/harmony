package rpc

import (
	"context"

	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
)

// PrivateDebugService Internal JSON RPC for debugging purpose
type PrivateDebugService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPrivateDebugAPI creates a new API for the RPC interface
// TODO(dm): expose public via config
func NewPrivateDebugAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PrivateDebugService{hmy, version},
		Public:    false,
	}
}

// ConsensusViewChangingID return the current view changing ID to RPC
func (s *PrivateDebugService) ConsensusViewChangingID(
	ctx context.Context,
) uint64 {
	return s.hmy.NodeAPI.GetConsensusViewChangingID()
}

// ConsensusCurViewID return the current view ID to RPC
func (s *PrivateDebugService) ConsensusCurViewID(
	ctx context.Context,
) uint64 {
	return s.hmy.NodeAPI.GetConsensusCurViewID()
}

// GetConsensusMode return the current consensus mode
func (s *PrivateDebugService) GetConsensusMode(
	ctx context.Context,
) string {
	return s.hmy.NodeAPI.GetConsensusMode()
}

// GetConsensusPhase return the current consensus mode
func (s *PrivateDebugService) GetConsensusPhase(
	ctx context.Context,
) string {
	return s.hmy.NodeAPI.GetConsensusPhase()
}

// GetConfig get harmony config
func (s *PrivateDebugService) GetConfig(
	ctx context.Context,
) (StructuredResponse, error) {
	return NewStructuredResponse(s.hmy.NodeAPI.GetConfig())
}

// GetLastSigningPower get last signed power
func (s *PrivateDebugService) GetLastSigningPower(
	ctx context.Context,
) (float64, error) {
	return s.hmy.NodeAPI.GetLastSigningPower()
}

// GetLastSigningPower2 get last signed power
func (s *PrivateDebugService) GetLastSigningPower2(
	ctx context.Context,
) (float64, error) {
	return s.hmy.NodeAPI.GetLastSigningPower2()
}
