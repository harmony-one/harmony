package syncing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node/worker"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for syncing.
const (
	SleepTimeAfterNonConsensusBlockHashes        = time.Second * 30
	DownloadBlocksRetryLimit                     = 5 // Downloadblocks service retry limit
	RegistrationNumber                           = 3
	SyncingPortDifference                        = 3000
	inSyncThreshold                              = 0    // when peerBlockHeight - myBlockHeight <= inSyncThreshold, it's ready to join consensus
	BatchSize                             uint32 = 1000 //maximum size for one query of block hashes
	SyncLoopFrequency                            = 1    // unit in second
	LastMileBlocksSize                           = 10
)

// SyncBlockTask is the task struct to sync a specific block.
type SyncBlockTask struct {
	index     int
	blockHash []byte
}

// CreateStateSync returns the implementation of StateSyncInterface interface.
func CreateStateSync(ip string, port string, peerHash [20]byte) *StateSync {
	stateSync := &StateSync{}
	stateSync.selfip = ip
	stateSync.selfport = port
	stateSync.selfPeerHash = peerHash
	stateSync.commonBlocks = make(map[int]*types.Block)
	stateSync.lastMileBlocks = []*types.Block{}
	return stateSync
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	selfip             string
	selfport           string
	selfPeerHash       [20]byte // hash of ip and address combination
	commonBlocks       map[int]*types.Block
	lastMileBlocks     []*types.Block // last mile blocks to catch up with the consensus
	syncConfig         *SyncConfig
	stateSyncTaskQueue *queue.Queue
	syncMux            sync.Mutex
	lastMileMux        sync.Mutex
}

func (ss *StateSync) purgeAllBlocksFromCache() {
	ss.commonBlocks = make(map[int]*types.Block)
	ss.lastMileBlocks = nil
	ss.syncConfig.PurgeAllBlocks()
}

func (ss *StateSync) purgeOldBlocksFromCache() {
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncConfig.PurgeOldBlocks()
}

// AddLastMileBlock add the lastest a few block into queue for syncing
// only keep the latest blocks with size capped by LastMileBlocksSize
func (ss *StateSync) AddLastMileBlock(block *types.Block) {
	ss.lastMileMux.Lock()
	defer ss.lastMileMux.Unlock()
	if len(ss.lastMileBlocks) >= LastMileBlocksSize {
		ss.lastMileBlocks = ss.lastMileBlocks[1:]
	}
	ss.lastMileBlocks = append(ss.lastMileBlocks, block)
}

// AddNewBlock will add newly received block into state syncing queue
func (ss *StateSync) AddNewBlock(peerHash []byte, block *types.Block) {
	pc := ss.syncConfig.FindPeerByHash(peerHash)
	if pc == nil {
		// Received a block with no active peer; just ignore.
		return
	}
	// TODO ek – we shouldn't mess with SyncPeerConfig's mutex.
	//  Factor this into a method, like pc.AddNewBlock(block)
	pc.mux.Lock()
	defer pc.mux.Unlock()
	pc.newBlocks = append(pc.newBlocks, block)
	utils.Logger().Debug().
		Int("total", len(pc.newBlocks)).
		Uint64("blockHeight", block.NumberU64()).
		Msg("[SYNC] new block received")
}

// InitSyncConfig creates SyncConfig for StateSync object.
func (ss *StateSync) InitSyncConfig(peers []p2p.Peer, isBeacon bool) error {
	utils.Logger().Debug().
		Int("len", len(peers)).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] InitSyncConfig: len of peers")

	if len(peers) == 0 {
		return ctxerror.New("[SYNC] no peers to connect to")
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
		Int("len", ss.syncConfig.Len()).
		Bool("isBeacon", isBeacon).
		Msg("[SYNC] Finished making connection to peers")

	return nil
}

// GetActivePeerNumber returns the number of active peers
func (ss *StateSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	// len() is atomic; no need to hold mutex.
	return ss.syncConfig.Len()
}

