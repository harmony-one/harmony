package syncing

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/api/service/syncing/downloader"
	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/assert"
)

// Simple test for IncorrectResponse
func TestCreateTestSyncPeerConfig(t *testing.T) {
	client := &downloader.Client{}
	blockHashes := [][]byte{{}}
	syncPeerConfig := CreateTestSyncPeerConfig(client, blockHashes)
	assert.Equal(t, client, syncPeerConfig.GetClient(), "error")
}

// Simple test for IncorrectResponse
func TestCompareSyncPeerConfigByblockHashes(t *testing.T) {
	client := &downloader.Client{}
	blockHashes1 := [][]byte{{1, 2, 3}}
	syncPeerConfig1 := CreateTestSyncPeerConfig(client, blockHashes1)
	blockHashes2 := [][]byte{{1, 2, 4}}
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
	assert.Equal(t,
		CompareSyncPeerConfigByblockHashes(
			syncPeerConfig1, syncPeerConfig2,
		),
		0, "syncPeerConfig1 is less than syncPeerConfig2")
}

func TestCreateStateSync(t *testing.T) {
	stateSync := CreateStateSync("127.0.0.1", "8000", [20]byte{})

	if stateSync == nil {
		t.Error("Unable to create stateSync")
	}
}

func TestLimitPeersWithBound(t *testing.T) {
	tests := []struct {
		size    int
		expSize int
	}{
		{0, 0},
		{1, 1},
		{3, 3},
		{4, 3},
		{7, 3},
		{8, 4},
		{10, 5},
		{11, 5},
		{100, 5},
	}
	for _, test := range tests {
		ps := makePeersForTest(test.size)

		res := limitNumPeers(ps, 1)

		if len(res) != test.expSize {
			t.Errorf("result size unexpected: %v / %v", len(res), test.expSize)
		}
		if err := checkTestPeerDuplicity(res); err != nil {
			t.Error(err)
		}
	}
}

func TestLimitPeersWithBound_random(t *testing.T) {
	ps1 := makePeersForTest(100)
	ps2 := makePeersForTest(100)
	s1, s2 := int64(1), int64(2)

	res1 := limitNumPeers(ps1, s1)
	res2 := limitNumPeers(ps2, s2)
	if reflect.DeepEqual(res1, res2) {
		t.Fatal("not randomized limit peer")
	}
}

func makePeersForTest(size int) []p2p.Peer {
	ps := make([]p2p.Peer, 0, size)
	for i := 0; i != size; i++ {
		ps = append(ps, p2p.Peer{
			IP: makeTestPeerIP(i),
		})
	}
	return ps
}

func checkTestPeerDuplicity(ps []p2p.Peer) error {
	m := make(map[string]struct{})
	for _, p := range ps {
		if _, ok := m[p.IP]; ok {
			return fmt.Errorf("duplicate ip")
		}
		m[p.IP] = struct{}{}
	}
	return nil
}

func makeTestPeerIP(i interface{}) string {
	return fmt.Sprintf("%v", i)
}
