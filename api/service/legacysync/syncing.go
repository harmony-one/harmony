package legacysync

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/consensus"
	consensus2 "github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

// Constants for syncing.
const (
	downloadBlocksRetryLimit        = 10 // downloadBlocks service retry limit
	RegistrationNumber              = 3
	SyncingPortDifference           = 3000
	inSyncThreshold                 = 0   // when peerBlockHeight - myBlockHeight <= inSyncThreshold, it's ready to join consensus
	SyncLoopBatchSize        uint32 = 30  // maximum size for one query of block hashes
	verifyHeaderBatchSize    uint64 = 100 // block chain header verification batch size (not used for now)
	LastMileBlocksSize              = 50

	// after cutting off a number of connected peers, the result number of peers
	// shall be between numPeersLowBound and numPeersHighBound
	NumPeersLowBound  = 3
	numPeersHighBound = 5

	downloadTaskBatch = 5

	//LoopMinTime sync loop must take at least as this value, otherwise it waits for it
	LoopMinTime = 0
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	peer        p2p.Peer
	peerHash    []byte
	client      *downloader.Client
	blockHashes [][]byte       // block hashes before node doing sync
	newBlocks   []*types.Block // blocks after node doing sync
	mux         sync.Mutex
}

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// IsEqual checks the equality between two sync peers
func (peerConfig *SyncPeerConfig) IsEqual(pc2 *SyncPeerConfig) bool {
	return peerConfig.peer.IP == pc2.peer.IP && peerConfig.peer.Port == pc2.peer.Port
}

// SyncBlockTask is the task struct to sync a specific block.
type SyncBlockTask struct {
	index     int
	blockHash []byte
}

type syncBlockTasks []SyncBlockTask

func (tasks syncBlockTasks) blockHashes() [][]byte {
	hashes := make([][]byte, 0, len(tasks))
	for _, task := range tasks {
		hash := make([]byte, len(task.blockHash))
		copy(hash, task.blockHash)
		hashes = append(hashes, task.blockHash)
	}
	return hashes
}

func (tasks syncBlockTasks) blockHashesStr() []string {
	hashes := make([]string, 0, len(tasks))
	for _, task := range tasks {
		hash := hex.EncodeToString(task.blockHash)
		hashes = append(hashes, hash)
	}
	return hashes
}

func (tasks syncBlockTasks) indexes() []int {
	indexes := make([]int, 0, len(tasks))
	for _, task := range tasks {
		indexes = append(indexes, task.index)
	}
	return indexes
}

// SyncConfig contains an array of SyncPeerConfig.
type SyncConfig struct {
	// mtx locks peers, and *SyncPeerConfig pointers in peers.
	// SyncPeerConfig itself is guarded by its own mutex.
	mtx sync.RWMutex

	peers      []*SyncPeerConfig
	shardID    uint32
	selfPeerID libp2p_peer.ID
}

func NewSyncConfig(shardID uint32, selfPeerID libp2p_peer.ID, peers []*SyncPeerConfig) *SyncConfig {
	return &SyncConfig{
		peers:      peers,
		shardID:    shardID,
		selfPeerID: selfPeerID,
	}
}

func (sc *SyncConfig) ShardID() uint32 {
	return sc.shardID
}

// AddPeer adds the given sync peer.
func (sc *SyncConfig) AddPeer(peer *SyncPeerConfig) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	// Ensure no duplicate peers
	for _, p2 := range sc.peers {
		if peer.IsEqual(p2) {
			return
		}
		if peer.peer.PeerID == sc.selfPeerID {
			return
		}
	}
	sc.peers = append(sc.peers, peer)
}

func (sc *SyncConfig) GetPeers() []*SyncPeerConfig {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	out := make([]*SyncPeerConfig, len(sc.peers))
	copy(out, sc.peers)
	return out
}

// ForEachPeer calls the given function with each peer.
// It breaks the iteration iff the function returns true.
func (sc *SyncConfig) ForEachPeer(f func(peer *SyncPeerConfig) (brk bool)) {
	sc.mtx.RLock()
	peers := make([]*SyncPeerConfig, len(sc.peers))
	copy(peers, sc.peers)
	sc.mtx.RUnlock()

	for _, peer := range peers {
		if f(peer) {
			break
		}
	}
}

func (sc *SyncConfig) PeersCount() int {
	if sc == nil {
		return 0
	}
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	return len(sc.peers)
}

// RemovePeer removes a peer from SyncConfig
func (sc *SyncConfig) RemovePeer(peer *SyncPeerConfig, reason string) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	closeReason := fmt.Sprintf("remove peer (reason: %s)", reason)
	peer.client.Close(closeReason)
	for i, p := range sc.peers {
		if p == peer {
			sc.peers = append(sc.peers[:i], sc.peers[i+1:]...)
			break
		}
	}
	utils.Logger().Info().
		Str("peerIP", peer.peer.IP).
		Str("peerPortMsg", peer.peer.Port).
		Str("reason", reason).
		Msg("[SYNC] remove GRPC peer")
}

