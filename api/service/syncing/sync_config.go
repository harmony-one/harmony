package syncing

import (
	"bytes"
	"container/heap"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// SyncConfig contains an array of SyncPeerConfig with mutex, and provides min-heap push/pop collections api
type SyncConfig struct {
	// mtx locks peers, and *SyncPeerConfig pointers in peers.
	// SyncPeerConfig itself is guarded by its own mutex.
	mtx   sync.RWMutex
	peers []*SyncPeerConfig // min-heap collection of SyncPeerConfigs fetched from peers
}

// heap interface function.
func (h *SyncConfig) Len() int { return len(h.peers) }

// heap interface function.
func (h *SyncConfig) Less(i, j int) bool {
	return CompareSyncPeerConfigByBlockHashes(h.peers[i], h.peers[j]) < 1
}

// heap interface function.
func (h *SyncConfig) Swap(i, j int) { h.peers[i], h.peers[j] = h.peers[j], h.peers[i] }

// Push ... heap interface function.
func (h *SyncConfig) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	(*h).peers = append((*h).peers, x.(*SyncPeerConfig))
}

// CreateSyncConfig creates and initializes the min-heap structure SyncConfig instance
func CreateSyncConfig() *SyncConfig {
	syncConfig := &SyncConfig{}
	heap.Init(syncConfig)
	return syncConfig
}

// Pop ... heap interface function.
func (h *SyncConfig) Pop() interface{} {
	old := (*h).peers
	n := len(old)
	x := old[n-1]
	(*h).peers = old[0 : n-1]
	return x
}

// Get ...
func (h *SyncConfig) Get(i int) *SyncPeerConfig {
	if i < 0 || i >= len((*h).peers) {
		return nil
	}
	return ((*h).peers)[i]
}

// Lock ...
func (h *SyncConfig) Lock() {
	h.mtx.Lock()
}

// Unlock ...
func (h *SyncConfig) Unlock() {
	h.mtx.Unlock()
}

// RLock ...
func (h *SyncConfig) RLock() {
	h.mtx.RLock()
}

// RUnlock ...
func (h *SyncConfig) RUnlock() {
	h.mtx.RUnlock()
}

// AddPeer adds the given sync peer.
func (h *SyncConfig) AddPeer(peer *SyncPeerConfig) {
	h.Lock()
	defer h.Unlock()
	h.Push(peer)
}

// Remove ...
func (h *SyncConfig) Remove(i int) {
	// TODO: move it into a util delete func.
	// See tip https://github.com/golang/go/wiki/SliceTricks
	// Close the client and remove the peer out of the
	copy((*h).peers[i:], (*h).peers[i+1:])
	(*h).peers[len((*h).peers)-1] = nil
	(*h).peers = (*h).peers[:len(((*h).peers))-1]
}

// ForEachPeer calls the given function with each peer.
// It breaks the iteration iff the function returns true.
// thread-safe
func (h *SyncConfig) ForEachPeer(f func(peer *SyncPeerConfig) (brk bool), write bool) {
	if write {
		h.Lock()
		defer h.Unlock()
	} else {
		h.RLock()
		defer h.RUnlock()
	}

	for i := 0; i < h.Len(); i++ {
		if f(((*h).peers)[i]) {
			break
		}
	}
}

// CloseConnections close grpc connections for state sync clients
func (h *SyncConfig) CloseConnections() {
	h.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		peerConfig.client.Close()
		return
	}, true /* write */)
}

// FindPeerByHash returns the peer with the given hash, or nil if not found.
func (h *SyncConfig) FindPeerByHash(peerHash []byte) *SyncPeerConfig {
	var ret *SyncPeerConfig
	h.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		if bytes.Compare(peerConfig.peerHash, peerHash) == 0 {
			ret = peerConfig
		}
		return
	}, false /* write */)
	return ret
}

// InitForTesting used for testing.
func (h *SyncConfig) InitForTesting(client *downloader.Client, blockHashes [][]byte) {
	h.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = blockHashes
		configPeer.client = client
		brk = true
		return
	}, true /* write */)
}

// CleanUpPeers cleans up all peers whose blockHashes are not equal to
// consensus block hashes.  Caller shall ensure mtx is locked for RW.
func (h *SyncConfig) CleanUpPeers(maxFirstID int) {
	h.RLock()
	defer h.RUnlock()

	fixedPeer := h.Get(maxFirstID)
	for i := 0; i < h.Len(); i++ {
		peer := h.Get(i)
		if CompareSyncPeerConfigByBlockHashes(fixedPeer, peer) != 0 {
			peer.client.Close()
			h.Remove(i)
		}
	}
}

// GetBlockHashesConsensusAndCleanUp checks if all consensus hashes are equal.
func (h *SyncConfig) GetBlockHashesConsensusAndCleanUp() bool {
	h.RLock()
	defer h.RUnlock()

	maxFirstID, maxCount := h.GetHowManyMaxConsensus()
	utils.Logger().Info().
		Int("maxFirstID", maxFirstID).
		Int("maxCount", maxCount).
		Msg("[SYNC] block consensus hashes")
	if float64(maxCount) >= core.ShardingSchedule.ConsensusRatio()*float64(len(h.peers)) {
		h.CleanUpPeers(maxFirstID)
		return true
	}
	return false
}

// GetMaxConsensusBlockFromParentHash computes the max consensus blocks's leaf block to use for node state syncing
func (h *SyncConfig) GetMaxConsensusBlockFromParentHash(parentHash common.Hash) *types.Block {
	candidateBlocks := []*types.Block{}

	h.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		for _, block := range peerConfig.newBlocks {
			ph := block.ParentHash()
			if bytes.Compare(ph[:], parentHash[:]) == 0 {
				candidateBlocks = append(candidateBlocks, block)
				break
			}
		}
		return
	}, false /* write */)

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

// GetHowManyMaxConsensus returns max number of consensus nodes and the first ID of consensus group.
// Caller shall ensure mtx is locked for reading.
func (h *SyncConfig) GetHowManyMaxConsensus() (int, int) {
	// As all peers are sorted by their blockHashes, all equal blockHashes should come together and consecutively.
	curCount := 0
	curFirstID := -1
	maxCount := 0
	maxFirstID := -1

	i := 0

	h.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		if curFirstID == -1 || CompareSyncPeerConfigByBlockHashes(h.Get(curFirstID), h.Get(i)) != 0 {
			curCount = 1
			curFirstID = i
		} else {
			curCount++
		}
		if curCount > maxCount {
			maxCount = curCount
			maxFirstID = curFirstID
		}
		i++
		return
	}, false /* write */)

	return maxFirstID, maxCount
}

// PurgeOldBlocks sets common, old blocks, to nil for each peer for garbage collection
func (h *SyncConfig) PurgeOldBlocks() {
	h.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		return
	}, true /* write */)
}

// PurgeAllBlocks sets all blocks, common and new blocks to nil for each peer for garbage collection
func (h *SyncConfig) PurgeAllBlocks() {
	h.ForEachPeer(func(configPeer *SyncPeerConfig) (brk bool) {
		configPeer.blockHashes = nil
		configPeer.newBlocks = nil
		return
	}, true /* write */)
}