// GetConsensusHashes gets all hashes needed to download.
func (ss *StateSync) GetConsensusHashes(startHash []byte, size uint32) bool {
	count := 0
	for {
		var wg sync.WaitGroup

		ss.syncConfig.Lock()
		defer ss.syncConfig.Unlock()
		ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
			wg.Add(1)
			go func() {
				defer wg.Done()

				response := peerConfig.client.GetBlockHashes(startHash, size, ss.selfip, ss.selfport)
				if response == nil {
					utils.Logger().Warn().
						Str("peerIP", peerConfig.ip).
						Str("peerPort", peerConfig.port).
						Msg("[SYNC] GetConsensusHashes Nil Response")
					return
				}
				if len(response.Payload) > int(size+1) {
					utils.Logger().Warn().
						Uint32("requestSize", size).
						Int("respondSize", len(response.Payload)).
						Msg("[SYNC] GetConsensusHashes: receive more blockHahses than request!")
					peerConfig.blockHashes = response.Payload[:size+1]
				} else {
					peerConfig.blockHashes = response.Payload
				}
			}()
			return
		})

		wg.Wait()

		if ss.syncConfig.GetBlockHashesConsensusAndCleanUp() {
			break
		}

		if count > DownloadBlocksRetryLimit {
			utils.Logger().Info().Msg("[SYNC] GetConsensusHashes: reached retry limit")
			return false
		}

		count++
		time.Sleep(SleepTimeAfterNonConsensusBlockHashes)
	}
	utils.Logger().Info().Msg("[SYNC] Finished getting consensus block hashes")
	return true
}

func (ss *StateSync) generateStateSyncTaskQueue(bc *core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)

	ss.syncConfig.RLock()
	defer ss.syncConfig.RUnlock()
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if err := ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				utils.Logger().Warn().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("cannot add task")
			}
		}
		brk = true
		return
	})

	utils.Logger().Info().Int64("length", ss.stateSyncTaskQueue.Len()).Msg("[SYNC] Finished generateStateSyncTaskQueue")
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc *core.BlockChain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	failNumber := 0

	ss.syncConfig.RLock()
	defer ss.syncConfig.RUnlock()
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)

		go func(stateSyncTaskQueue *queue.Queue, bc *core.BlockChain) {
			defer wg.Done()

			for !stateSyncTaskQueue.Empty() {
				task, err := ss.stateSyncTaskQueue.Poll(1, time.Millisecond)
				if len(task) == 0 {
					break
				}
				if err == queue.ErrTimeout {
					utils.Logger().Error().Err(err).Msg("[SYNC] ss.stateSyncTaskQueue poll timeout")
					break
				}

				var blockObj types.Block
				syncTask := task[0].(SyncBlockTask)

				//id := syncTask.index
				payload, err := peerConfig.GetBlocks([][]byte{syncTask.blockHash})
				if err != nil || len(payload) == 0 {
					goto Exit
				}

				// currently only send one block a time
				err = rlp.DecodeBytes(payload[0], &blockObj)
				if err != nil {
					goto Exit
				}

				ss.syncMux.Lock()
				ss.commonBlocks[syncTask.index] = &blockObj
				ss.syncMux.Unlock()

			Exit:
				if err != nil {
					failNumber++
					utils.Logger().Error().Err(err).Int("failNumber", failNumber).Msg("[SYNC] downloadBlocks failed")
					if failNumber > DownloadBlocksRetryLimit {
						break
					}

					if err := ss.stateSyncTaskQueue.Put(syncTask); err != nil {
						utils.Logger().Warn().
							Err(err).
							Int("taskIndex", syncTask.index).
							Str("taskBlock", hex.EncodeToString(syncTask.blockHash)).
							Msg("cannot add task")
					}
				}

			}
		}(ss.stateSyncTaskQueue, bc)

		return
	})
	wg.Wait()
	utils.Logger().Info().Msg("[SYNC] Finished downloadBlocks")
}

// CompareBlockByHash compares two block by hash, it will be used in sort the blocks
func CompareBlockByHash(a *types.Block, b *types.Block) int {
	ha := a.Hash()
	hb := b.Hash()
	return bytes.Compare(ha[:], hb[:])
}

// GetHowManyMaxConsensus will get the most common blocks and the first such blockID
func GetHowManyMaxConsensus(blocks []*types.Block) (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range blocks {
		if curFirstID == -1 || CompareBlockByHash(blocks[curFirstID], blocks[i]) != 0 {
			curCount = 1
			curFirstID = i
		} else {
			curCount++
		}
		if curCount > maxCount {
			maxCount = curCount
			maxFirstID = curFirstID
		}
	}
	return maxFirstID, maxCount
}