// CreateStateSync returns the implementation of StateSyncInterface interface.
func CreateStateSync(bc blockChain, ip string, port string, peerHash [20]byte, peerID libp2p_peer.ID, isExplorer bool, role nodeconfig.Role) *StateSync {
	stateSync := &StateSync{}
	stateSync.blockChain = bc
	stateSync.selfip = ip
	stateSync.selfport = port
	stateSync.selfPeerHash = peerHash
	stateSync.commonBlocks = make(map[int]*types.Block)
	stateSync.lastMileBlocks = []*types.Block{}
	stateSync.isExplorer = isExplorer
	stateSync.syncConfig = NewSyncConfig(bc.ShardID(), peerID, nil)

	stateSync.syncStatus = newSyncStatus(role)
	return stateSync
}

// Small subset from Blockchain struct.
type blockChain interface {
	CurrentBlock() *types.Block
	ShardID() uint32
}

// StateSync is the struct that implements StateSyncInterface.
type StateSync struct {
	blockChain         blockChain
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

func (ss *StateSync) IntoEpochSync() *EpochSync {
	return &EpochSync{
		beaconChain:        ss.blockChain,
		selfip:             ss.selfip,
		selfport:           ss.selfport,
		selfPeerHash:       ss.selfPeerHash,
		commonBlocks:       ss.commonBlocks,
		lastMileBlocks:     ss.lastMileBlocks,
		syncConfig:         ss.syncConfig,
		isExplorer:         ss.isExplorer,
		stateSyncTaskQueue: ss.stateSyncTaskQueue,
		syncMux:            sync.Mutex{},
		lastMileMux:        sync.Mutex{},
		syncStatus:         ss.syncStatus.Clone(),
	}
}

func (ss *StateSync) purgeAllBlocksFromCache() {
	ss.lastMileMux.Lock()
	ss.lastMileBlocks = nil
	ss.lastMileMux.Unlock()

	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.commonBlocks = make(map[int]*types.Block)

	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		configPeer.newBlocks = nil
		return
	})
}

func (ss *StateSync) purgeOldBlocksFromCache() {
	ss.syncMux.Lock()
	defer ss.syncMux.Unlock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		return
	})
}

// AddLastMileBlock add the latest a few block into queue for syncing
// only keep the latest blocks with size capped by LastMileBlocksSize
func (ss *StateSync) AddLastMileBlock(block *types.Block) {
	ss.lastMileMux.Lock()
	defer ss.lastMileMux.Unlock()
	if ss.lastMileBlocks != nil {
		if len(ss.lastMileBlocks) >= LastMileBlocksSize {
			ss.lastMileBlocks = ss.lastMileBlocks[1:]
		}
		ss.lastMileBlocks = append(ss.lastMileBlocks, block)
	}
}

// CloseConnections close grpc connections for state sync clients
func (sc *SyncConfig) CloseConnections() {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, pc := range sc.peers {
		pc.client.Close("close all connections")
	}
}

// FindPeerByHash returns the peer with the given hash, or nil if not found.
func (sc *SyncConfig) FindPeerByHash(peerHash []byte) *SyncPeerConfig {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, pc := range sc.peers {
		if bytes.Equal(pc.peerHash, peerHash) {
			return pc
		}
	}
	return nil
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

// CreateTestSyncPeerConfig used for testing.
func CreateTestSyncPeerConfig(client *downloader.Client, blockHashes [][]byte) *SyncPeerConfig {
	return &SyncPeerConfig{
		client:      client,
		blockHashes: blockHashes,
	}
}

// CompareSyncPeerConfigByblockHashes compares two SyncPeerConfig by blockHashes.
func CompareSyncPeerConfigByblockHashes(a *SyncPeerConfig, b *SyncPeerConfig) int {
	if len(a.blockHashes) != len(b.blockHashes) {
		if len(a.blockHashes) < len(b.blockHashes) {
			return -1
		}
		return 1
	}
	for id := range a.blockHashes {
		if !reflect.DeepEqual(a.blockHashes[id], b.blockHashes[id]) {
			return bytes.Compare(a.blockHashes[id], b.blockHashes[id])
		}
	}
	return 0
}

// BlockWithSig the serialization structure for request DownloaderRequest_BLOCKWITHSIG
// The block is encoded as block + commit signature
type BlockWithSig struct {
	Block              *types.Block
	CommitSigAndBitmap []byte
}

// GetBlocks gets blocks by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlocks(hashes [][]byte) ([][]byte, error) {
	response := peerConfig.client.GetBlocksAndSigs(hashes)
	if response == nil {
		return nil, ErrGetBlock
	}
	return response.Payload, nil
}

