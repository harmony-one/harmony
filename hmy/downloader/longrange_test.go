package downloader

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

func TestDownloader_doLongRangeSync(t *testing.T) {
	targetBN := uint64(1000)
	bc := newTestBlockChain(1, nil)

	d := &Downloader{
		bc:           bc,
		syncProtocol: newTestSyncProtocol(targetBN, 32, nil),
		config: Config{
			Concurrency: 16,
			MinStreams:  16,
		},
		ctx: context.Background(),
	}
	synced, err := d.doLongRangeSync()
	if err != nil {
		t.Error(err)
	}
	if synced == 0 {
		t.Errorf("synced false")
	}
	if curNum := d.bc.CurrentBlock().NumberU64(); curNum != targetBN {
		t.Errorf("block number not expected: %v / %v", curNum, targetBN)
	}
}

func TestLrSyncIter_EstimateCurrentNumber(t *testing.T) {
	lsi := &lrSyncIter{
		p:   newTestSyncProtocol(100, 32, nil),
		ctx: context.Background(),
		config: Config{
			Concurrency: 16,
			MinStreams:  10,
		},
	}
	bn, err := lsi.estimateCurrentNumber()
	if err != nil {
		t.Error(err)
	}
	if bn != 100 {
		t.Errorf("unexpected block number: %v / %v", bn, 100)
	}
}

func TestGetBlocksManager_GetNextBatch(t *testing.T) {
	tests := []struct {
		gbm    *getBlocksManager
		expBNs []uint64
	}{
		{
			gbm: makeGetBlocksManager(
				10, 100, []uint64{9, 11, 12, 13},
				[]uint64{14, 15, 16}, []uint64{}, 0,
			),
			expBNs: []uint64{17, 18, 19, 20, 21, 22, 23, 24, 25, 26},
		},
		{
			gbm: makeGetBlocksManager(
				10, 100, []uint64{9, 13, 14, 15, 16},
				[]uint64{}, []uint64{10, 11, 12}, 0,
			),
			expBNs: []uint64{11, 12, 17, 18, 19, 20, 21, 22, 23, 24},
		},
		{
			gbm: makeGetBlocksManager(
				10, 100, []uint64{9, 13, 14, 15, 16},
				[]uint64{}, []uint64{10, 11, 12}, 120,
			),
			expBNs: []uint64{11, 12},
		},
		{
			gbm: makeGetBlocksManager(
				10, 100, []uint64{9, 13, 14, 15, 16},
				[]uint64{}, []uint64{}, 120,
			),
			expBNs: []uint64{},
		},
		{
			gbm: makeGetBlocksManager(
				10, 20, []uint64{9, 13, 14, 15, 16},
				[]uint64{}, []uint64{}, 0,
			),
			expBNs: []uint64{11, 12, 17, 18, 19, 20},
		},
		{
			gbm: makeGetBlocksManager(
				10, 100, []uint64{9, 13, 14, 15, 16},
				[]uint64{}, []uint64{}, 0,
			),
			expBNs: []uint64{11, 12, 17, 18, 19, 20, 21, 22, 23, 24},
		},
	}

	for i, test := range tests {
		if i < 4 {
			continue
		}
		batch := test.gbm.GetNextBatch()
		if len(test.expBNs) != len(batch) {
			t.Errorf("Test %v: unexpected size [%v] / [%v]", i, batch, test.expBNs)
		}
		for i := range test.expBNs {
			if test.expBNs[i] != batch[i] {
				t.Errorf("Test %v: [%v] / [%v]", i, batch, test.expBNs)
			}
		}
	}
}

func TestLrSyncIter_FetchAndInsertBlocks(t *testing.T) {
	targetBN := uint64(1000)
	chain := newTestBlockChain(0, nil)
	protocol := newTestSyncProtocol(targetBN, 32, nil)
	ctx := context.Background()

	lsi := &lrSyncIter{
		bc:  chain,
		d:   &Downloader{bc: chain},
		p:   protocol,
		gbm: nil,
		config: Config{
			Concurrency: 100,
		},
		ctx: ctx,
	}
	lsi.fetchAndInsertBlocks(targetBN)

	if err := fetchAndInsertBlocksResultCheck(lsi, targetBN, initStreamNum); err != nil {
		t.Error(err)
	}
}

// When FetchAndInsertBlocks, one request has an error
func TestLrSyncIter_FetchAndInsertBlocks_ErrRequest(t *testing.T) {
	targetBN := uint64(1000)
	var once sync.Once
	errHook := func(bn uint64) error {
		var err error
		once.Do(func() {
			err = errors.New("test error expected")
		})
		return err
	}
	chain := newTestBlockChain(0, nil)
	protocol := newTestSyncProtocol(targetBN, 32, errHook)
	ctx := context.Background()

	lsi := &lrSyncIter{
		bc:  chain,
		d:   &Downloader{bc: chain},
		p:   protocol,
		gbm: nil,
		config: Config{
			Concurrency: 100,
		},
		ctx: ctx,
	}
	lsi.fetchAndInsertBlocks(targetBN)

	if err := fetchAndInsertBlocksResultCheck(lsi, targetBN, initStreamNum-1); err != nil {
		t.Error(err)
	}
}