func (ss *StateSync) getBlockFromOldBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Compare(ph[:], parentHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (ss *StateSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.lastMileBlocks {
		ph := block.ParentHash()
		if bytes.Compare(ph[:], parentHash[:]) == 0 {
			return block
		}
	}
	return nil
}

func (ss *StateSync) updateBlockAndStatus(block *types.Block, bc *core.BlockChain, worker *worker.Worker) bool {
	utils.Logger().Info().Str("blockHex", bc.CurrentBlock().Hash().Hex()).Msg("[SYNC] Current Block")

	// Verify block signatures
	// TODO chao: only when block is verified against last commit sigs, we can update the block and status
	if block.NumberU64() > 1 {
		// Verify signature every 100 blocks
		verifySig := block.NumberU64()%100 == 0
		err := bc.Engine().VerifyHeader(bc, block.Header(), verifySig)
		if err != nil {
			utils.Logger().Error().Err(err).Msgf("[SYNC] failed verifying signatures for new block %d", block.NumberU64())
			utils.Logger().Debug().Interface("block", bc.CurrentBlock()).Msg("[SYNC] Rolling back last 99 blocks!")
			for i := 0; i < 99; i++ {
				bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()})
			}
			return false
		}
	}

	_, err := bc.InsertChain([]*types.Block{block})
	if err != nil {
		utils.Logger().Error().Err(err).Msgf("[SYNC] Error adding new block to blockchain %d %d", block.NumberU64(), block.ShardID())

		utils.Logger().Debug().Interface("block", bc.CurrentBlock()).Msg("[SYNC] Rolling back current block!")
		bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()})
		return false
	}
	utils.Logger().Info().
		Uint64("blockHeight", bc.CurrentBlock().NumberU64()).
		Str("blockHex", bc.CurrentBlock().Hash().Hex()).
		Msg("[SYNC] new block added to blockchain")
	return true
}

// generateNewState will construct most recent state from downloaded blocks
func (ss *StateSync) generateNewState(bc *core.BlockChain, worker *worker.Worker) {
	// update blocks created before node start sync
	parentHash := bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromOldBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
			break
		}
		parentHash = block.Hash()
	}
	ss.syncMux.Lock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncMux.Unlock()

	// update blocks after node start sync
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.syncConfig.GetMaxConsensusBlockFromParentHash(parentHash)
		if block == nil {
			break
		}
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
			break
		}
		parentHash = block.Hash()
	}
	// TODO ek – Do we need to hold syncMux now that syncConfig has its own mutex?
	ss.syncConfig.Lock()
	defer ss.syncConfig.Unlock()
	ss.syncConfig.ForEachPeer(func(peer *SyncPeerConfig) (brk bool) {
		peer.newBlocks = []*types.Block{}
		return
	})

	// update last mile blocks if any
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromLastMileBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		ok := ss.updateBlockAndStatus(block, bc, worker)
		if !ok {
			break
		}
		parentHash = block.Hash()
	}
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
// TODO: return error
func (ss *StateSync) ProcessStateSync(startHash []byte, size uint32, bc *core.BlockChain, worker *worker.Worker) {
	// Gets consensus hashes.
	if !ss.GetConsensusHashes(startHash, size) {
		utils.Logger().Debug().Msg("[SYNC] ProcessStateSync unable to reach consensus on ss.GetConsensusHashes")
		return
	}
	ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	if ss.stateSyncTaskQueue.Len() > 0 {
		ss.downloadBlocks(bc)
	}
	ss.generateNewState(bc, worker)
}

func registerToBroadcast(peerConfig *SyncPeerConfig, peerHash []byte, ip, port string) error {
	response := peerConfig.client.Register(peerHash, ip, port)
	if response == nil || response.Type == pb.DownloaderResponse_FAIL {
		return ErrRegistrationFail
	} else if response.Type == pb.DownloaderResponse_SUCCESS {
		return nil
	}
	return ErrRegistrationFail
}

