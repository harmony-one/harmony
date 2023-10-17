package stagedsync

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/rand"
	"reflect"
	"sort"
	"sync"

	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	pb "github.com/harmony-one/harmony/api/service/legacysync/downloader/proto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"

	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
)

// Constants for syncing.
const (
	downloadBlocksRetryLimit        = 3 // downloadBlocks service retry limit
	RegistrationNumber              = 3
	SyncingPortDifference           = 3000
	inSyncThreshold                 = 0  // when peerBlockHeight - myBlockHeight <= inSyncThreshold, it's ready to join consensus
	SyncLoopBatchSize        uint32 = 30 // maximum size for one query of block hashes
	LastMileBlocksSize              = 50

	// after cutting off a number of connected peers, the result number of peers
	// shall be between numPeersLowBound and numPeersHighBound
	NumPeersLowBound  = 3
	numPeersHighBound = 5

	// NumPeersReserved is the number reserved peers which will be replaced with any broken peer
	NumPeersReserved = 2

	// downloadTaskBatch is the number of tasks per each downloader request
	downloadTaskBatch = 5
)

// SyncPeerConfig is peer config to sync.
type SyncPeerConfig struct {
	peer        p2p.Peer
	ip          string
	port        string
	peerHash    []byte
	client      *downloader.Client
	blockHashes [][]byte       // block hashes before node doing sync
	newBlocks   []*types.Block // blocks after node doing sync
	mux         sync.RWMutex
	failedTimes uint64
}

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// AddFailedTime considers one more peer failure and checks against max allowed failed times
func (peerConfig *SyncPeerConfig) AddFailedTime(maxFailures uint64) (mustStop bool) {
	peerConfig.mux.Lock()
	defer peerConfig.mux.Unlock()
	peerConfig.failedTimes++
	if peerConfig.failedTimes > maxFailures {
		return true
	}
	return false
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
	mtx           sync.RWMutex
	reservedPeers []*SyncPeerConfig
	peers         []*SyncPeerConfig
	selfPeerID    libp2p_peer.ID
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

// SelectRandomPeers limits number of peers to release some server end sources.
func (sc *SyncConfig) SelectRandomPeers(peers []p2p.Peer, randSeed int64) int {
	numPeers := len(peers)
	targetSize := calcNumPeersWithBound(numPeers, NumPeersLowBound, numPeersHighBound)
	// if number of peers is less than required number, keep all in list
	if numPeers <= targetSize {
		utils.Logger().Warn().
			Int("num connected peers", numPeers).
			Msg("[STAGED_SYNC] not enough connected peers to sync, still sync will on going")
		return numPeers
	}
	//shuffle peers list
	r := rand.New(rand.NewSource(randSeed))
	r.Shuffle(numPeers, func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })

	return targetSize
}

// calcNumPeersWithBound calculates the number of connected peers with bound
// peers are expected to limited at half of the size, capped between lowBound and highBound.
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
func (sc *SyncConfig) RemovePeer(peer *SyncPeerConfig, reason string) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	peer.client.Close(reason)
	for i, p := range sc.peers {
		if p == peer {
			sc.peers = append(sc.peers[:i], sc.peers[i+1:]...)
			break
		}
	}
	utils.Logger().Info().
		Str("peerIP", peer.ip).
		Str("peerPortMsg", peer.port).
		Str("reason", reason).
		Msg("[STAGED_SYNC] remove GRPC peer")
}

