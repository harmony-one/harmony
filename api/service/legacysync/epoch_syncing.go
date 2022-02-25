package legacysync

import (
	"fmt"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/engine"
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
	blockChain         *core.BlockChain
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
func (ss *EpochSync) isInSync(doubleCheck bool) SyncCheckResult {
	if ss.syncConfig == nil {
		return SyncCheckResult{} // If syncConfig is not instantiated, return not in sync
	}
	otherHeight1 := ss.getMaxPeerHeight(false)
	lastHeight := ss.blockChain.CurrentBlock().NumberU64()
	wasOutOfSync := lastHeight+inSyncThreshold < otherHeight1

	if !doubleCheck {
		heightDiff := otherHeight1 - lastHeight
		if otherHeight1 < lastHeight {
			heightDiff = 0 //
		}
		utils.Logger().Info().
			Uint64("OtherHeight", otherHeight1).
			Uint64("lastHeight", lastHeight).
			Msg("[SYNC] Checking sync status")
		return SyncCheckResult{
			IsInSync:    !wasOutOfSync,
			OtherHeight: otherHeight1,
			HeightDiff:  heightDiff,
		}
	}
	// double check the sync status after 1 second to confirm (avoid false alarm)
	time.Sleep(1 * time.Second)

	otherHeight2 := ss.getMaxPeerHeight(false)
	currentHeight := ss.blockChain.CurrentBlock().NumberU64()

	isOutOfSync := currentHeight+inSyncThreshold < otherHeight2
	utils.Logger().Info().
		Uint64("OtherHeight1", otherHeight1).
		Uint64("OtherHeight2", otherHeight2).
		Uint64("lastHeight", lastHeight).
		Uint64("currentHeight", currentHeight).
		Msg("[SYNC] Checking sync status")
	// Only confirm out of sync when the node has lower height and didn't move in heights for 2 consecutive checks
	heightDiff := otherHeight2 - lastHeight
	if otherHeight2 < lastHeight {
		heightDiff = 0 // overflow
	}
	return SyncCheckResult{
		IsInSync:    !(wasOutOfSync && isOutOfSync && lastHeight == currentHeight),
		OtherHeight: otherHeight2,
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
	for {
		<-time.After(5 * time.Second)
		ss.syncLoop(bc, worker, isBeacon, consensus)
	}
}

func (ss *EpochSync) syncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool, consensus *consensus.Consensus) {
	for {
		maxHeight := ss.getMaxPeerHeight(isBeacon)
		block := bc.CurrentBlock()
		height := block.NumberU64()
		fmt.Println("EpochSync Syncloop ", height, " ", maxHeight)
		if height >= maxHeight {
			utils.Logger().Info().
				Msgf("[SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), maxHeight, height)
			break
		}
		utils.Logger().Info().
			Msgf("[SYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
				isBeacon, bc.ShardID(), maxHeight, height)

		var heights []uint64
		curEpoch := block.Epoch()
		for i := 0; i < int(SyncLoopBatchSize); i++ {
			epochLastBlock := shard.Schedule.EpochLastBlock(curEpoch.Uint64())
			if epochLastBlock > maxHeight {
				break
			}
			heights = append(heights, epochLastBlock)
			curEpoch = curEpoch.Add(curEpoch, common.Big1)
		}

		err := ss.ProcessStateSync(heights, bc, worker)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msgf("[SYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), maxHeight, height)
			break
		}
	}
}

// getMaxPeerHeight gets the maximum blockchain heights from peers
func (ss *EpochSync) getMaxPeerHeight(isBeacon bool) uint64 {
	maxHeight := uint64(math.MaxUint64)
	var (
		wg   sync.WaitGroup
		lock sync.Mutex
	)

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//debug
			// utils.Logger().Debug().Bool("isBeacon", isBeacon).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]getMaxPeerHeight")
			response, err := peerConfig.client.GetBlockChainHeight()
			if err != nil {
				utils.Logger().Warn().Err(err).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]GetBlockChainHeight failed")
				ss.syncConfig.RemovePeer(peerConfig)
				return
			}
			utils.Logger().Info().Str("peerIP", peerConfig.ip).Uint64("blockHeight", response.BlockHeight).
				Msg("[SYNC] getMaxPeerHeight")

			lock.Lock()
			if response != nil {
				if maxHeight == uint64(math.MaxUint64) || maxHeight < response.BlockHeight {
					maxHeight = response.BlockHeight
				}
			}
			lock.Unlock()
		}()
		return
	})
	wg.Wait()
	return maxHeight
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func (ss *EpochSync) ProcessStateSync(heights []uint64, bc *core.BlockChain, worker *worker.Worker) error {

	var payload [][]byte

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		resp := peerConfig.GetClient().GetBlocksByHeights(heights, ss.selfip, ss.selfport)
		if resp == nil {
			return false
		}
		if len(resp.Payload) == 0 {
			return false
		}
		payload = resp.Payload
		return true
	})

	if len(payload) == 0 {
		return errors.New("empty payload")
	}

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
	// TODO(k.potapov) validate blocks

	for _, block := range decoded {
		_, err := bc.StoreShardStateBytes(block.Header().Epoch(), block.Header().ShardState())
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
	}

	return nil
}

