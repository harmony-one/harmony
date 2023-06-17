package downloader

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
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

func (bc *testBlockChain) CurrentHeader() *block.Header {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	return makeTestBlock(bc.curBN).Header()
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

func (bc *testBlockChain) ShardID() uint32                                          { return 0 }
func (bc *testBlockChain) ReadShardState(epoch *big.Int) (*shard.State, error)      { return nil, nil }
func (bc *testBlockChain) TrieNode(hash common.Hash) ([]byte, error)                { return []byte{}, nil }
func (bc *testBlockChain) Config() *params.ChainConfig                              { return nil }
func (bc *testBlockChain) WriteCommitSig(blockNum uint64, lastCommits []byte) error { return nil }
func (bc *testBlockChain) GetHeader(hash common.Hash, number uint64) *block.Header  { return nil }
func (bc *testBlockChain) GetHeaderByNumber(number uint64) *block.Header            { return nil }
func (bc *testBlockChain) GetHeaderByHash(hash common.Hash) *block.Header           { return nil }
func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block    { return nil }
func (bc *testBlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts        { return nil }
func (bc *testBlockChain) ContractCode(hash common.Hash) ([]byte, error)            { return []byte{}, nil }
func (bc *testBlockChain) ValidatorCode(hash common.Hash) ([]byte, error)           { return []byte{}, nil }
func (bc *testBlockChain) ReadValidatorList() ([]common.Address, error)             { return nil, nil }
func (bc *testBlockChain) ReadCommitSig(blockNum uint64) ([]byte, error)            { return nil, nil }
func (bc *testBlockChain) ReadBlockRewardAccumulator(uint64) (*big.Int, error)      { return nil, nil }
func (bc *testBlockChain) ValidatorCandidates() []common.Address                    { return nil }
func (bc *testBlockChain) Engine() engine.Engine                                    { return &dummyEngine{} }
func (cr *testBlockChain) ReadValidatorInformationAtState(
	addr common.Address, state *state.DB,
) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (cr *testBlockChain) StateAt(root common.Hash) (*state.DB, error) {
	return nil, nil
}
func (bc *testBlockChain) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *testBlockChain) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}
func (bc *testBlockChain) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}
func (bc *testBlockChain) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return nil, nil
}
func (bc *testBlockChain) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	return nil, nil
}

type dummyEngine struct{}

func (e *dummyEngine) VerifyHeader(engine.ChainReader, *block.Header, bool) error {
	return nil
}
func (e *dummyEngine) VerifyHeaderSignature(engine.ChainReader, *block.Header, bls.SerializedSignature, []byte) error {
	return nil
}
func (e *dummyEngine) VerifyCrossLink(engine.ChainReader, types.CrossLink) error {
	return nil
}
func (e *dummyEngine) VerifyVRF(chain engine.ChainReader, header *block.Header) error {
	return nil
}
func (e *dummyEngine) VerifyHeaders(engine.ChainReader, []*block.Header, []bool) (chan<- struct{}, <-chan error) {
	return nil, nil
}
func (e *dummyEngine) VerifySeal(engine.ChainReader, *block.Header) error { return nil }
func (e *dummyEngine) VerifyShardState(engine.ChainReader, engine.ChainReader, *block.Header) error {
	return nil
}
func (e *dummyEngine) Beaconchain() engine.ChainReader   { return nil }
func (e *dummyEngine) SetBeaconchain(engine.ChainReader) {}
func (e *dummyEngine) Finalize(
	chain engine.ChainReader, beacon engine.ChainReader, header *block.Header,
	state *state.DB, txs []*types.Transaction,
	receipts []*types.Receipt, outcxs []*types.CXReceipt,
	incxs []*types.CXReceiptsProof, stks staking.StakingTransactions,
	doubleSigners slash.Records, sigsReady chan bool, viewID func() uint64,
) (*types.Block, reward.Reader, error) {
	return nil, nil, nil
}

type testInsertHelper struct {
	bc *testBlockChain
}

func (ch *testInsertHelper) verifyAndInsertBlock(block *types.Block) error {
	_, err := ch.bc.InsertChain(types.Blocks{block}, true)
	return err
}
func (ch *testInsertHelper) verifyAndInsertBlocks(blocks types.Blocks) (int, error) {
	return ch.bc.InsertChain(blocks, true)
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

func (sp *testSyncProtocol) RemoveStream(target sttypes.StreamID) {
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
