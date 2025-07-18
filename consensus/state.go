package consensus

import (
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/types"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
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

	quorumAchievedBlock *types.SafeMap[quorum.Phase, uint64]
}

func NewState(mode Mode, shardID uint32) State {
	state := State{
		mode:                uint32(mode),
		ShardID:             shardID,
		phase:               atomic.Value{},
		quorumAchievedBlock: types.NewSafeMap[quorum.Phase, uint64](),
	}
	state.phase.Store(FBFTAnnounce)
	return state
}

// Mode return the current node mode
func (pm *State) Mode() Mode {
	return Mode(atomic.LoadUint32(&pm.mode))
}

// SetMode set the node mode as required
func (pm *State) SetMode(s Mode) {
	atomic.StoreUint32(&pm.mode, uint32(s))
}

func (pm *State) getBlockNum() uint64 {
	return atomic.LoadUint64(&pm.blockNum)
}

// GetBlockNum returns the block number
func (pm *State) GetBlockNum() uint64 {
	return pm.getBlockNum()
}

// setBlockNum sets the FBFT blockNum in consensus object, called at node bootstrap
func (pm *State) setBlockNum(blockNum uint64) {
	atomic.StoreUint64(&pm.blockNum, blockNum)
}

// SetBlockNum sets the FBFT blockNum in consensus object, called at node bootstrap
func (pm *State) SetBlockNum(blockNum uint64) {
	pm.setBlockNum(blockNum)
}

func (pm *State) getLeaderPubKey() *bls_cosi.PublicKeyWrapper {
	return (*bls_cosi.PublicKeyWrapper)(atomic.LoadPointer(&pm.leaderPubKey))
}

func (pm *State) setLeaderPubKey(pub *bls_cosi.PublicKeyWrapper) {
	atomic.StorePointer(&pm.leaderPubKey, unsafe.Pointer(pub))
}

// GetCurBlockViewID return the current view id
func (pm *State) GetCurBlockViewID() uint64 {
	return atomic.LoadUint64(&pm.blockViewID)
}

// SetCurBlockViewID sets the current view id
func (pm *State) SetCurBlockViewID(viewID uint64) uint64 {
	atomic.StoreUint64(&pm.blockViewID, viewID)
	return viewID
}

// GetViewChangingID return the current view changing id
// It is meaningful during view change mode
func (pm *State) GetViewChangingID() uint64 {
	return atomic.LoadUint64(&pm.viewChangingID)
}

// SetViewChangingID set the current view changing id
// It is meaningful during view change mode
func (pm *State) SetViewChangingID(id uint64) {
	atomic.StoreUint64(&pm.viewChangingID, id)
}

// GetLastQuorumAchievedBlock retrieves the block number of the last block
// that achieved quorum for the specified phase.
// If no quorum has been achieved for the given phase, it returns 0.
func (pm *State) GetLastQuorumAchievedBlock(p quorum.Phase) uint64 {
	lqab, exists := pm.quorumAchievedBlock.Get(p)
	if !exists {
		return 0
	}
	return lqab
}

// SetLastQuorumAchievedBlock updates the block number of the last block
// that achieved quorum for the specified phase.
func (pm *State) SetLastQuorumAchievedBlock(p quorum.Phase, blockNum uint64) {
	pm.quorumAchievedBlock.Set(p, blockNum)
}

// GetViewChangeDuraion return the duration of the current view change
// It increase in the power of difference betweeen view changing ID and current view ID
func (pm *State) GetViewChangeDuraion() time.Duration {
	diff := int64(pm.GetViewChangingID() - pm.GetCurBlockViewID())
	return time.Duration(diff * diff * int64(viewChangeDuration))
}

// fallbackNextViewID return the next view ID and duration when there is an exception
// to calculate the time-based viewId
func (pm *State) fallbackNextViewID() (uint64, time.Duration) {
	diff := int64(pm.GetViewChangingID() + 1 - pm.GetCurBlockViewID())
	if diff <= 0 {
		diff = int64(1)
	}
	pm.getLogger().Error().
		Int64("diff", diff).
		Msg("[fallbackNextViewID] use legacy viewID algorithm")
	return pm.GetViewChangingID() + 1, time.Duration(diff * diff * int64(viewChangeDuration))
}

// getNextViewID return the next view ID based on the timestamp
// The next view ID is calculated based on the difference of validator's timestamp
// and the block's timestamp. So that it can be deterministic to return the next view ID
// only based on the blockchain block and the validator's current timestamp.
// The next view ID is the single factor used to determine
// the next leader, so it is mod the number of nodes per shard.
// It returns the next viewID and duration of the view change
// The view change duration is a fixed duration now to avoid stuck into offline nodes during
// the view change.
// viewID is only used as the fallback mechansim to determine the nextViewID
func (pm *State) getNextViewID(curHeader *block.Header) (uint64, time.Duration) {
	if curHeader == nil {
		return pm.fallbackNextViewID()
	}
	blockTimestamp := curHeader.Time().Int64()
	stuckBlockViewID := curHeader.ViewID().Uint64() + 1
	curTimestamp := time.Now().Unix()

	// timestamp messed up in current validator node
	if curTimestamp <= blockTimestamp {
		pm.getLogger().Error().
			Int64("curTimestamp", curTimestamp).
			Int64("blockTimestamp", blockTimestamp).
			Msg("[getNextViewID] timestamp of block too high")
		return pm.fallbackNextViewID()
	}
	// diff only increases, since view change timeout is shorter than
	// view change slot now, we want to make sure diff is always greater than 0
	diff := uint64((curTimestamp-blockTimestamp)/viewChangeSlot + 1)
	nextViewID := diff + stuckBlockViewID

	pm.getLogger().Info().
		Int64("curTimestamp", curTimestamp).
		Int64("blockTimestamp", blockTimestamp).
		Uint64("nextViewID", nextViewID).
		Uint64("stuckBlockViewID", stuckBlockViewID).
		Msg("[getNextViewID]")

	// duration is always the fixed view change duration for synchronous view change
	return nextViewID, viewChangeDuration
}

