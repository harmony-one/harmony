package syncing

import (
	"bytes"
	"reflect"
	"sync"

	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/core/types"
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

// GetClient returns client pointer of downloader.Client
func (peerConfig *SyncPeerConfig) GetClient() *downloader.Client {
	return peerConfig.client
}

// CreateTestSyncPeerConfig used for testing.
func CreateTestSyncPeerConfig(client *downloader.Client, blockHashes [][]byte) *SyncPeerConfig {
	return &SyncPeerConfig{
		client:      client,
		blockHashes: blockHashes,
	}
}

// GetBlocks gets blocks by calling grpc request to the corresponding peer.
func (peerConfig *SyncPeerConfig) GetBlocks(hashes [][]byte) ([][]byte, error) {
	response := peerConfig.client.GetBlocks(hashes)
	if response == nil {
		return nil, ErrGetBlock
	}
	return response.Payload, nil
}

// Compare returns comparison result between the two sync peer configs' block hashes
func (peerConfig *SyncPeerConfig) Compare(other *SyncPeerConfig) int {
	if len(peerConfig.blockHashes) != len(other.blockHashes) {
		if len(peerConfig.blockHashes) < len(other.blockHashes) {
			return -1
		}
		return 1
	}
	for id := range peerConfig.blockHashes {
		if !reflect.DeepEqual(peerConfig.blockHashes[id], other.blockHashes[id]) {
			return bytes.Compare(peerConfig.blockHashes[id], other.blockHashes[id])
		}
	}
	return 0
}
