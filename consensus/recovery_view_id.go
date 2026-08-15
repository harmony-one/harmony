package consensus

import (
	"errors"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
)

const (
	// EmergencyRecoveryShard0RetainedBlock is the last retained shard-0 block
	// for the August 2026 mainnet recovery.
	EmergencyRecoveryShard0RetainedBlock uint64 = params.EmergencyRecoveryShard0RetainedBlock

	// EmergencyRecoveryShard1RetainedBlock is the last retained shard-1 block
	// for the August 2026 mainnet recovery.
	EmergencyRecoveryShard1RetainedBlock uint64 = params.EmergencyRecoveryShard1RetainedBlock

	// EmergencyRecoveryViewIDFloor is the recovery release's signed activation
	// floor for mainnet shards 0 and 1.
	EmergencyRecoveryViewIDFloor uint64 = params.EmergencyRecoveryViewIDFloor
)

var (
	ErrEmergencyRecoveryViewIDFloorUnset = errors.New("emergency recovery ViewID floor is unset")
	ErrEmergencyRecoveryViewIDBelowFloor = errors.New("ViewID is below emergency recovery floor")
	ErrViewIDExhausted                   = errors.New("ViewID exhausted")
)

// checkedNextViewID returns viewID+1 without permitting uint64 wraparound.
func checkedNextViewID(viewID uint64) (uint64, error) {
	if viewID == math.MaxUint64 {
		return 0, ErrViewIDExhausted
	}
	return viewID + 1, nil
}

// CheckedNextViewID is the startup-safe form of viewID+1.
func CheckedNextViewID(viewID uint64) (uint64, error) {
	return checkedNextViewID(viewID)
}

func checkedAddViewID(a, b uint64) (uint64, error) {
	if math.MaxUint64-a < b {
		return 0, ErrViewIDExhausted
	}
	return a + b, nil
}

func checkedLeaderViewGap(viewID, lastBlockViewID uint64) (int, error) {
	firstStuckView, err := checkedNextViewID(lastBlockViewID)
	if err != nil {
		return 0, err
	}
	if viewID < firstStuckView || viewID-firstStuckView > uint64(^uint(0)>>1) {
		return 0, errors.New("invalid recovery leader ViewID gap")
	}
	return int(viewID - firstStuckView), nil
}

// emergencyRecoveryViewIDFloorFor scopes the one-off rule to recovered mainnet
// shards at and after each shard's retained block. The bool reports whether the
// rule applies even when its required release value is still unset.
func emergencyRecoveryViewIDFloorFor(
	config *params.ChainConfig, shardID uint32, headHeight uint64,
) (floor uint64, applies bool, err error) {
	if config == nil || config.ChainID == nil ||
		config.ChainID.Cmp(params.MainnetChainID) != 0 {
		return 0, false, nil
	}

	var retainedBlock uint64
	switch shardID {
	case shard.BeaconChainShardID:
		retainedBlock = EmergencyRecoveryShard0RetainedBlock
	case 1:
		retainedBlock = EmergencyRecoveryShard1RetainedBlock
	default:
		return 0, false, nil
	}
	if headHeight < retainedBlock {
		return 0, false, nil
	}

	if EmergencyRecoveryViewIDFloor == 0 {
		return 0, true, ErrEmergencyRecoveryViewIDFloorUnset
	}
	if EmergencyRecoveryViewIDFloor == math.MaxUint64 {
		return 0, true, fmt.Errorf("%w: emergency recovery floor is max uint64", ErrViewIDExhausted)
	}
	return EmergencyRecoveryViewIDFloor, true, nil
}

// ConfigureEmergencyRecoveryViewIDFloor must run before networking or any BLS
// signing. It intentionally fails closed for an applicable build whose audited
// floor has not been filled in.
func (consensus *Consensus) ConfigureEmergencyRecoveryViewIDFloor() error {
	blockchain := consensus.Blockchain()
	if blockchain == nil {
		return errors.New("cannot configure emergency recovery ViewID floor without blockchain")
	}
	header := blockchain.CurrentHeader()
	if header == nil {
		return errors.New("cannot configure emergency recovery ViewID floor without current header")
	}
	floor, applies, err := emergencyRecoveryViewIDFloorFor(
		blockchain.Config(), blockchain.ShardID(), header.Number().Uint64(),
	)
	if err != nil {
		return err
	}
	if !applies {
		return nil
	}

	consensus.current.SetViewIDFloor(floor)
	consensus.getLogger().Warn().
		Uint64("viewIDFloor", floor).
		Uint64("headHeight", header.Number().Uint64()).
		Msg("emergency recovery ViewID floor enabled")
	return nil
}

