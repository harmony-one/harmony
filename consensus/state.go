package consensus

import (
	"sync/atomic"
	"unsafe"

	"github.com/harmony-one/harmony/consensus/quorum"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/rs/zerolog"
)

// State contains current(inserted block + 1) fields, or in other words, the state of consensus.
type State struct {
	mode uint32

	// blockNum: the next blockNumber that FBFT is going to agree on,
	// should be equal to the blockNumber of next block
	blockNum uint64

	// current view id in normal mode
	// it changes per successful consensus
	blockViewID uint64

	// view changing id is used during view change mode
	// it is the next view id
	viewChangingID uint64

	// the publickey of leader
	leaderPubKey unsafe.Pointer //*bls.PublicKeyWrapper

	// Blockhash - 32 byte
	blockHash [32]byte
	// Block to run consensus on
	block []byte

	// FBFT phase: Announce, Prepare, Commit
	phase atomic.Value // FBFTPhase

	// ShardID of the consensus
	ShardID uint32

	// lastPrepareQuorumBlock / lastCommitQuorumBlock record the block number for which
	// prepare/commit quorum side-effects were already applied. Used to fire those
	// side-effects once per consensus round, including the multi-BLS case where the
	// leader's own keys may already meet quorum before the first external vote.
	// Cleared in resetState so the same blockNum can be retried after view change.
	lastPrepareQuorumBlock uint64
	lastCommitQuorumBlock  uint64
}

func NewState(mode Mode, shardID uint32) State {
	state := State{
		mode:    uint32(mode),
		ShardID: shardID,
		phase:   atomic.Value{},
	}
	state.phase.Store(FBFTAnnounce)
	return state
}

func (pm *State) getBlockNum() uint64 {
	return atomic.LoadUint64(&pm.blockNum)
}

// setBlockNum sets the blockNum in consensus object, called at node bootstrap
func (pm *State) setBlockNum(blockNum uint64) {
	atomic.StoreUint64(&pm.blockNum, blockNum)
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (pm *State) SetBlockNum(blockNum uint64) {
	pm.setBlockNum(blockNum)
}

// GetBlockNum returns the block number
func (pm *State) GetBlockNum() uint64 {
	return pm.getBlockNum()
}

// GetLastQuorumAchievedBlock returns the last block number for which quorum
// side-effects were applied for the given phase, or 0 if none.
func (pm *State) GetLastQuorumAchievedBlock(p quorum.Phase) uint64 {
	switch p {
	case quorum.Prepare:
		return atomic.LoadUint64(&pm.lastPrepareQuorumBlock)
	case quorum.Commit:
		return atomic.LoadUint64(&pm.lastCommitQuorumBlock)
	default:
		return 0
	}
}

// SetLastQuorumAchievedBlock records that quorum side-effects were applied for
// the given phase at blockNum.
func (pm *State) SetLastQuorumAchievedBlock(p quorum.Phase, blockNum uint64) {
	switch p {
	case quorum.Prepare:
		atomic.StoreUint64(&pm.lastPrepareQuorumBlock, blockNum)
	case quorum.Commit:
		atomic.StoreUint64(&pm.lastCommitQuorumBlock, blockNum)
	}
}

// clearLastQuorumAchievedBlocks clears prepare/commit quorum markers so a new
// consensus round (including same blockNum after view change) can fire again.
func (pm *State) clearLastQuorumAchievedBlocks() {
	atomic.StoreUint64(&pm.lastPrepareQuorumBlock, 0)
	atomic.StoreUint64(&pm.lastCommitQuorumBlock, 0)
}

func (pm *State) getLeaderPubKey() *bls_cosi.PublicKeyWrapper {
	return (*bls_cosi.PublicKeyWrapper)(atomic.LoadPointer(&pm.leaderPubKey))
}

func (pm *State) setLeaderPubKey(pub *bls_cosi.PublicKeyWrapper) {
	atomic.StorePointer(&pm.leaderPubKey, unsafe.Pointer(pub))
}

func (pm *State) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Uint32("shardID", pm.ShardID).
		Uint64("myBlock", pm.getBlockNum()).
		Uint64("myViewID", pm.GetCurBlockViewID()).
		Str("phase", pm.phase.Load().(FBFTPhase).String()).
		Str("mode", pm.Mode().String()).
		Logger()
	return &logger
}

// switchPhase will switch FBFTPhase to desired phase.
func (pm *State) switchPhase(subject string, desired FBFTPhase) {
	pm.getLogger().Info().
		Str("from:", pm.phase.Load().(FBFTPhase).String()).
		Str("to:", desired.String()).
		Str("switchPhase:", subject)

	pm.phase.Store(desired)
}

// GetCurBlockViewID returns the current view ID of the consensus
func (pm *State) getCurBlockViewID() uint64 {
	return atomic.LoadUint64(&pm.blockViewID)
}