// CreateSyncConfig creates SyncConfig for StateSync object.
func (ss *StateSync) CreateSyncConfig(peers []p2p.Peer, shardID uint32, selfPeerID libp2p_peer.ID, waitForEachPeerToConnect bool) error {
	var err error
	ss.syncConfig, err = createSyncConfig(ss.syncConfig, peers, shardID, selfPeerID, waitForEachPeerToConnect)
	return err
}

// checkPeersDuplicity checks whether there are duplicates in p2p.Peer
func checkPeersDuplicity(ps []p2p.Peer) error {
	type peerDupID struct {
		ip   string
		port string
	}
	m := make(map[peerDupID]struct{})
	for _, p := range ps {
		dip := peerDupID{p.IP, p.Port}
		if _, ok := m[dip]; ok {
			return fmt.Errorf("duplicate peer [%v:%v]", p.IP, p.Port)
		}
		m[dip] = struct{}{}
	}
	return nil
}

// limitNumPeers limits number of peers to release some server end sources.
func limitNumPeers(ps []p2p.Peer, randSeed int64) (int, []p2p.Peer) {
	targetSize := calcNumPeersWithBound(len(ps), NumPeersLowBound, numPeersHighBound)
	if len(ps) <= targetSize {
		return len(ps), ps
	}

	r := rand.New(rand.NewSource(randSeed))
	r.Shuffle(len(ps), func(i, j int) { ps[i], ps[j] = ps[j], ps[i] })

	return targetSize, ps
}

// Peers are expected to limited at half of the size, capped between lowBound and highBound.
func calcNumPeersWithBound(size int, lowBound, highBound int) int {
	if size < lowBound {
		return size
	}
	expLen := size / 2
	if expLen < lowBound {
		expLen = lowBound
	}
	if expLen > highBound {
		expLen = highBound
	}
	return expLen
}

// GetActivePeerNumber returns the number of active peers
func (ss *StateSync) GetActivePeerNumber() int {
	if ss.syncConfig == nil {
		return 0
	}
	// len() is atomic; no need to hold mutex.
	return len(ss.syncConfig.peers)
}

// getHowManyMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Assumption: all peers are sorted by CompareSyncPeerConfigByBlockHashes first.
// Caller shall ensure mtx is locked for reading.
func (sc *SyncConfig) getHowManyMaxConsensus() (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1
	for i := range sc.peers {
		if curFirstID == -1 || CompareSyncPeerConfigByblockHashes(sc.peers[curFirstID], sc.peers[i]) != 0 {
			curCount = 1
			curFirstID = i
		} else {
			curCount++
		}
		if curCount >= maxCount {
			maxCount = curCount
			maxFirstID = curFirstID
		}
	}
	return maxFirstID, maxCount
}

// InitForTesting used for testing.
func (sc *SyncConfig) InitForTesting(client *downloader.Client, blockHashes [][]byte) {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for i := range sc.peers {
		sc.peers[i].blockHashes = blockHashes
		sc.peers[i].client = client
	}
}

// cleanUpPeers cleans up all peers whose blockHashes are not equal to
// consensus block hashes.  Caller shall ensure mtx is locked for RW.
func (sc *SyncConfig) cleanUpPeers(maxFirstID int) {
	fixedPeer := sc.peers[maxFirstID]

	var removedPeers int
	for i := 0; i < len(sc.peers); i++ {
		if CompareSyncPeerConfigByblockHashes(fixedPeer, sc.peers[i]) != 0 {
			// TODO: move it into a util delete func.
			// See tip https://github.com/golang/go/wiki/SliceTricks
			// Close the client and remove the peer out of the
			sc.peers[i].client.Close("cleanup peers")
			copy(sc.peers[i:], sc.peers[i+1:])
			sc.peers[len(sc.peers)-1] = nil
			sc.peers = sc.peers[:len(sc.peers)-1]
			removedPeers++
		}
	}
	utils.Logger().Info().Int("removed peers", removedPeers).Msg("[SYNC] post cleanUpPeers")
}