// RegisterNodeInfo will register node to peers to accept future new block broadcasting
// return number of successful registration
func (ss *StateSync) RegisterNodeInfo() int {
	registrationNumber := RegistrationNumber
	utils.Logger().Debug().
		Int("registrationNumber", registrationNumber).
		Int("activePeerNumber", ss.syncConfig.Len()).
		Msg("[SYNC] node registration to peers")

	ss.syncConfig.RLock()
	defer ss.syncConfig.RUnlock()

	count := 0

	ss.syncConfig.Lock()
	defer ss.syncConfig.Unlock()
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		logger := utils.Logger().With().Str("peerPort", peerConfig.port).Str("peerIP", peerConfig.ip).Logger()
		if count >= registrationNumber {
			brk = true
			return
		}
		if peerConfig.ip == ss.selfip && peerConfig.port == GetSyncingPort(ss.selfport) {
			logger.Debug().
				Str("selfport", ss.selfport).
				Str("selfsyncport", GetSyncingPort(ss.selfport)).
				Msg("[SYNC] skip self")
			return
		}
		err := registerToBroadcast(peerConfig, ss.selfPeerHash[:], ss.selfip, ss.selfport)
		if err != nil {
			logger.Debug().
				Hex("selfPeerHash", ss.selfPeerHash[:]).
				Msg("[SYNC] register failed to peer")
			return
		}

		logger.Debug().Msg("[SYNC] register success")
		count++
		return
	})
	return count
}

// getMaxPeerHeight gets the maximum blockchain heights from peers
func (ss *StateSync) getMaxPeerHeight(isBeacon bool) uint64 {
	maxHeight := uint64(0)
	var wg sync.WaitGroup

	ss.syncConfig.RLock()
	defer ss.syncConfig.RUnlock()
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//debug
			// utils.Logger().Debug().Bool("isBeacon", isBeacon).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]getMaxPeerHeight")
			response, err := peerConfig.client.GetBlockChainHeight()
			if err != nil {
				utils.Logger().Warn().Err(err).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]GetBlockChainHeight failed")
				return
			}
			ss.syncMux.Lock()
			if response != nil && maxHeight < response.BlockHeight {
				maxHeight = response.BlockHeight
			}
			ss.syncMux.Unlock()
		}()
		return
	})

	wg.Wait()

	return maxHeight
}

// IsSameBlockchainHeight checks whether the node is out of sync from other peers
func (ss *StateSync) IsSameBlockchainHeight(bc *core.BlockChain) (uint64, bool) {
	otherHeight := ss.getMaxPeerHeight(false)
	currentHeight := bc.CurrentBlock().NumberU64()
	return otherHeight, currentHeight == otherHeight
}

// IsOutOfSync checks whether the node is out of sync from other peers
func (ss *StateSync) IsOutOfSync(bc *core.BlockChain) bool {
	otherHeight := ss.getMaxPeerHeight(false)
	currentHeight := bc.CurrentBlock().NumberU64()
	utils.Logger().Debug().
		Uint64("OtherHeight", otherHeight).
		Uint64("MyHeight", currentHeight).
		Bool("IsOutOfSync", currentHeight+inSyncThreshold < otherHeight).
		Msg("[SYNC] Checking sync status")
	return currentHeight+inSyncThreshold < otherHeight
}

// SyncLoop will keep syncing with peers until catches up
func (ss *StateSync) SyncLoop(bc *core.BlockChain, worker *worker.Worker, isBeacon bool) {
	if !isBeacon {
		ss.RegisterNodeInfo()
	}
	// remove SyncLoopFrequency
	ticker := time.NewTicker(SyncLoopFrequency * time.Second)
Loop:
	for {
		select {
		case <-ticker.C:
			otherHeight := ss.getMaxPeerHeight(isBeacon)
			currentHeight := bc.CurrentBlock().NumberU64()
			if currentHeight >= otherHeight {
				utils.Logger().Info().Msgf("[SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)", isBeacon, bc.ShardID(), otherHeight, currentHeight)
				break Loop
			} else {
				utils.Logger().Debug().Msgf("[SYNC] Node is Not in Sync (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)", isBeacon, bc.ShardID(), otherHeight, currentHeight)
			}
			startHash := bc.CurrentBlock().Hash()
			size := uint32(otherHeight - currentHeight)
			if size > BatchSize {
				size = BatchSize
			}
			ss.ProcessStateSync(startHash[:], size, bc, worker)
			ss.purgeOldBlocksFromCache()
		}
		time.Sleep(SyncLoopFrequency * time.Second)
	}
	ss.purgeAllBlocksFromCache()
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}
