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

// SetNodeToBackupMode sets the node to backup mode. This changes how the node
// takes part in consensus, so it is kept off the public surface and is only
// reachable when the operator has enabled the debug APIs.
func (s *PrivateDebugService) SetNodeToBackupMode(ctx context.Context, isBackup bool) (bool, error) {
	timer := DoMetricRPCRequest(SetNodeToBackupMode)
	defer DoRPCRequestDuration(SetNodeToBackupMode, timer)
	return s.hmy.NodeAPI.SetNodeBackupMode(isBackup), nil
}
