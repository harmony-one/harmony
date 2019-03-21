package syncing

import (
	"testing"

	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/stretchr/testify/assert"
)

// Simple test for IncorrectResponse
func TestCreateTestSyncPeerConfig(t *testing.T) {
	client := &downloader.Client{}
	blockHashes := [][]byte{[]byte{}}
	syncPeerConfig := CreateTestSyncPeerConfig(client, blockHashes)
	assert.Equal(t, client, syncPeerConfig.GetClient(), "error")
}

// Simple test for IncorrectResponse
func TestCompareSyncPeerConfigByblockHashes(t *testing.T) {
	client := &downloader.Client{}
	blockHashes1 := [][]byte{[]byte{1, 2, 3}}
	syncPeerConfig1 := CreateTestSyncPeerConfig(client, blockHashes1)
	blockHashes2 := [][]byte{[]byte{1, 2, 4}}
	syncPeerConfig2 := CreateTestSyncPeerConfig(client, blockHashes2)

	// syncPeerConfig1 is less than syncPeerConfig2
	assert.Equal(t, CompareSyncPeerConfigByblockHashes(syncPeerConfig1, syncPeerConfig2), -1, "syncPeerConfig1 is less than syncPeerConfig2")

	// syncPeerConfig1 is greater than syncPeerConfig2
	blockHashes1[0][2] = 5
	assert.Equal(t, CompareSyncPeerConfigByblockHashes(syncPeerConfig1, syncPeerConfig2), 1, "syncPeerConfig1 is greater than syncPeerConfig2")

	// syncPeerConfig1 is equal to syncPeerConfig2
	blockHashes1[0][2] = 4
	assert.Equal(t, CompareSyncPeerConfigByblockHashes(syncPeerConfig1, syncPeerConfig2), 0, "syncPeerConfig1 is equal to syncPeerConfig2")

	// syncPeerConfig1 is less than syncPeerConfig2
	blockHashes1 = blockHashes1[:1]
	assert.Equal(t, CompareSyncPeerConfigByblockHashes(syncPeerConfig1, syncPeerConfig2), 0, "syncPeerConfig1 is less than syncPeerConfig2")
}

func TestCreateStateSync(t *testing.T) {
	stateSync := CreateStateSync("127.0.0.1", "8000", [20]byte{})

	if stateSync == nil {
		t.Error("Unable to create stateSync")
	}
}
