package legacysync

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	peer "github.com/libp2p/go-libp2p-core/peer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/block"
	headerV3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/assert"
)

func TestSyncPeerConfig_IsEqual(t *testing.T) {
	tests := []struct {
		p1, p2 *SyncPeerConfig
		exp    bool
	}{
		{
			p1: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.1",
					Port: "1",
				},
			},
			p2: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.1",
					Port: "2",
				},
			},
			exp: false,
		},
		{
			p1: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.1",
					Port: "1",
				},
			},
			p2: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.2",
					Port: "1",
				},
			},
			exp: false,
		},
		{
			p1: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.1",
					Port: "1",
				},
			},
			p2: &SyncPeerConfig{
				peer: p2p.Peer{
					IP:   "0.0.0.1",
					Port: "1",
				},
			},
			exp: true,
		},
	}
	for i, test := range tests {
		res := test.p1.IsEqual(test.p2)
		if res != test.exp {
			t.Errorf("Test %v: unexpected res %v / %v", i, res, test.exp)
		}
	}
}

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

type mockBlockchain struct {
}

func (mockBlockchain) CurrentBlock() *types.Block {
	panic("implement me")
}

func (mockBlockchain) ShardID() uint32 {
	return 0
}

func TestCreateStateSync(t *testing.T) {
	pID, _ := peer.IDFromBytes([]byte{})
	stateSync := CreateStateSync(mockBlockchain{}, "127.0.0.1", "8000", [20]byte{}, pID, false, nodeconfig.Validator)

	if stateSync == nil {
		t.Error("Unable to create stateSync")
	}
}

func TestCheckPeersDuplicity(t *testing.T) {
	tests := []struct {
		peers  []p2p.Peer
		expErr error
	}{
		{
			peers:  makePeersForTest(100),
			expErr: nil,
		},
		{
			peers: append(makePeersForTest(100), p2p.Peer{
				IP: makeTestPeerIP(0),
			}),
			expErr: errors.New("duplicate peer"),
		},
	}

	for i, test := range tests {
		err := checkPeersDuplicity(test.peers)

		if assErr := assertTestError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
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

		sz, res := limitNumPeers(ps, 1)
		res = res[:sz]

		if len(res) != test.expSize {
			t.Errorf("result size unexpected: %v / %v", len(res), test.expSize)
		}
		if err := checkPeersDuplicity(res); err != nil {
			t.Error(err)
		}
	}
}

func TestLimitPeersWithBound_random(t *testing.T) {
	ps1 := makePeersForTest(100)
	ps2 := makePeersForTest(100)
	s1, s2 := int64(1), int64(2)

	sz1, res1 := limitNumPeers(ps1, s1)
	res1 = res1[:sz1]
	sz2, res2 := limitNumPeers(ps2, s2)
	res2 = res2[:sz2]
	if reflect.DeepEqual(res1, res2) {
		t.Fatal("not randomized limit peer")
	}
}

func TestCommonBlockIter(t *testing.T) {
	size := 10
	blocks := makeTestBlocks(size)
	iter := newCommonBlockIter(blocks, testGenesis.Hash())

	itCnt := 0
	for {
		hasNext := iter.HasNext()
		b := iter.Next()
		if (b == nil) == hasNext {
			t.Errorf("has next unexpected: %v / %v", hasNext, b != nil)
		}
		if b == nil {
			break
		}
		itCnt++
	}
	if itCnt != size {
		t.Errorf("unexpected iteration count: %v / %v", itCnt, size)
	}
}

func makePeersForTest(size int) []p2p.Peer {
	ps := make([]p2p.Peer, 0, size)
	for i := 0; i != size; i++ {
		ps = append(ps, makePeerForTest(i))
	}
	return ps
}

func makePeerForTest(i interface{}) p2p.Peer {
	return p2p.Peer{
		IP: makeTestPeerIP(i),
	}
}

func makeTestPeerIP(i interface{}) string {
	return fmt.Sprintf("%v", i)
}

func assertTestError(got, expect error) error {
	if (got == nil) && (expect == nil) {
		return nil
	}
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error: [%v] / [%v]", got, expect)
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error: [%v] / [%v]", got, expect)
	}
	return nil
}

func makeTestBlocks(size int) map[int]*types.Block {
	m := make(map[int]*types.Block)
	parentHash := testGenesis.Hash()
	for i := 0; i != size; i++ {
		b := makeTestBlock(uint64(i)+1, parentHash)
		parentHash = b.Hash()

		m[i] = b
	}
	return m
}

var testGenesis = makeTestBlock(0, common.Hash{})

func makeTestBlock(bn uint64, parentHash common.Hash) *types.Block {
	testHeader := &block.Header{Header: headerV3.NewHeader()}
	testHeader.SetNumber(big.NewInt(int64(bn)))
	testHeader.SetParentHash(parentHash)
	block := types.NewBlockWithHeader(testHeader)
	return block
}

func TestSyncStatus_Get_Concurrency(t *testing.T) {
	t.Skip()

	ss := newSyncStatus(nodeconfig.Validator)
	ss.expiration = 2 * time.Second
	var (
		total   int32
		updated int32
		wg      sync.WaitGroup
		stop    = make(chan struct{})
	)

	fb := func() SyncCheckResult {
		time.Sleep(1 * time.Second)
		atomic.AddInt32(&updated, 1)
		return SyncCheckResult{IsSynchronized: true}
	}
	for i := 0; i != 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			t := time.NewTicker(20 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					atomic.AddInt32(&total, 1)
					ss.Get(fb)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(stop)
	wg.Wait()

	fmt.Printf("updated %v times\n", updated)
	fmt.Printf("total %v times\n", total)
}
