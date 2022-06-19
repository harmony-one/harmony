package stagedsync

import (
	"bytes"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"sync"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
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
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	ip          string
	port        string
	peerHash    []byte
	client      *downloader.Client
	blockHashes [][]byte       // block hashes before node doing sync
	newBlocks   []*types.Block // blocks after node doing sync
	mux         sync.Mutex
}

// CreateTestSyncPeerConfig used for testing.
func CreateTestSyncPeerConfig(client *downloader.Client, blockHashes [][]byte) *SyncPeerConfig {
	return &SyncPeerConfig{
		client:      client,
		blockHashes: blockHashes,
	}
}

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// IsEqual checks the equality between two sync peers
func (peerConfig *SyncPeerConfig) IsEqual(pc2 *SyncPeerConfig) bool {
	return peerConfig.ip == pc2.ip && peerConfig.port == pc2.port
}

// GetBlocks gets blocks by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlocks(hashes [][]byte) ([][]byte, error) {
	response := peerConfig.client.GetBlocksAndSigs(hashes)
	if response == nil {
		return nil, ErrGetBlock
	}
	return response.Payload, nil
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

	peers []*SyncPeerConfig
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
	}
	sc.peers = append(sc.peers, peer)
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

// RemovePeer removes a peer from SyncConfig
func (sc *SyncConfig) RemovePeer(peer *SyncPeerConfig) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	peer.client.Close()
	for i, p := range sc.peers {
		if p == peer {
			sc.peers = append(sc.peers[:i], sc.peers[i+1:]...)
			break
		}
	}
	utils.Logger().Info().Str("peerIP", peer.ip).Str("peerPortMsg", peer.port).
		Msg("[STAGED_SYNC] remove GRPC peer")
}

// CloseConnections close grpc connections for state sync clients
func (sc *SyncConfig) CloseConnections() {
	sc.mtx.RLock()
	defer sc.mtx.RUnlock()
	for _, pc := range sc.peers {
		pc.client.Close()
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

	utils.Logger().Info().Int("peers", len(sc.peers)).Msg("[STAGED_SYNC] before cleanUpPeers")
	for i := 0; i < len(sc.peers); i++ {
		if CompareSyncPeerConfigByblockHashes(fixedPeer, sc.peers[i]) != 0 {
			// TODO: move it into a util delete func.
			// See tip https://github.com/golang/go/wiki/SliceTricks
			// Close the client and remove the peer out of the
			sc.peers[i].client.Close()
			copy(sc.peers[i:], sc.peers[i+1:])
			sc.peers[len(sc.peers)-1] = nil
			sc.peers = sc.peers[:len(sc.peers)-1]
		}
	}
	utils.Logger().Info().Int("peers", len(sc.peers)).Msg("[STAGED_SYNC] post cleanUpPeers")
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
		Str("targetPeerIP", sc.peers[maxFirstID].ip).
		Int("maxCount", maxCount).
		Int("hashSize", len(sc.peers[maxFirstID].blockHashes)).
		Msg("[STAGED_SYNC] block consensus hashes")

	sc.cleanUpPeers(maxFirstID)
	return nil
}