// GetBlockHashesConsensusAndCleanUp selects the most common peer config based on their block hashes to download/sync.
// Note that choosing the most common peer config does not guarantee that the blocks to be downloaded are the correct ones.
// The subsequent node syncing steps of verifying the block header chain will give such confirmation later.
// If later block header verification fails with the sync peer config chosen here, the entire sync loop gets retried with a new peer set.
func (sc *SyncConfig) GetBlockHashesConsensusAndCleanUp() error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	// Sort all peers by the blockHashes.
	sort.Slice(sc.peers, func(i, j int) bool {
		return CompareSyncPeerConfigByblockHashes(sc.peers[i], sc.peers[j]) == -1
	})
	maxFirstID, maxCount := sc.getHowManyMaxConsensus()

	if maxFirstID == -1 {
		return errors.New("invalid peer index -1 for block hashes query")
	}
	utils.Logger().Info().
		Int("maxFirstID", maxFirstID).
		Str("targetPeerIP", sc.peers[maxFirstID].peer.IP).
		Int("maxCount", maxCount).
		Int("hashSize", len(sc.peers[maxFirstID].blockHashes)).
		Msg("[SYNC] block consensus hashes")

	sc.cleanUpPeers(maxFirstID)
	return nil
}

// getConsensusHashes gets all hashes needed to download.
func (ss *StateSync) getConsensusHashes(startHash []byte, size uint32) error {
	var wg sync.WaitGroup
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			response := peerConfig.client.GetBlockHashes(startHash, size, ss.selfip, ss.selfport)
			if response == nil {
				utils.Logger().Warn().
					Str("peerIP", peerConfig.peer.IP).
					Str("peerPort", peerConfig.peer.Port).
					Msg("[SYNC] getConsensusHashes Nil Response")
				ss.syncConfig.RemovePeer(peerConfig, fmt.Sprintf("StateSync %d: nil response for GetBlockHashes", ss.blockChain.ShardID()))
				return
			}
			utils.Logger().Info().Uint32("queried blockHash size", size).
				Int("got blockHashSize", len(response.Payload)).
				Str("PeerIP", peerConfig.peer.IP).
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

func (ss *StateSync) generateStateSyncTaskQueue(bc core.BlockChain) {
	ss.stateSyncTaskQueue = queue.New(0)
	ss.syncConfig.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		for id, blockHash := range configPeer.blockHashes {
			if err := ss.stateSyncTaskQueue.Put(SyncBlockTask{index: id, blockHash: blockHash}); err != nil {
				utils.Logger().Warn().
					Err(err).
					Int("taskIndex", id).
					Str("taskBlock", hex.EncodeToString(blockHash)).
					Msg("[SYNC] generateStateSyncTaskQueue: cannot add task")
			}
		}
		brk = true
		return
	})
	utils.Logger().Info().Int64("length", ss.stateSyncTaskQueue.Len()).Msg("[SYNC] generateStateSyncTaskQueue: finished")
}

// downloadBlocks downloads blocks from state sync task queue.
func (ss *StateSync) downloadBlocks(bc core.BlockChain) {
	// Initialize blockchain
	var wg sync.WaitGroup
	count := 0
	taskQueue := downloadTaskQueue{ss.stateSyncTaskQueue}
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !taskQueue.empty() {
				tasks, err := taskQueue.poll(downloadTaskBatch, time.Millisecond)
				if err != nil || len(tasks) == 0 {
					if err == queue.ErrDisposed {
						continue
					}
					utils.Logger().Error().Err(err).Msg("[SYNC] downloadBlocks: ss.stateSyncTaskQueue poll timeout")
					break
				}
				payload, err := peerConfig.GetBlocks(tasks.blockHashes())
				if err != nil {
					utils.Logger().Warn().Err(err).
						Str("peerID", peerConfig.peer.IP).
						Str("port", peerConfig.peer.Port).
						Msg("[SYNC] downloadBlocks: GetBlocks failed")
					ss.syncConfig.RemovePeer(peerConfig, fmt.Sprintf("StateSync %d: error returned for GetBlocks: %s", ss.blockChain.ShardID(), err.Error()))
					return
				}
				if len(payload) == 0 {
					count++
					utils.Logger().Error().Int("failNumber", count).
						Msg("[SYNC] downloadBlocks: no more retrievable blocks")
					if count > downloadBlocksRetryLimit {
						break
					}
					if err := taskQueue.put(tasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", tasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("downloadBlocks: cannot add task")
					}
					continue
				}

				failedTasks := ss.handleBlockSyncResult(payload, tasks)

				if len(failedTasks) != 0 {
					count++
					if count > downloadBlocksRetryLimit {
						break
					}
					if err := taskQueue.put(failedTasks); err != nil {
						utils.Logger().Warn().
							Err(err).
							Interface("taskIndexes", failedTasks.indexes()).
							Interface("taskBlockes", tasks.blockHashesStr()).
							Msg("cannot add task")
					}
					continue
				}
			}
		}()
		return
	})
	wg.Wait()
	utils.Logger().Info().Msg("[SYNC] downloadBlocks: finished")
}

