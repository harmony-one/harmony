package downloader

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

type testBlockChain struct {
	curBN         uint64
	insertErrHook func(bn uint64) error
	lock          sync.Mutex
}

func newTestBlockChain(curBN uint64, insertErrHook func(bn uint64) error) *testBlockChain {
	return &testBlockChain{
		curBN:         curBN,
		insertErrHook: insertErrHook,
	}
}

func (bc *testBlockChain) CurrentBlock() *types.Block {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	return makeTestBlock(bc.curBN)
}

func (bc *testBlockChain) currentBlockNumber() uint64 {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	return bc.curBN
}

func (bc *testBlockChain) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	for i, block := range chain {
		if bc.insertErrHook != nil {
			if err := bc.insertErrHook(block.NumberU64()); err != nil {
				return i, err
			}
		}
		if block.NumberU64() <= bc.curBN {
			continue
		}
		if block.NumberU64() != bc.curBN+1 {
			return i, fmt.Errorf("not expected block number: %v / %v", block.NumberU64(), bc.curBN+1)
		}
		bc.curBN++
	}
	return len(chain), nil
}

func (bc *testBlockChain) changeBlockNumber(val uint64) {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	bc.curBN = val
}

func (bc *testBlockChain) ShardID() uint32 {
	return 0
}

const (
	initStreamNum = 32
	minStreamNum  = 16
)

type testSyncProtocol struct {
	streamIDs      []sttypes.StreamID
	remoteChain    *testBlockChain
	requestErrHook func(uint64) error

	curIndex   int
	numStreams int
	lock       sync.Mutex
}

func newTestSyncProtocol(targetBN uint64, numStreams int, requestErrHook func(uint64) error) *testSyncProtocol {
	return &testSyncProtocol{
		streamIDs:      makeStreamIDs(numStreams),
		remoteChain:    newTestBlockChain(targetBN, nil),
		requestErrHook: requestErrHook,
		curIndex:       0,
		numStreams:     numStreams,
	}
}

func (sp *testSyncProtocol) GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	bn := sp.remoteChain.currentBlockNumber()

	return bn, sp.nextStreamID(), nil
}

func (sp *testSyncProtocol) GetBlocksByNumber(ctx context.Context, bns []uint64, opts ...syncproto.Option) ([]*types.Block, sttypes.StreamID, error) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	res := make([]*types.Block, 0, len(bns))
	for _, bn := range bns {
		if sp.requestErrHook != nil {
			if err := sp.requestErrHook(bn); err != nil {
				return nil, sp.nextStreamID(), err
			}
		}
		if bn > sp.remoteChain.currentBlockNumber() {
			res = append(res, nil)
		} else {
			res = append(res, makeTestBlock(bn))
		}
	}
	return res, sp.nextStreamID(), nil
}

func (sp *testSyncProtocol) GetBlockHashes(ctx context.Context, bns []uint64, opts ...syncproto.Option) ([]common.Hash, sttypes.StreamID, error) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	res := make([]common.Hash, 0, len(bns))
	for _, bn := range bns {
		if sp.requestErrHook != nil {
			if err := sp.requestErrHook(bn); err != nil {
				return nil, sp.nextStreamID(), err
			}
		}
		if bn > sp.remoteChain.currentBlockNumber() {
			res = append(res, emptyHash)
		} else {
			res = append(res, makeTestBlockHash(bn))
		}
	}
	return res, sp.nextStreamID(), nil
}

func (sp *testSyncProtocol) GetBlocksByHashes(ctx context.Context, hs []common.Hash, opts ...syncproto.Option) ([]*types.Block, sttypes.StreamID, error) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	res := make([]*types.Block, 0, len(hs))
	for _, h := range hs {
		bn := testHashToNumber(h)
		if sp.requestErrHook != nil {
			if err := sp.requestErrHook(bn); err != nil {
				return nil, sp.nextStreamID(), err
			}
		}
		if bn > sp.remoteChain.currentBlockNumber() {
			res = append(res, nil)
		} else {
			res = append(res, makeTestBlock(bn))
		}
	}
	return res, sp.nextStreamID(), nil
}

func (sp *testSyncProtocol) RemoveStream(target sttypes.StreamID) error {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	for i, stid := range sp.streamIDs {
		if stid == target {
			if i == len(sp.streamIDs)-1 {
				sp.streamIDs = sp.streamIDs[:i]
			} else {
				sp.streamIDs = append(sp.streamIDs[:i], sp.streamIDs[i+1:]...)
			}
			// mock discovery
			if len(sp.streamIDs) < minStreamNum {
				sp.streamIDs = append(sp.streamIDs, makeStreamID(sp.numStreams))
				sp.numStreams++
			}
		}
	}
	return nil
}

func (sp *testSyncProtocol) NumStreams() int {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	return len(sp.streamIDs)
}

func (sp *testSyncProtocol) SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription {
	var evtFeed event.Feed
	go func() {
		sp.lock.Lock()
		num := len(sp.streamIDs)
		sp.lock.Unlock()
		for i := 0; i != num; i++ {
			evtFeed.Send(streammanager.EvtStreamAdded{Stream: nil})
		}
	}()
	return evtFeed.Subscribe(ch)
}

// TODO: add with whitelist stuff
func (sp *testSyncProtocol) nextStreamID() sttypes.StreamID {
	if sp.curIndex >= len(sp.streamIDs) {
		sp.curIndex = 0
	}
	index := sp.curIndex
	sp.curIndex++
	if sp.curIndex >= len(sp.streamIDs) {
		sp.curIndex = 0
	}
	return sp.streamIDs[index]
}

func (sp *testSyncProtocol) changeBlockNumber(val uint64) {
	sp.remoteChain.changeBlockNumber(val)
}

func makeStreamIDs(size int) []sttypes.StreamID {
	res := make([]sttypes.StreamID, 0, size)
	for i := 0; i != size; i++ {
		res = append(res, makeStreamID(i))
	}
	return res
}

func makeStreamID(index int) sttypes.StreamID {
	return sttypes.StreamID(fmt.Sprintf("test stream %v", index))
}

var (
	hashNumberMap  = map[common.Hash]uint64{}
	computed       uint64
	hashNumberLock sync.Mutex
)

func testHashToNumber(h common.Hash) uint64 {
	hashNumberLock.Lock()
	defer hashNumberLock.Unlock()

	if h == emptyHash {
		panic("not allowed")
	}
	if bn, ok := hashNumberMap[h]; ok {
		return bn
	}
	for ; ; computed++ {
		ch := makeTestBlockHash(computed)
		hashNumberMap[ch] = computed
		if ch == h {
			return computed
		}
	}
}

func testNumberToHashes(nums []uint64) []common.Hash {
	hashes := make([]common.Hash, 0, len(nums))
	for _, num := range nums {
		hashes = append(hashes, makeTestBlockHash(num))
	}
	return hashes
}
