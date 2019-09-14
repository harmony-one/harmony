package syncing

import (
	"container/heap"
	"testing"

	"github.com/harmony-one/harmony/api/service/syncing/downloader"
)

func TestSyncConfig(t *testing.T) {
	client := &downloader.Client{}
	blockHashes1 := [][]byte{{2, 2, 3}}
	syncPeerConfig1 := CreateTestSyncPeerConfig(client, blockHashes1)
	blockHashes2 := [][]byte{{1, 2, 4}}
	syncPeerConfig2 := CreateTestSyncPeerConfig(client, blockHashes2)
	blockHashes3 := [][]byte{{1, 2, 5}}
	syncPeerConfig3 := CreateTestSyncPeerConfig(client, blockHashes3)

	h := &SyncConfig{}
	heap.Init(h)
	heap.Push(h, syncPeerConfig1)
	heap.Push(h, syncPeerConfig2)
	heap.Push(h, syncPeerConfig3)

	expected := []*SyncPeerConfig{syncPeerConfig2, syncPeerConfig3, syncPeerConfig1}
	i := 0
	for h.Len() > 0 {
		value := heap.Pop(h).(*SyncPeerConfig)
		if CompareSyncPeerConfigByBlockHashes(value, expected[i]) != 0 {
			t.Fatalf("unexpected result for max in heap. Expected: %s, but got %s",
				expected[i].peerHash, value.peerHash)
		}
		i++
	}
}
