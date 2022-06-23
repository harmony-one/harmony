package legacysync

import (
	"math"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

type EpochSync struct {
	beaconChain        *core.BlockChain
	selfip             string
	selfport           string
	selfPeerHash       [20]byte // hash of ip and address combination
	commonBlocks       map[int]*types.Block
	lastMileBlocks     []*types.Block // last mile blocks to catch up with the consensus
	syncConfig         *SyncConfig
	isExplorer         bool
	stateSyncTaskQueue *queue.Queue
	syncMux            sync.Mutex
	lastMileMux        sync.Mutex

	syncStatus syncStatus
}

// GetSyncStatus get the last sync status for other modules (E.g. RPC, explorer).
// If the last sync result is not expired, return the sync result immediately.
// If the last result is expired, ask the remote DNS nodes for latest height and return the result.
func (ss *EpochSync) GetSyncStatus() SyncCheckResult {
	return ss.syncStatus.Get(func() SyncCheckResult {
		return ss.isInSync(false)
	})
}

// isInSync query the remote DNS node for the latest height to check what is the current
// sync status
func (ss *EpochSync) isInSync(_ bool) SyncCheckResult {
	if ss.syncConfig == nil {
		return SyncCheckResult{} // If syncConfig is not instantiated, return not in sync
	}
	otherHeight1 := getMaxPeerHeight(ss.syncConfig)
	if otherHeight1 == math.MaxUint64 {
		utils.Logger().Error().
			Uint64("OtherHeight", otherHeight1).
			Int("Peers count", ss.syncConfig.PeersCount()).
			Msg("[EPOCHSYNC] No peers for get height")
		return SyncCheckResult{}
	}
	curEpoch := ss.beaconChain.CurrentBlock().Epoch().Uint64()
	otherEpoch := shard.Schedule.CalcEpochNumber(otherHeight1).Uint64()
	normalizedOtherEpoch := otherEpoch - 1
	inSync := curEpoch == normalizedOtherEpoch

	heightDiff := normalizedOtherEpoch - curEpoch
	if normalizedOtherEpoch < curEpoch {
		heightDiff = 0
	}

	utils.Logger().Info().
		Uint64("OtherEpoch", otherEpoch).
		Uint64("CurrentEpoch", curEpoch).
		Msg("[EPOCHSYNC] Checking sync status")
	return SyncCheckResult{
		IsInSync:    inSync,
		OtherHeight: otherHeight1,
		HeightDiff:  heightDiff,
	}
}

// GetActivePeerNumber returns the number of active peers
func (ss *EpochSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	// len() is atomic; no need to hold mutex.
	return len(ss.syncConfig.peers)
}

// SyncLoop will keep syncing with peers until catches up
func (ss *EpochSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus) {
	timeout := 2
	for {
		<-time.After(time.Duration(timeout) * time.Second)
		timeout = ss.syncLoop(bc, worker, isBeacon, consensus)
	}
}

func (ss *EpochSync) syncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, _ *consensus.Consensus) (timeout int) {
	maxHeight := getMaxPeerHeight(ss.syncConfig)
	for {
		curEpoch := bc.CurrentBlock().Epoch().Uint64()
		otherEpoch := shard.Schedule.CalcEpochNumber(maxHeight).Uint64()
		if otherEpoch-1 <= curEpoch {
			utils.Logger().Info().
				Msgf("[EPOCHSYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d)",
					isBeacon, bc.ShardID(), otherEpoch, curEpoch)
			return 60
		}

		utils.Logger().Info().
			Msgf("[EPOCHSYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d, peers count %d)",
				isBeacon, bc.ShardID(), otherEpoch, curEpoch, ss.syncConfig.PeersCount())

		var heights []uint64
		loopEpoch := curEpoch + 1
		for len(heights) < int(SyncLoopBatchSize) {
			epochLastBlockHeight := shard.Schedule.EpochLastBlock(loopEpoch)
			if epochLastBlockHeight > maxHeight {
				break
			}
			heights = append(heights, epochLastBlockHeight)
			loopEpoch = loopEpoch + 1
		}

		if len(heights) == 0 {
			// We currently on top.
			return 10
		}

		err := ss.ProcessStateSync(heights, bc, worker)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msgf("[EPOCHSYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d)",
					isBeacon, bc.ShardID(), otherEpoch, curEpoch)
			return 2
		}
	}
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func (ss *EpochSync) ProcessStateSync(heights []uint64, bc *core.BlockChain, worker *worker.Worker) error {

	var payload [][]byte
	var peerCfg *SyncPeerConfig

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		resp := peerConfig.GetClient().GetBlocksByHeights(heights)
		if resp == nil {
			return false
		}
		if len(resp.Payload) == 0 {
			return false
		}
		payload = resp.Payload
		peerCfg = peerConfig
		return true
	})

	if len(payload) == 0 {
		return errors.New("empty payload")
	}
	err := ss.processWithPayload(payload, bc)
	if err != nil {
		// Assume that node sent us invalid data.
		ss.syncConfig.RemovePeer(peerCfg)
		utils.Logger().Error().Err(err).
			Msgf("[EPOCHSYNC] Removing peer %s for invalid data", peerCfg.String())
		return err
	}
	return nil
}

func (ss *EpochSync) processWithPayload(payload [][]byte, bc *core.BlockChain) error {
	decoded := make([]*types.Block, 0, len(payload))
	for idx, blockBytes := range payload {
		block, err := RlpDecodeBlockOrBlockWithSig(blockBytes)
		if err != nil {
			return errors.Wrap(err, "failed decode")
		}

		if !block.IsLastBlockInEpoch() {
			return errors.Errorf("block is not last index %d hash %s", idx, block.Hash().Hex())
		}

		decoded = append(decoded, block)
	}

	for _, block := range decoded {
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return errors.Wrap(err, "parse commitSigAndBitmap")
		}

		// Signature validation.
		err = bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap)
		if err != nil {
			return errors.Wrap(err, "failed signature validation")
		}

		ep := block.Header().Epoch()
		next := ep.Add(ep, common.Big1)
		_, err = bc.StoreShardStateBytes(next, block.Header().ShardState())
		if err != nil {
			return err
		}

		err = bc.WriteBlockWithoutState(block, common.Big1)
		if err != nil {
			return err
		}

		err = bc.WriteHeadBlock(block)
		if err != nil {
			return err
		}
		utils.Logger().Info().
			Msgf("[EPOCHSYNC] Added block %d %s", block.NumberU64(), block.Hash().Hex())
	}

	return nil
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *EpochSync) CreateSyncConfig(peers []p2p.Peer, isBeacon bool) error {
	var err error
	ss.syncConfig, err = createSyncConfig(ss.syncConfig, peers, isBeacon)
	return err
}