func (ss *StateSync) handleBlockSyncResult(payload [][]byte, tasks syncBlockTasks) syncBlockTasks {
	if len(payload) > len(tasks) {
		utils.Logger().Warn().
			Err(errors.New("unexpected number of block delivered")).
			Int("expect", len(tasks)).
			Int("got", len(payload))
		return tasks
	}

	var failedTasks syncBlockTasks
	if len(payload) < len(tasks) {
		utils.Logger().Warn().
			Err(errors.New("unexpected number of block delivered")).
			Int("expect", len(tasks)).
			Int("got", len(payload))
		failedTasks = append(failedTasks, tasks[len(payload):]...)
	}

	for i, blockBytes := range payload {
		// For forward compatibility at server side, it can be types.block or BlockWithSig
		blockObj, err := RlpDecodeBlockOrBlockWithSig(blockBytes)
		if err != nil {
			utils.Logger().Warn().
				Err(err).
				Int("taskIndex", tasks[i].index).
				Str("taskBlock", hex.EncodeToString(tasks[i].blockHash)).
				Msg("download block")
			failedTasks = append(failedTasks, tasks[i])
			continue
		}
		gotHash := blockObj.Hash()
		if !bytes.Equal(gotHash[:], tasks[i].blockHash) {
			utils.Logger().Warn().
				Err(errors.New("wrong block delivery")).
				Str("expectHash", hex.EncodeToString(tasks[i].blockHash)).
				Str("gotHash", hex.EncodeToString(gotHash[:]))
			failedTasks = append(failedTasks, tasks[i])
			continue
		}
		ss.syncMux.Lock()
		ss.commonBlocks[tasks[i].index] = blockObj
		ss.syncMux.Unlock()
	}
	return failedTasks
}

// RlpDecodeBlockOrBlockWithSig decode payload to types.Block or BlockWithSig.
// Return the block with commitSig if set.
func RlpDecodeBlockOrBlockWithSig(payload []byte) (*types.Block, error) {
	var block *types.Block
	if err := rlp.DecodeBytes(payload, &block); err == nil {
		// received payload as *types.Block
		return block, nil
	}

	var bws BlockWithSig
	if err := rlp.DecodeBytes(payload, &bws); err == nil {
		block := bws.Block
		block.SetCurrentCommitSig(bws.CommitSigAndBitmap)
		return block, nil
	}
	return nil, errors.New("failed to decode to either types.Block or BlockWithSig")
}

// downloadTaskQueue is wrapper around Queue with item to be SyncBlockTask
type downloadTaskQueue struct {
	q *queue.Queue
}