// getConsensusHashes gets all hashes needed to download.
func (ss *EpochSync) getConsensusHashes(startHash []byte, size uint32) error {
	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			response := peerConfig.client.GetBlockHashes(startHash, size, ss.selfip, ss.selfport)
			if response == nil {
				utils.Logger().Warn().
					Str("peerIP", peerConfig.ip).
					Str("peerPort", peerConfig.port).
					Msg("[SYNC] getConsensusHashes Nil Response")
				ss.syncConfig.RemovePeer(peerConfig)
				return
			}
			utils.Logger().Info().Uint32("queried blockHash size", size).
				Int("got blockHashSize", len(response.Payload)).
				Str("PeerIP", peerConfig.ip).
				Msg("[SYNC] GetBlockHashes")
			if len(response.Payload) > int(size+1) {
				utils.Logger().Warn().
					Uint32("requestSize", size).
					Int("respondSize", len(response.Payload)).
					Msg("[SYNC] getConsensusHashes: receive more blockHashes than requested!")
				peerConfig.blockHashes = response.Payload[:size+1]
			} else {
				peerConfig.blockHashes = response.Payload
			}
		}()
		return
	})
	wg.Wait()
	if err := ss.syncConfig.GetBlockHashesConsensusAndCleanUp(); err != nil {
		return err
	}
	utils.Logger().Info().Msg("[SYNC] Finished getting consensus block hashes")
	return nil
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *EpochSync) CreateSyncConfig(peers []p2p.Peer, isBeacon bool) error {
	// sanity check to ensure no duplicate peers
	if err := checkPeersDuplicity(peers); err != nil {
		return err
	}
	// limit the number of dns peers to connect
	randSeed := time.Now().UnixNano()
	peers = limitNumPeers(peers, randSeed)

	utils.Logger().Debug().
		Int("len", len(peers)).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] CreateSyncConfig: len of peers")

	if len(peers) == 0 {
		return errors.New("[SYNC] no peers to connect to")
	}
	if ss.syncConfig != nil {
		ss.syncConfig.CloseConnections()
	}
	ss.syncConfig = &SyncConfig{}

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer p2p.Peer) {
			defer wg.Done()
			client := downloader.ClientSetup(peer.IP, peer.Port)
			if client == nil {
				return
			}
			peerConfig := &SyncPeerConfig{
				ip:     peer.IP,
				port:   peer.Port,
				client: client,
			}
			ss.syncConfig.AddPeer(peerConfig)
		}(peer)
	}
	wg.Wait()
	utils.Logger().Info().
		Int("len", len(ss.syncConfig.peers)).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] Finished making connection to peers")

	return nil
}

// UpdateBlockAndStatus ...
func (ss *EpochSync) UpdateBlockAndStatus(block *types.Block, bc *core.BlockChain, verifyAllSig bool) error {
	// disabled
	return nil
	if block.NumberU64() != bc.CurrentBlock().NumberU64()+1 {
		utils.Logger().Debug().Uint64("curBlockNum", bc.CurrentBlock().NumberU64()).Uint64("receivedBlockNum", block.NumberU64()).Msg("[SYNC] Inappropriate block number, ignore!")
		return nil
	}

	haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
	// Verify block signatures
	if block.NumberU64() > 1 {
		// Verify signature every 100 blocks
		verifySeal := block.NumberU64()%verifyHeaderBatchSize == 0 || verifyAllSig
		verifyCurrentSig := verifyAllSig && haveCurrentSig
		if verifyCurrentSig {
			sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
			if err != nil {
				return errors.Wrap(err, "parse commitSigAndBitmap")
			}

			startTime := time.Now()
			if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap); err != nil {
				return errors.Wrapf(err, "verify header signature %v", block.Hash().String())
			}
			utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("[Sync] VerifyHeaderSignature")
		}
		err := bc.Engine().VerifyHeader(bc, block.Header(), verifySeal)
		if err == engine.ErrUnknownAncestor {
			return err
		} else if err != nil {
			utils.Logger().Error().Err(err).Msgf("[SYNC] UpdateBlockAndStatus: failed verifying signatures for new block %d", block.NumberU64())

			if !verifyAllSig {
				utils.Logger().Info().Interface("block", bc.CurrentBlock()).Msg("[SYNC] UpdateBlockAndStatus: Rolling back last 99 blocks!")
				for i := uint64(0); i < verifyHeaderBatchSize-1; i++ {
					if rbErr := bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()}); rbErr != nil {
						utils.Logger().Err(rbErr).Msg("[SYNC] UpdateBlockAndStatus: failed to rollback")
						return err
					}
				}
			}
			return err
		}
	}

	_, err := bc.InsertChain([]*types.Block{block}, false /* verifyHeaders */)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf(
				"[SYNC] UpdateBlockAndStatus: Error adding newck to blockchain %d %d",
				block.NumberU64(),
				block.ShardID(),
			)
		return err
	}
	utils.Logger().Info().
		Uint64("blockHeight", block.NumberU64()).
		Uint64("blockEpoch", block.Epoch().Uint64()).
		Str("blockHex", block.Hash().Hex()).
		Uint32("ShardID", block.ShardID()).
		Msg("[SYNC] UpdateBlockAndStatus: New Block Added to Blockchain")

	for i, tx := range block.StakingTransactions() {
		utils.Logger().Info().
			Msgf(
				"StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage(),
			)
	}
	return nil
}
