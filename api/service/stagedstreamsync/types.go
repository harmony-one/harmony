package stagedstreamsync

import (
	"container/heap"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

var (
	emptyHash common.Hash
)

type status struct {
	isSyncing     bool
	targetBN      uint64
	pivotBlock    *types.Block
	cycleSyncMode SyncMode
	statesSynced  bool
	lock          sync.RWMutex
}

func NewStatus() *status {
	return &status{
		cycleSyncMode: FullSync,
	}
}

func (s *status) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.isSyncing = false
	s.targetBN = 0
	s.pivotBlock = nil
	s.cycleSyncMode = FullSync
	s.statesSynced = false
}

func (s *status) StartSyncing() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.isSyncing = true
}

func (s *status) IsSyncing() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isSyncing
}

func (s *status) FinishSyncing() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.isSyncing = false
}

func (s *status) SetPivotBlock(pivot *types.Block) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.pivotBlock = pivot
}

func (s *status) GetPivotBlock() *types.Block {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.pivotBlock
}

func (s *status) HasPivotBlock() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.pivotBlock != nil
}

func (s *status) GetPivotBlockNumber() uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.pivotBlock == nil {
		return 0
	}
	return s.pivotBlock.NumberU64()
}

func (s *status) SetCycleSyncMode(sm SyncMode) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.cycleSyncMode = sm
}

func (s *status) GetCycleSyncMode() SyncMode {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.cycleSyncMode
}

func (s *status) IsFullSyncCycle() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.cycleSyncMode == FullSync
}

func (s *status) SetStatesSynced(ss bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.statesSynced = ss
}

func (s *status) IsStatesSynced() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.statesSynced
}

func (s *status) SetTargetBN(tbn uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.targetBN = tbn
}

func (s *status) GetTargetBN() uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.targetBN
}

func (s *status) Get() (bool, uint64) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isSyncing, s.targetBN
}

type getBlocksResult struct {
	bns    []uint64
	blocks []*types.Block
	stid   sttypes.StreamID
}

type resultQueue struct {
	results *priorityQueue
	lock    sync.RWMutex
}

func newResultQueue() *resultQueue {
	pq := make(priorityQueue, 0, 200) // 200 - rough estimate
	heap.Init(&pq)
	return &resultQueue{
		results: &pq,
	}
}

// addBlockResults adds the blocks to the result queue to be processed by insertChainLoop.
// If a nil block is detected in the block list, will not process further blocks.
func (rq *resultQueue) addBlockResults(blocks []*types.Block, stid sttypes.StreamID) {
	rq.lock.Lock()
	defer rq.lock.Unlock()

	for _, block := range blocks {
		if block == nil {
			continue
		}
		heap.Push(rq.results, &blockResult{
			block: block,
			stid:  stid,
		})
	}
}

// popBlockResults pops a continuous list of blocks starting at expStartBN with capped size.
// Returns the stale block numbers as the second return value.
func (rq *resultQueue) popBlockResults(expStartBN uint64, cap int) ([]*blockResult, []uint64) {
	rq.lock.Lock()
	defer rq.lock.Unlock()

	var (
		res    = make([]*blockResult, 0, cap)
		stales []uint64
	)

	for cnt := 0; rq.results.Len() > 0 && cnt < cap; cnt++ {
		br := heap.Pop(rq.results).(*blockResult)
		// stale block number
		if br.block.NumberU64() < expStartBN {
			stales = append(stales, br.block.NumberU64())
			continue
		}
		if br.block.NumberU64() != expStartBN {
			heap.Push(rq.results, br)
			return res, stales
		}
		res = append(res, br)
		expStartBN++
	}
	return res, stales
}