// When FetchAndInsertBlocks, one insertion has an error
func TestLrSyncIter_FetchAndInsertBlocks_ErrInsert(t *testing.T) {
	targetBN := uint64(1000)
	var once sync.Once
	errHook := func(bn uint64) error {
		var err error
		once.Do(func() {
			err = errors.New("test error expected")
		})
		return err
	}
	chain := newTestBlockChain(0, errHook)
	protocol := newTestSyncProtocol(targetBN, 32, nil)
	ctx := context.Background()

	lsi := &lrSyncIter{
		bc:  chain,
		d:   &Downloader{bc: chain},
		p:   protocol,
		gbm: nil,
		config: Config{
			Concurrency: 100,
		},
		ctx: ctx,
	}
	lsi.fetchAndInsertBlocks(targetBN)

	if err := fetchAndInsertBlocksResultCheck(lsi, targetBN, initStreamNum-1); err != nil {
		t.Error(err)
	}
}

// When FetchAndInsertBlocks, randomly error happens
func TestLrSyncIter_FetchAndInsertBlocks_RandomErr(t *testing.T) {
	targetBN := uint64(10000)
	rand.Seed(0)
	errHook := func(bn uint64) error {
		// 10% error happens
		if rand.Intn(10)%10 == 0 {
			return errors.New("error expected")
		}
		return nil
	}
	chain := newTestBlockChain(0, errHook)
	protocol := newTestSyncProtocol(targetBN, 32, errHook)
	ctx := context.Background()

	lsi := &lrSyncIter{
		bc:  chain,
		d:   &Downloader{bc: chain},
		p:   protocol,
		gbm: nil,
		config: Config{
			Concurrency: 100,
		},
		ctx: ctx,
	}
	lsi.fetchAndInsertBlocks(targetBN)

	if err := fetchAndInsertBlocksResultCheck(lsi, targetBN, minStreamNum); err != nil {
		t.Error(err)
	}
}

func fetchAndInsertBlocksResultCheck(lsi *lrSyncIter, targetBN uint64, expNumStreams int) error {
	if bn := lsi.bc.CurrentBlock().NumberU64(); bn != targetBN {
		return fmt.Errorf("did not reached targetBN: %v / %v", bn, targetBN)
	}
	lsi.gbm.lock.Lock()
	defer lsi.gbm.lock.Unlock()
	if len(lsi.gbm.processing) != 0 {
		return fmt.Errorf("not empty processing: %v", lsi.gbm.processing)
	}
	if len(lsi.gbm.requesting) != 0 {
		return fmt.Errorf("not empty requesting: %v", lsi.gbm.requesting)
	}
	if lsi.gbm.retries.length() != 0 {
		return fmt.Errorf("not empty retries: %v", lsi.gbm.retries)
	}
	if lsi.gbm.rq.length() != 0 {
		return fmt.Errorf("not empty result queue: %v", lsi.gbm.rq.results)
	}
	tsp := lsi.p.(*testSyncProtocol)
	if len(tsp.streamIDs) != expNumStreams {
		return fmt.Errorf("num streams not expected: %v / %v", len(tsp.streamIDs), expNumStreams)
	}
	return nil
}

func TestComputeBNMaxVote(t *testing.T) {
	tests := []struct {
		votes map[sttypes.StreamID]uint64
		exp   uint64
	}{
		{
			votes: map[sttypes.StreamID]uint64{
				makeStreamID(0): 10,
				makeStreamID(1): 10,
				makeStreamID(2): 20,
			},
			exp: 10,
		},
		{
			votes: map[sttypes.StreamID]uint64{
				makeStreamID(0): 10,
				makeStreamID(1): 20,
			},
			exp: 20,
		},
		{
			votes: map[sttypes.StreamID]uint64{
				makeStreamID(0): 20,
				makeStreamID(1): 10,
				makeStreamID(2): 20,
			},
			exp: 20,
		},
	}

	for i, test := range tests {
		res := computeBlockNumberByMaxVote(test.votes)
		if res != test.exp {
			t.Errorf("Test %v: unexpected bn %v / %v", i, res, test.exp)
		}
	}
}

func makeGetBlocksManager(curBN, targetBN uint64, requesting, processing, retries []uint64, sizeRQ int) *getBlocksManager {
	chain := newTestBlockChain(curBN, nil)
	requestingM := make(map[uint64]struct{})
	for _, bn := range requesting {
		requestingM[bn] = struct{}{}
	}
	processingM := make(map[uint64]struct{})
	for _, bn := range processing {
		processingM[bn] = struct{}{}
	}
	retriesPN := newPrioritizedNumbers()
	for _, retry := range retries {
		retriesPN.push(retry)
	}
	rq := newResultQueue()
	for i := uint64(0); i != uint64(sizeRQ); i++ {
		rq.addBlockResults(makeTestBlocks([]uint64{i + curBN}), "")
	}
	return &getBlocksManager{
		chain:      chain,
		targetBN:   targetBN,
		requesting: requestingM,
		processing: processingM,
		retries:    retriesPN,
		rq:         rq,
		resultC:    make(chan struct{}, 1),
	}
}