// ReplacePeerWithReserved tries to replace a peer from reserved peer list
func (sc *SyncConfig) ReplacePeerWithReserved(peer *SyncPeerConfig, reason string) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	peer.client.Close(reason)
	for i, p := range sc.peers {
		if p == peer {
			if len(sc.reservedPeers) > 0 {
				sc.peers = append(sc.peers[:i], sc.peers[i+1:]...)
				sc.peers = append(sc.peers, sc.reservedPeers[0])
				utils.Logger().Info().
					Str("peerIP", peer.ip).
					Str("peerPort", peer.port).
					Str("reservedPeerIP", sc.reservedPeers[0].ip).
					Str("reservedPeerPort", sc.reservedPeers[0].port).
					Str("reason", reason).
					Msg("[STAGED_SYNC] replaced GRPC peer by reserved")
				sc.reservedPeers = sc.reservedPeers[1:]
			} else {
				sc.peers = append(sc.peers[:i], sc.peers[i+1:]...)
				utils.Logger().Info().
					Str("peerIP", peer.ip).
					Str("peerPortMsg", peer.port).
					Str("reason", reason).
					Msg("[STAGED_SYNC] remove GRPC peer without replacement")
			}
			break
		}
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

// getHowManyMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Assumption: all peers are sorted by CompareSyncPeerConfigByBlockHashes first.
// Caller shall ensure mtx is locked for reading.
func getHowManyMaxConsensus(peers []*SyncPeerConfig) (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	if len(peers) == 0 {
		return -1, 0
	} else if len(peers) == 1 {
		return 0, 1
	}
	maxFirstID := len(peers) - 1
	for i := maxFirstID - 1; i >= 0; i-- {
		if CompareSyncPeerConfigByblockHashes(peers[maxFirstID], peers[i]) != 0 {
			break
		}
		maxFirstID = i
	}
	maxCount := len(peers) - maxFirstID
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
	countBeforeCleanUp := len(sc.peers)
	for i := 0; i < len(sc.peers); i++ {
		if CompareSyncPeerConfigByblockHashes(fixedPeer, sc.peers[i]) != 0 {
			// TODO: move it into a util delete func.
			// See tip https://github.com/golang/go/wiki/SliceTricks
			// Close the client and remove the peer out of the
			sc.peers[i].client.Close("close by cleanup function, because blockHashes is not equal to consensus block hashes")
			copy(sc.peers[i:], sc.peers[i+1:])
			sc.peers[len(sc.peers)-1] = nil
			sc.peers = sc.peers[:len(sc.peers)-1]
		}
	}
	if len(sc.peers) < countBeforeCleanUp {
		utils.Logger().Debug().
			Int("removed peers", len(sc.peers)-countBeforeCleanUp).
			Msg("[STAGED_SYNC] cleanUpPeers: a few peers removed")
	}
}

// cleanUpInvalidPeers cleans up all peers whose missed a few required block hash or sent an invalid block hash
// Caller shall ensure mtx is locked for RW.
func (sc *SyncConfig) cleanUpInvalidPeers(ipm map[string]bool) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	countBeforeCleanUp := len(sc.peers)
	for i := 0; i < len(sc.peers); i++ {
		if ipm[string(sc.peers[i].peerHash)] == true {
			sc.peers[i].client.Close("cleanup invalid peers, it may missed a few required block hashes or sent an invalid block hash")
			copy(sc.peers[i:], sc.peers[i+1:])
			sc.peers[len(sc.peers)-1] = nil
			sc.peers = sc.peers[:len(sc.peers)-1]
		}
	}
	if len(sc.peers) < countBeforeCleanUp {
		utils.Logger().Debug().
			Int("removed peers", len(sc.peers)-countBeforeCleanUp).
			Msg("[STAGED_SYNC] cleanUpPeers: a few peers removed")
	}
}

// GetBlockHashesConsensusAndCleanUp selects the most common peer config based on their block hashes to download/sync.
// Note that choosing the most common peer config does not guarantee that the blocks to be downloaded are the correct ones.
// The subsequent node syncing steps of verifying the block header chain will give such confirmation later.
// If later block header verification fails with the sync peer config chosen here, the entire sync loop gets retried with a new peer set.
func (sc *SyncConfig) GetBlockHashesConsensusAndCleanUp(bgMode bool) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	// Sort all peers by the blockHashes.
	sort.Slice(sc.peers, func(i, j int) bool {
		return CompareSyncPeerConfigByblockHashes(sc.peers[i], sc.peers[j]) == -1
	})
	maxFirstID, maxCount := getHowManyMaxConsensus(sc.peers)
	if maxFirstID == -1 {
		return errors.New("invalid peer index -1 for block hashes query")
	}
	utils.Logger().Info().
		Int("maxFirstID", maxFirstID).
		Str("targetPeerIP", sc.peers[maxFirstID].ip).
		Int("maxCount", maxCount).
		Int("hashSize", len(sc.peers[maxFirstID].blockHashes)).
		Msg("[STAGED_SYNC] block consensus hashes")

	if bgMode {
		if maxCount != len(sc.peers) {
			return ErrNodeNotEnoughBlockHashes
		}
	} else {
		sc.cleanUpPeers(maxFirstID)
	}
	return nil
}