// removeResultsByStreamID removes the block results of the given stream,
// returns the block numbers removed from the queue
func (rq *resultQueue) removeResultsByStreamID(stid sttypes.StreamID) []uint64 {
	rq.lock.Lock()
	defer rq.lock.Unlock()

	var removed []uint64

	for i := 0; i < rq.results.Len(); i++ {
		blockRes := (*rq.results)[i].(*blockResult)
		if blockRes.stid == stid {
			rq.removeByIndex(i)
			removed = append(removed, blockRes.block.NumberU64())
			i-- // Adjust index after removal
		}
	}
	return removed
}

func (rq *resultQueue) length() int {
	rq.lock.RLock()
	defer rq.lock.RUnlock()
	return len(*rq.results)
}

func (rq *resultQueue) removeByIndex(index int) {
	heap.Remove(rq.results, index)
}

// bnPrioritizedItem is the item which uses block number to determine its priority
type bnPrioritizedItem interface {
	getBlockNumber() uint64
}

type blockResult struct {
	block *types.Block
	stid  sttypes.StreamID
}

func (br *blockResult) getBlockNumber() uint64 {
	return br.block.NumberU64()
}

func blockResultsToBlocks(results []*blockResult) []*types.Block {
	blocks := make([]*types.Block, 0, len(results))

	for _, result := range results {
		blocks = append(blocks, result.block)
	}
	return blocks
}

type (
	prioritizedNumber uint64

	prioritizedNumbers struct {
		q *priorityQueue
	}
)

func (b prioritizedNumber) getBlockNumber() uint64 {
	return uint64(b)
}

func newPrioritizedNumbers() *prioritizedNumbers {
	pqs := make(priorityQueue, 0)
	heap.Init(&pqs)
	return &prioritizedNumbers{
		q: &pqs,
	}
}

func (pbs *prioritizedNumbers) push(bn uint64) {
	heap.Push(pbs.q, prioritizedNumber(bn))
}

func (pbs *prioritizedNumbers) pop() uint64 {
	if pbs.q.Len() == 0 {
		return 0
	}
	item := heap.Pop(pbs.q)
	return uint64(item.(prioritizedNumber))
}

func (pbs *prioritizedNumbers) length() int {
	return len(*pbs.q)
}

type (
	blockByNumber types.Block

	// blocksByNumber is the priority queue ordered by number
	blocksByNumber struct {
		q   *priorityQueue
		cap int
	}
)

func (b *blockByNumber) getBlockNumber() uint64 {
	raw := (*types.Block)(b)
	return raw.NumberU64()
}

func newBlocksByNumber(cap int) *blocksByNumber {
	pqs := make(priorityQueue, 0)
	heap.Init(&pqs)
	return &blocksByNumber{
		q:   &pqs,
		cap: cap,
	}
}

func (bs *blocksByNumber) push(b *types.Block) {
	heap.Push(bs.q, (*blockByNumber)(b))
	for bs.q.Len() > bs.cap {
		heap.Pop(bs.q)
	}
}

func (bs *blocksByNumber) pop() *types.Block {
	if bs.q.Len() == 0 {
		return nil
	}
	item := heap.Pop(bs.q)
	return (*types.Block)(item.(*blockByNumber))
}

func (bs *blocksByNumber) len() int {
	return bs.q.Len()
}

// priorityQueue is a priority queue with lowest block number with highest priority
type priorityQueue []bnPrioritizedItem

func (q priorityQueue) Len() int {
	return len(q)
}

func (q priorityQueue) Less(i, j int) bool {
	bn1 := q[i].getBlockNumber()
	bn2 := q[j].getBlockNumber()
	return bn1 < bn2 // small block number has higher priority
}

func (q priorityQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q *priorityQueue) Push(x interface{}) {
	item, ok := x.(bnPrioritizedItem)
	if !ok {
		panic(ErrWrongGetBlockNumberType)
	}
	*q = append(*q, item)
}

func (q *priorityQueue) Pop() interface{} {
	prev := *q
	n := len(prev)
	if n == 0 {
		return nil
	}
	res := prev[n-1]
	*q = prev[0 : n-1]
	return res
}
