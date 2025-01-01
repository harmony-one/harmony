package consensus

import (
	"sync/atomic"
	"unsafe"

	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/rs/zerolog"
)

// State contains current mode and current viewID
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

	phase FBFTPhase

	ShardID uint32
}

func NewState(mode Mode, shardID uint32) State {
	return State{
		mode:    uint32(mode),
		ShardID: shardID,
	}
}

func (pm *State) getBlockNum() uint64 {
	return atomic.LoadUint64(&pm.blockNum)
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (pm *State) setBlockNum(blockNum uint64) {
	atomic.StoreUint64(&pm.blockNum, blockNum)
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (pm *State) SetBlockNum(blockNum uint64) {
	pm.setBlockNum(blockNum)
}

func (pm *State) GetBlockNum() uint64 {
	return pm.getBlockNum()
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
		Str("phase", pm.phase.String()).
		Str("mode", pm.Mode().String()).
		Logger()
	return &logger
}

// switchPhase will switch FBFTPhase to desired phase.
func (pm *State) switchPhase(subject string, desired FBFTPhase) {
	pm.getLogger().Info().
		Str("from:", pm.phase.String()).
		Str("to:", desired.String()).
		Str("switchPhase:", subject)

	pm.phase = desired
}

// GetCurBlockViewID returns the current view ID of the consensus
func (pm *State) getCurBlockViewID() uint64 {
	return atomic.LoadUint64(&pm.blockViewID)
}