// getNextLeaderKey uniquely determine who is the leader for given viewID
// It reads the current leader's pubkey based on the blockchain data and returns
// the next leader based on the gap of the viewID of the view change and the last
// know view id of the block.
func (pm *State) getNextLeaderKey(blockchain engine.ChainReader, decider quorum.Decider, viewID uint64, committee *shard.Committee) *bls.PublicKeyWrapper {
	gap := 1

	cur := pm.GetCurBlockViewID()
	if viewID > cur {
		gap = int(viewID - cur)
	}
	var lastLeaderPubKey *bls.PublicKeyWrapper
	var err error
	epoch := big.NewInt(0)
	if blockchain == nil {
		pm.getLogger().Error().Msg("[getNextLeaderKey] Blockchain is nil. Use consensus.LeaderPubKey")
		lastLeaderPubKey = pm.getLeaderPubKey()
	} else {
		curHeader := blockchain.CurrentHeader()
		if curHeader == nil {
			pm.getLogger().Error().Msg("[getNextLeaderKey] Failed to get current header from blockchain")
			lastLeaderPubKey = pm.getLeaderPubKey()
		} else {
			stuckBlockViewID := curHeader.ViewID().Uint64() + 1
			gap = int(viewID - stuckBlockViewID)
			// this is the truth of the leader based on blockchain blocks
			lastLeaderPubKey, err = chain.GetLeaderPubKeyFromCoinbase(blockchain, curHeader)
			if err != nil || lastLeaderPubKey == nil {
				pm.getLogger().Error().Err(err).
					Msg("[getNextLeaderKey] Unable to get leaderPubKey from coinbase. Set it to consensus.LeaderPubKey")
				lastLeaderPubKey = pm.getLeaderPubKey()
			}
			epoch = curHeader.Epoch()
			// viewchange happened at the first block of new epoch
			// use the LeaderPubKey as the base of the next leader
			// as we shouldn't use lastLeader from coinbase as the base.
			// The LeaderPubKey should be updated to the node of index 0 of the committee
			// so, when validator joined the view change process later in the epoch block
			// it can still sync with other validators.
			if curHeader.IsLastBlockInEpoch() {
				pm.getLogger().Info().Msg("[getNextLeaderKey] view change in the first block of new epoch")
				lastLeaderPubKey = decider.FirstParticipant(shard.Schedule.InstanceForEpoch(epoch))
			}
		}
	}
	pm.getLogger().Info().
		Str("lastLeaderPubKey", lastLeaderPubKey.Bytes.Hex()).
		Str("leaderPubKey", pm.getLeaderPubKey().Bytes.Hex()).
		Int("gap", gap).
		Uint64("newViewID", viewID).
		Uint64("myCurBlockViewID", pm.getCurBlockViewID()).
		Msg("[getNextLeaderKey] got leaderPubKey from coinbase")
	// wasFound, next := consensus.Decider.NthNext(lastLeaderPubKey, gap)
	// FIXME: rotate leader on harmony nodes only before fully externalization
	var wasFound bool
	var next *bls.PublicKeyWrapper
	if blockchain != nil && blockchain.Config().IsLeaderRotationInternalValidators(epoch) {
		if blockchain.Config().IsLeaderRotationV2Epoch(epoch) {
			wasFound, next = decider.NthNextValidatorV2(
				committee.Slots,
				lastLeaderPubKey,
				gap)
		} else if blockchain.Config().IsLeaderRotationExternalValidatorsAllowed(epoch) {
			wasFound, next = decider.NthNextValidator(
				committee.Slots,
				lastLeaderPubKey,
				gap)
		} else {
			wasFound, next = decider.NthNextHmy(
				shard.Schedule.InstanceForEpoch(epoch),
				lastLeaderPubKey,
				gap)
		}
	} else {
		wasFound, next = decider.NthNextHmy(
			shard.Schedule.InstanceForEpoch(epoch),
			lastLeaderPubKey,
			gap)
	}
	if !wasFound {
		pm.getLogger().Warn().
			Str("key", pm.getLeaderPubKey().Bytes.Hex()).
			Msg("[getNextLeaderKey] currentLeaderKey not found")
	}
	pm.getLogger().Info().
		Str("nextLeader", next.Bytes.Hex()).
		Msg("[getNextLeaderKey] next Leader")
	return next
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
