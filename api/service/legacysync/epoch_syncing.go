package legacysync

import (
	"fmt"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"

	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
)

type EpochSync struct {
	beaconChain        blockChain
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
		return ss.isSynchronized(false)
	})
}

// isSynchronized query the remote DNS node for the latest height to check what is the current
// sync status
func (ss *EpochSync) isSynchronized(_ bool) SyncCheckResult {
	if ss.syncConfig == nil {
		return SyncCheckResult{} // If syncConfig is not instantiated, return not in sync
	}
	otherHeight1, errMaxHeight := getMaxPeerHeight(ss.syncConfig)
	if errMaxHeight != nil {
		utils.Logger().Error().
			Uint64("OtherHeight", otherHeight1).
			Int("Peers count", ss.syncConfig.PeersCount()).
			Err(errMaxHeight).
			Msg("[EPOCHSYNC] No peers for get height")
		return SyncCheckResult{}
	}
	curEpoch := ss.beaconChain.CurrentBlock().Epoch().Uint64()
	otherEpoch := shard.Schedule.CalcEpochNumber(otherHeight1).Uint64()
	normalizedOtherEpoch := otherEpoch - 1
	inSync := curEpoch == normalizedOtherEpoch

	epochDiff := normalizedOtherEpoch - curEpoch
	if normalizedOtherEpoch < curEpoch {
		epochDiff = 0
	}

	utils.Logger().Info().
		Uint64("OtherEpoch", otherEpoch).
		Uint64("CurrentEpoch", curEpoch).
		Msg("[EPOCHSYNC] Checking sync status")
	return SyncCheckResult{
		IsSynchronized: inSync,
		OtherHeight:    otherHeight1,
		HeightDiff:     epochDiff,
	}
}

// GetActivePeerNumber returns the number of active peers
func (ss *EpochSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	return ss.syncConfig.PeersCount()
}

// SyncLoop will keep syncing with peers until catches up
func (ss *EpochSync) SyncLoop(bc core.BlockChain, consensus *consensus.Consensus) time.Duration {
	return time.Duration(syncLoop(bc, ss.syncConfig)) * time.Second
}

func syncLoop(bc core.BlockChain, syncConfig *SyncConfig) (timeout int) {
	isBeacon := bc.ShardID() == shard.BeaconChainShardID
	maxHeight, errMaxHeight := getMaxPeerHeight(syncConfig)
	if errMaxHeight != nil {
		utils.Logger().Info().
			Msgf("[EPOCHSYNC] No peers to sync (isBeacon: %t, ShardID: %d, peersCount: %d)",
				isBeacon, bc.ShardID(), syncConfig.PeersCount())
		return 10
	}

	for {
		curEpoch := bc.CurrentBlock().Epoch().Uint64()
		otherEpoch := shard.Schedule.CalcEpochNumber(maxHeight).Uint64()
		if otherEpoch == curEpoch+1 {
			utils.Logger().Info().
				Msgf("[EPOCHSYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d, peersCount: %d)",
					isBeacon, bc.ShardID(), otherEpoch, curEpoch, syncConfig.PeersCount())
			return 60
		}
		if otherEpoch < curEpoch {
			for _, peerCfg := range syncConfig.GetPeers() {
				syncConfig.RemovePeer(peerCfg, fmt.Sprintf("[EPOCHSYNC]: current height is higher than others, remove peers: %s", peerCfg.String()))
			}
			return 2
		}

		utils.Logger().Info().
			Msgf("[EPOCHSYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d, peers count %d)",
				isBeacon, bc.ShardID(), otherEpoch, curEpoch, syncConfig.PeersCount())

		var heights []uint64
		loopEpoch := curEpoch + 1
		for len(heights) < int(SyncLoopBatchSize) {
			if loopEpoch >= otherEpoch {
				break
			}
			heights = append(heights, shard.Schedule.EpochLastBlock(loopEpoch))
			loopEpoch = loopEpoch + 1
		}

		if len(heights) == 0 {
			// We currently on top.
			return 10
		}

		err := ProcessStateSync(syncConfig, heights, bc)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msgf("[EPOCHSYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherEpoch: %d, currentEpoch: %d)",
					isBeacon, bc.ShardID(), otherEpoch, curEpoch)
			return 2
		}
	}
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func ProcessStateSync(syncConfig *SyncConfig, heights []uint64, bc core.BlockChain) error {
	var payload [][]byte
	var peerCfg *SyncPeerConfig

	peers := syncConfig.GetPeers()
	if len(peers) == 0 {
		return errors.New("no peers to sync")
	}

	for index, peerConfig := range peers {
		resp := peerConfig.GetClient().GetBlocksByHeights(heights)
		if resp == nil {
			syncConfig.RemovePeer(peerConfig, fmt.Sprintf("[EPOCHSYNC]: no response from peer: #%d %s, count %d", index, peerConfig.String(), len(peers)))
			continue
		}
		if len(resp.Payload) == 0 {
			syncConfig.RemovePeer(peerConfig, fmt.Sprintf("[EPOCHSYNC]: empty payload response from peer: #%d %s, count %d", index, peerConfig.String(), len(peers)))
			continue
		}
		payload = resp.Payload
		peerCfg = peerConfig
		break
	}
	if len(payload) == 0 {
		return errors.Errorf("empty payload: no blocks were returned by GetBlocksByHeights for all peers, currentPeersCount %d", syncConfig.PeersCount())
	}
	err := processWithPayload(payload, bc)
	if err != nil {
		// Assume that node sent us invalid data.
		syncConfig.RemovePeer(peerCfg, fmt.Sprintf("[EPOCHSYNC]: failed to process with payload from peer: %s", err.Error()))
		utils.Logger().Error().Err(err).
			Msgf("[EPOCHSYNC] Removing peer %s for invalid data", peerCfg.String())
		return err
	}
	return nil
}

func processWithPayload(payload [][]byte, bc core.BlockChain) error {
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

	_, err := bc.InsertChain(decoded, true)
	return err
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *EpochSync) CreateSyncConfig(peers []p2p.Peer, shardID uint32, selfPeerID libp2p_peer.ID, waitForEachPeerToConnect bool) error {
	var err error
	ss.syncConfig, err = createSyncConfig(ss.syncConfig, peers, shardID, selfPeerID, waitForEachPeerToConnect)
	return err
}