// InitializeEmergencyRecoveryLeader deterministically derives the leader for
// the exact effective recovery view. This prevents different nodes from
// retaining the old head's leader after a large ViewID jump.
func (consensus *Consensus) InitializeEmergencyRecoveryLeader() error {
	floor := consensus.current.GetViewIDFloor()
	if floor == 0 {
		return nil
	}
	viewID := consensus.current.GetCurBlockViewID()
	leader, err := consensus.expectedLeaderForViewID(viewID)
	if err != nil {
		return err
	}
	consensus.setLeaderPubKey(leader)
	consensus.IgnoreViewIDCheck.UnSet()
	consensus.getLogger().Warn().
		Uint64("viewID", viewID).
		Str("leader", leader.Bytes.Hex()).
		Msg("emergency recovery leader initialized")
	return nil
}

func (consensus *Consensus) expectedLeaderForViewID(viewID uint64) (*bls.PublicKeyWrapper, error) {
	if err := consensus.assertEmergencyRecoveryViewID(viewID); err != nil {
		return nil, err
	}
	blockchain := consensus.Blockchain()
	if blockchain == nil || blockchain.CurrentHeader() == nil {
		return nil, errors.New("cannot derive leader without blockchain head")
	}
	shardState, err := blockchain.ReadShardState(blockchain.CurrentHeader().Epoch())
	if err != nil {
		return nil, fmt.Errorf("read shard state for leader selection: %w", err)
	}
	committee, err := shardState.FindCommitteeByID(consensus.ShardID)
	if err != nil {
		return nil, fmt.Errorf("find committee for leader selection: %w", err)
	}
	leader := consensus.current.getNextLeaderKey(blockchain, consensus.decider(), viewID, committee)
	if leader == nil {
		return nil, errors.New("cannot derive leader for ViewID")
	}
	return leader, nil
}

func atomicMaxUint64(target *uint64, candidate uint64) uint64 {
	for {
		current := atomic.LoadUint64(target)
		if candidate <= current {
			return current
		}
		if atomic.CompareAndSwapUint64(target, current, candidate) {
			return candidate
		}
	}
}

// SetViewIDFloor raises (and can never lower) the process-local ViewID floor.
// It immediately raises both mutable ViewIDs so no signing path observes a
// value below the floor after this method returns.
func (pm *State) SetViewIDFloor(floor uint64) {
	atomicMaxUint64(&pm.viewIDFloor, floor)
	atomicMaxUint64(&pm.blockViewID, floor)
	atomicMaxUint64(&pm.viewChangingID, floor)
}

func (pm *State) GetViewIDFloor() uint64 {
	return atomic.LoadUint64(&pm.viewIDFloor)
}

func (pm *State) clampViewID(viewID uint64) uint64 {
	if floor := pm.GetViewIDFloor(); viewID < floor {
		return floor
	}
	return viewID
}

// nextViewID clamps a calculated next view against both the recovery floor and
// current+1. This is the single normalization used by view-change and leader
// selection.
func (pm *State) nextViewID(calculated uint64) (uint64, error) {
	nextCurrent, err := checkedNextViewID(pm.GetCurBlockViewID())
	if err != nil {
		return 0, err
	}
	next := pm.clampViewID(calculated)
	if next < nextCurrent {
		next = nextCurrent
	}
	if next == math.MaxUint64 {
		return 0, ErrViewIDExhausted
	}
	return next, nil
}

func (consensus *Consensus) assertEmergencyRecoveryViewID(viewID uint64) error {
	if floor := consensus.current.GetViewIDFloor(); floor != 0 {
		if viewID < floor {
			return fmt.Errorf("%w: got %d, floor %d", ErrEmergencyRecoveryViewIDBelowFloor, viewID, floor)
		}
		if viewID == math.MaxUint64 {
			return ErrViewIDExhausted
		}
	}
	return nil
}

func (consensus *Consensus) assertEmergencyRecoveryBlockViewID(viewID uint64) error {
	if err := consensus.assertEmergencyRecoveryViewID(viewID); err != nil {
		return fmt.Errorf("refusing to sign block: %w", err)
	}
	return nil
}

func (consensus *Consensus) validateEmergencyRecoveryMessageBlockViewID(block *types.Block, messageViewID uint64) error {
	if block == nil || block.Header() == nil {
		return errors.New("block is missing its header")
	}
	blockViewID := block.Header().ViewID().Uint64()
	if blockViewID != messageViewID {
		return errors.New("block ViewID does not match message ViewID")
	}
	return consensus.assertEmergencyRecoveryBlockViewID(blockViewID)
}

func (consensus *Consensus) validateCurrentConsensusBlockViewID() error {
	if consensus.current.GetViewIDFloor() == 0 {
		return nil
	}
	var block types.Block
	if err := rlp.DecodeBytes(consensus.current.block, &block); err != nil {
		return fmt.Errorf("decode current consensus block: %w", err)
	}
	return consensus.validateEmergencyRecoveryMessageBlockViewID(&block, consensus.getCurBlockViewID())
}

func (consensus *Consensus) verifyEmergencyRecoveryBlock(block *types.Block) error {
	if block == nil || block.Header() == nil {
		return errors.New("block is missing its header")
	}
	if err := consensus.assertEmergencyRecoveryBlockViewID(block.Header().ViewID().Uint64()); err != nil {
		return err
	}
	return consensus.verifyBlock(block)
}