func (queue downloadTaskQueue) poll(num int64, timeOut time.Duration) (syncBlockTasks, error) {
	items, err := queue.q.Poll(num, timeOut)
	if err != nil {
		return nil, err
	}
	tasks := make(syncBlockTasks, 0, len(items))
	for _, item := range items {
		task := item.(SyncBlockTask)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (queue downloadTaskQueue) put(tasks syncBlockTasks) error {
	for _, task := range tasks {
		if err := queue.q.Put(task); err != nil {
			return err
		}
	}
	return nil
}

func (queue downloadTaskQueue) empty() bool {
	return queue.q.Empty()
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

func (ss *StateSync) getMaxConsensusBlockFromParentHash(parentHash common.Hash) *types.Block {
	var (
		candidateBlocks []*types.Block
		candidateLock   sync.Mutex
	)

	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		peerConfig.mux.Lock()
		defer peerConfig.mux.Unlock()

		for _, block := range peerConfig.newBlocks {
			ph := block.ParentHash()
			if bytes.Equal(ph[:], parentHash[:]) {
				candidateLock.Lock()
				candidateBlocks = append(candidateBlocks, block)
				candidateLock.Unlock()
				break
			}
		}
		return
	})
	if len(candidateBlocks) == 0 {
		return nil
	}
	// Sort by blockHashes.
	sort.Slice(candidateBlocks, func(i, j int) bool {
		return CompareBlockByHash(candidateBlocks[i], candidateBlocks[j]) == -1
	})
	maxFirstID, maxCount := GetHowManyMaxConsensus(candidateBlocks)
	hash := candidateBlocks[maxFirstID].Hash()
	utils.Logger().Debug().
		Hex("parentHash", parentHash[:]).
		Hex("hash", hash[:]).
		Int("maxCount", maxCount).
		Msg("[SYNC] Find block with matching parenthash")
	return candidateBlocks[maxFirstID]
}

func (ss *StateSync) getBlockFromOldBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.commonBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

func (ss *StateSync) getCommonBlockIter(parentHash common.Hash) *commonBlockIter {
	return newCommonBlockIter(ss.commonBlocks, parentHash)
}

type commonBlockIter struct {
	parentToChild map[common.Hash]*types.Block
	curParentHash common.Hash
}

func newCommonBlockIter(blocks map[int]*types.Block, startHash common.Hash) *commonBlockIter {
	m := make(map[common.Hash]*types.Block)
	for _, block := range blocks {
		m[block.ParentHash()] = block
	}
	return &commonBlockIter{
		parentToChild: m,
		curParentHash: startHash,
	}
}

func (iter *commonBlockIter) Next() *types.Block {
	curBlock, ok := iter.parentToChild[iter.curParentHash]
	if !ok || curBlock == nil {
		return nil
	}
	iter.curParentHash = curBlock.Hash()
	return curBlock
}

func (iter *commonBlockIter) HasNext() bool {
	_, ok := iter.parentToChild[iter.curParentHash]
	return ok
}

func (ss *StateSync) getBlockFromLastMileBlocksByParentHash(parentHash common.Hash) *types.Block {
	for _, block := range ss.lastMileBlocks {
		ph := block.ParentHash()
		if bytes.Equal(ph[:], parentHash[:]) {
			return block
		}
	}
	return nil
}

// UpdateBlockAndStatus ...
func (ss *StateSync) UpdateBlockAndStatus(block *types.Block, bc core.BlockChain, verifyAllSig bool) error {
	if block.NumberU64() != bc.CurrentBlock().NumberU64()+1 {
		utils.Logger().Debug().Uint64("curBlockNum", bc.CurrentBlock().NumberU64()).Uint64("receivedBlockNum", block.NumberU64()).Msg("[SYNC] Inappropriate block number, ignore!")
		return nil
	}

	haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
	// Verify block signatures
	if block.NumberU64() > 1 {
		// Verify signature every N blocks (which N is verifyHeaderBatchSize and can be adjusted in configs)
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
				"[SYNC] UpdateBlockAndStatus: Error adding new block to blockchain %d %d",
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

// generateNewState will construct most recent state from downloaded blocks
func (ss *StateSync) generateNewState(bc core.BlockChain) error {
	// update blocks created before node start sync
	parentHash := bc.CurrentBlock().Hash()

	var err error

	commonIter := ss.getCommonBlockIter(parentHash)
	for {
		block := commonIter.Next()
		if block == nil {
			break
		}
		// Enforce sig check for the last block in a batch
		enforceSigCheck := !commonIter.HasNext()
		err = ss.UpdateBlockAndStatus(block, bc, enforceSigCheck)
		if err != nil {
			break
		}
	}

	ss.syncMux.Lock()
	ss.commonBlocks = make(map[int]*types.Block)
	ss.syncMux.Unlock()

	// update blocks after node start sync
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.getMaxConsensusBlockFromParentHash(parentHash)
		if block == nil {
			break
		}
		err = ss.UpdateBlockAndStatus(block, bc, true)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}
	// TODO ek – Do we need to hold syncMux now that syncConfig has its own mutex?
	ss.syncMux.Lock()
	ss.syncConfig.ForEachPeer(func(peer *SyncPeerConfig) (brk bool) {
		peer.newBlocks = []*types.Block{}
		return
	})
	ss.syncMux.Unlock()

	// update last mile blocks if any
	parentHash = bc.CurrentBlock().Hash()
	for {
		block := ss.getBlockFromLastMileBlocksByParentHash(parentHash)
		if block == nil {
			break
		}
		err = ss.UpdateBlockAndStatus(block, bc, false)
		if err != nil {
			break
		}
		parentHash = block.Hash()
	}

	return err
}

// ProcessStateSync processes state sync from the blocks received but not yet processed so far
func (ss *StateSync) ProcessStateSync(startHash []byte, size uint32, bc core.BlockChain) error {
	// Gets consensus hashes.
	if err := ss.getConsensusHashes(startHash, size); err != nil {
		return errors.Wrap(err, "getConsensusHashes")
	}
	ss.generateStateSyncTaskQueue(bc)
	// Download blocks.
	if ss.stateSyncTaskQueue.Len() > 0 {
		ss.downloadBlocks(bc)
	}
	return ss.generateNewState(bc)
}

func (peerConfig *SyncPeerConfig) registerToBroadcast(peerHash []byte, ip, port string) error {
	response := peerConfig.client.Register(peerHash, ip, port)
	if response == nil || response.Type == pb.DownloaderResponse_FAIL {
		return ErrRegistrationFail
	} else if response.Type == pb.DownloaderResponse_SUCCESS {
		return nil
	}
	return ErrRegistrationFail
}

func (peerConfig *SyncPeerConfig) String() interface{} {
	return fmt.Sprintf("peer: %s:%s ", peerConfig.peer.IP, peerConfig.peer.Port)
}

// RegisterNodeInfo will register node to peers to accept future new block broadcasting
// return number of successful registration
func (ss *StateSync) RegisterNodeInfo() int {
	registrationNumber := RegistrationNumber
	utils.Logger().Debug().
		Int("registrationNumber", registrationNumber).
		Int("activePeerNumber", len(ss.syncConfig.peers)).
		Msg("[SYNC] node registration to peers")

	count := 0
	ss.syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		logger := utils.Logger().With().Str("peerPort", peerConfig.peer.Port).Str("peerIP", peerConfig.peer.IP).Logger()
		if count >= registrationNumber {
			brk = true
			return
		}
		if peerConfig.peer.IP == ss.selfip && peerConfig.peer.Port == GetSyncingPort(ss.selfport) {
			logger.Debug().
				Str("selfport", ss.selfport).
				Str("selfsyncport", GetSyncingPort(ss.selfport)).
				Msg("[SYNC] skip self")
			return
		}
		err := peerConfig.registerToBroadcast(ss.selfPeerHash[:], ss.selfip, ss.selfport)
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

// IsSameBlockchainHeight checks whether the node is out of sync from other peers
func (ss *StateSync) IsSameBlockchainHeight(bc core.BlockChain) (uint64, bool) {
	otherHeight, err := getMaxPeerHeight(ss.syncConfig)
	if err != nil {
		return 0, false
	}
	currentHeight := bc.CurrentBlock().NumberU64()
	return otherHeight, currentHeight == otherHeight
}

// GetMaxPeerHeight ..
func (ss *StateSync) GetMaxPeerHeight() (uint64, error) {
	return getMaxPeerHeight(ss.syncConfig)
}

// SyncLoop will keep syncing with peers until catches up
func (ss *StateSync) SyncLoop(bc core.BlockChain, isBeacon bool, consensus *consensus.Consensus, loopMinTime time.Duration) {
	utils.Logger().Info().Msgf("legacy sync is executing ...")
	if !isBeacon {
		ss.RegisterNodeInfo()
	}

	for {
		start := time.Now()
		currentHeight := bc.CurrentBlock().NumberU64()
		otherHeight, errMaxHeight := getMaxPeerHeight(ss.syncConfig)
		if errMaxHeight != nil {
			utils.Logger().Error().
				Bool("isBeacon", isBeacon).
				Uint32("ShardID", bc.ShardID()).
				Uint64("currentHeight", currentHeight).
				Int("peers count", ss.syncConfig.PeersCount()).
				Msgf("[SYNC] get max height failed")
			break
		}
		if currentHeight >= otherHeight {
			utils.Logger().Info().
				Msgf("[SYNC] Node is now IN SYNC! (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), otherHeight, currentHeight)
			break
		}
		utils.Logger().Info().
			Msgf("[SYNC] Node is OUT OF SYNC (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
				isBeacon, bc.ShardID(), otherHeight, currentHeight)

		startHash := bc.CurrentBlock().Hash()
		size := uint32(otherHeight - currentHeight)
		if size > SyncLoopBatchSize {
			size = SyncLoopBatchSize
		}
		err := ss.ProcessStateSync(startHash[:], size, bc)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msgf("[SYNC] ProcessStateSync failed (isBeacon: %t, ShardID: %d, otherHeight: %d, currentHeight: %d)",
					isBeacon, bc.ShardID(), otherHeight, currentHeight)
			ss.purgeOldBlocksFromCache()
			break
		}
		ss.purgeOldBlocksFromCache()

		if loopMinTime != 0 {
			waitTime := loopMinTime - time.Since(start)
			c := time.After(waitTime)
			select {
			case <-c:
			}
		}
	}
	if consensus != nil {
		if err := ss.addConsensusLastMile(bc, consensus); err != nil {
			utils.Logger().Error().Err(err).Msg("[SYNC] Add consensus last mile")
		}
		// TODO: move this to explorer handler code.
		if ss.isExplorer {
			consensus.UpdateConsensusInformation()
		}
	}
	utils.Logger().Info().Msgf("legacy sync is executed")
	ss.purgeAllBlocksFromCache()
}

func (ss *StateSync) addConsensusLastMile(bc core.BlockChain, consensus *consensus.Consensus) error {
	curNumber := bc.CurrentBlock().NumberU64()
	err := consensus.GetLastMileBlockIter(curNumber+1, func(blockIter *consensus2.LastMileBlockIter) error {
		for {
			block := blockIter.Next()
			if block == nil {
				break
			}
			if _, err := bc.InsertChain(types.Blocks{block}, true); err != nil {
				return errors.Wrap(err, "failed to InsertChain")
			}
		}
		return nil
	})
	return err
}

// GetSyncingPort returns the syncing port.
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}

const (
	// syncStatusExpiration is the expiration time out of a sync status.
	// If last sync result in memory is before the expiration, the sync status
	// will be updated.
	syncStatusExpiration = 6 * time.Second

	// syncStatusExpirationNonValidator is the expiration of sync cache for non-validators.
	// Compared with non-validator, the sync check is not as strict as validator nodes.
	// TODO: add this field to harmony config
	syncStatusExpirationNonValidator = 12 * time.Second
)

type (
	syncStatus struct {
		lastResult     SyncCheckResult
		lastUpdateTime time.Time
		lock           sync.RWMutex
		expiration     time.Duration
	}

	SyncCheckResult struct {
		IsSynchronized bool
		OtherHeight    uint64
		HeightDiff     uint64
	}
)

func ParseResult(res SyncCheckResult) (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	IsSynchronized = res.IsSynchronized
	OtherHeight = res.OtherHeight
	HeightDiff = res.HeightDiff
	return IsSynchronized, OtherHeight, HeightDiff
}

func newSyncStatus(role nodeconfig.Role) syncStatus {
	expiration := getSyncStatusExpiration(role)
	return syncStatus{
		expiration: expiration,
	}
}

func getSyncStatusExpiration(role nodeconfig.Role) time.Duration {
	switch role {
	case nodeconfig.Validator:
		return syncStatusExpiration
	case nodeconfig.ExplorerNode:
		return syncStatusExpirationNonValidator
	default:
		return syncStatusExpirationNonValidator
	}
}

func (status *syncStatus) Get(fallback func() SyncCheckResult) SyncCheckResult {
	status.lock.RLock()
	if !status.expired() {
		result := status.lastResult
		status.lock.RUnlock()
		return result
	}
	status.lock.RUnlock()

	status.lock.Lock()
	defer status.lock.Unlock()
	if status.expired() {
		result := fallback()
		if result.OtherHeight > 0 && result.OtherHeight < uint64(math.MaxUint64) {
			status.update(result)
		}
	}
	return status.lastResult
}

func (status *syncStatus) Clone() syncStatus {
	return syncStatus{
		lastResult:     status.lastResult,
		lastUpdateTime: status.lastUpdateTime,
		lock:           sync.RWMutex{},
		expiration:     status.expiration,
	}
}

func (ss *StateSync) IsSynchronized() bool {
	result := ss.GetSyncStatus()
	return result.IsSynchronized
}

func (status *syncStatus) expired() bool {
	return time.Since(status.lastUpdateTime) > status.expiration
}

func (status *syncStatus) update(result SyncCheckResult) {
	status.lastUpdateTime = time.Now()
	status.lastResult = result
}

// GetSyncStatus get the last sync status for other modules (E.g. RPC, explorer).
// If the last sync result is not expired, return the sync result immediately.
// If the last result is expired, ask the remote DNS nodes for latest height and return the result.
func (ss *StateSync) GetSyncStatus() SyncCheckResult {
	return ss.syncStatus.Get(func() SyncCheckResult {
		return ss.isSynchronized(false)
	})
}

func (ss *StateSync) GetParsedSyncStatus() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	res := ss.syncStatus.Get(func() SyncCheckResult {
		return ss.isSynchronized(false)
	})
	return ParseResult(res)
}

// GetSyncStatusDoubleChecked return the sync status when enforcing a immediate query on DNS nodes
// with a double check to avoid false alarm.
func (ss *StateSync) GetSyncStatusDoubleChecked() SyncCheckResult {
	result := ss.isSynchronized(true)
	return result
}

func (ss *StateSync) GetParsedSyncStatusDoubleChecked() (IsSynchronized bool, OtherHeight uint64, HeightDiff uint64) {
	result := ss.isSynchronized(true)
	return ParseResult(result)
}

// isSynchronized query the remote DNS node for the latest height to check what is the current
// sync status
func (ss *StateSync) isSynchronized(doubleCheck bool) SyncCheckResult {
	if ss.syncConfig == nil {
		return SyncCheckResult{} // If syncConfig is not instantiated, return not in sync
	}
	lastHeight := ss.blockChain.CurrentBlock().NumberU64()
	otherHeight1, errMaxHeight1 := getMaxPeerHeight(ss.syncConfig)
	if errMaxHeight1 != nil {
		return SyncCheckResult{
			IsSynchronized: false,
			OtherHeight:    0,
			HeightDiff:     0,
		}
	}
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
			IsSynchronized: !wasOutOfSync,
			OtherHeight:    otherHeight1,
			HeightDiff:     heightDiff,
		}
	}
	// double check the sync status after 1 second to confirm (avoid false alarm)
	time.Sleep(1 * time.Second)

	otherHeight2, errMaxHeight2 := getMaxPeerHeight(ss.syncConfig)
	if errMaxHeight2 != nil {
		otherHeight2 = otherHeight1
	}
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
		IsSynchronized: !(wasOutOfSync && isOutOfSync && lastHeight == currentHeight),
		OtherHeight:    otherHeight2,
		HeightDiff:     heightDiff,
	}
}
