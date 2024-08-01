package downloader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core/types"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// doLongRangeSync does the long range sync.
// One LongRangeSync consists of several iterations.
// For each iteration, estimate the current block number, then fetch block & insert to blockchain
func (d *Downloader) doLongRangeSync() (int, error) {
	var totalInserted int

	for {
		ctx, cancel := context.WithCancel(d.ctx)

		iter := &lrSyncIter{
			bc:     d.bc,
			p:      d.syncProtocol,
			d:      d,
			ctx:    ctx,
			config: d.config,
			logger: d.logger.With().Str("mode", "long range").Logger(),
		}
		if err := iter.doLongRangeSync(); err != nil {
			cancel()
			return totalInserted + iter.inserted, err
		}
		cancel()

		totalInserted += iter.inserted

		if iter.inserted < lastMileThres {
			return totalInserted, nil
		}
	}
}

// lrSyncIter run a single iteration of a full long range sync.
// First get a rough estimate of the current block height, and then sync to this
// block number
type lrSyncIter struct {
	bc blockChain
	p  syncProtocol
	d  *Downloader

	gbm      *getBlocksManager // initialized when finished get block number
	inserted int

	config Config
	ctx    context.Context
	logger zerolog.Logger
}

func (lsi *lrSyncIter) doLongRangeSync() error {
	if err := lsi.checkPrerequisites(); err != nil {
		return err
	}
	bn, err := lsi.estimateCurrentNumber()
	if err != nil {
		return err
	}
	if curBN := lsi.bc.CurrentBlock().NumberU64(); bn <= curBN {
		lsi.logger.Info().Uint64("current number", curBN).Uint64("target number", bn).
			Msg("early return of long range sync")
		return nil
	}

	lsi.d.startSyncing()
	defer lsi.d.finishSyncing()

	lsi.logger.Info().Uint64("target number", bn).Msg("estimated remote current number")
	lsi.d.status.setTargetBN(bn)

	return lsi.fetchAndInsertBlocks(bn)
}

func (lsi *lrSyncIter) checkPrerequisites() error {
	return lsi.checkHaveEnoughStreams()
}

// estimateCurrentNumber roughly estimate the current block number.
// The block number does not need to be exact, but just a temporary target of the iteration
func (lsi *lrSyncIter) estimateCurrentNumber() (uint64, error) {
	var (
		cnResults = make(map[sttypes.StreamID]uint64)
		lock      sync.Mutex
		wg        sync.WaitGroup
	)
	wg.Add(lsi.config.Concurrency)
	for i := 0; i != lsi.config.Concurrency; i++ {
		go func() {
			defer wg.Done()
			bn, stid, err := lsi.doGetCurrentNumberRequest()
			if err != nil {
				lsi.logger.Err(err).Str("streamID", string(stid)).
					Msg("getCurrentNumber request failed. Removing stream")
				if !errors.Is(err, context.Canceled) {
					lsi.p.RemoveStream(stid)
				}
				return
			}
			lock.Lock()
			cnResults[stid] = bn
			lock.Unlock()
		}()
	}
	wg.Wait()

	if len(cnResults) == 0 {
		select {
		case <-lsi.ctx.Done():
			return 0, lsi.ctx.Err()
		default:
		}
		return 0, errors.New("zero block number response from remote nodes")
	}
	bn := computeBlockNumberByMaxVote(cnResults)
	return bn, nil
}

func (lsi *lrSyncIter) doGetCurrentNumberRequest() (uint64, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(lsi.ctx, 10*time.Second)
	defer cancel()

	bn, stid, err := lsi.p.GetCurrentBlockNumber(ctx, syncproto.WithHighPriority())
	if err != nil {
		return 0, stid, err
	}
	return bn, stid, nil
}

// fetchAndInsertBlocks use the pipeline pattern to boost the performance of inserting blocks.
// TODO: For resharding, use the pipeline to do fast sync (epoch loop, header loop, body loop)
func (lsi *lrSyncIter) fetchAndInsertBlocks(targetBN uint64) error {
	gbm := newGetBlocksManager(lsi.bc, targetBN, lsi.logger)
	lsi.gbm = gbm

	// Setup workers to fetch blocks from remote node
	for i := 0; i != lsi.config.Concurrency; i++ {
		worker := &getBlocksWorker{
			gbm:      gbm,
			protocol: lsi.p,
		}
		go worker.workLoop(lsi.ctx)
	}

	// insert the blocks to chain. Return when the target block number is reached.
	lsi.insertChainLoop(targetBN)

	select {
	case <-lsi.ctx.Done():
		return lsi.ctx.Err()
	default:
	}
	return nil
}

func (lsi *lrSyncIter) insertChainLoop(targetBN uint64) {
	var (
		gbm     = lsi.gbm
		t       = time.NewTicker(100 * time.Millisecond)
		resultC = make(chan struct{}, 1)
	)
	defer t.Stop()

	trigger := func() {
		select {
		case resultC <- struct{}{}:
		default:
		}
	}

	for {
		select {
		case <-lsi.ctx.Done():
			return

		case <-t.C:
			// Redundancy, periodically check whether there is blocks that can be processed
			trigger()

		case <-gbm.resultC:
			// New block arrive in resultQueue
			trigger()

		case <-resultC:
			blockResults := gbm.PullContinuousBlocks(blocksPerInsert)
			if len(blockResults) > 0 {
				lsi.processBlocks(blockResults, targetBN)
				// more blocks is expected being able to be pulled from queue
				trigger()
			}
			if lsi.bc.CurrentBlock().NumberU64() >= targetBN {
				return
			}
		}
	}
}

func (lsi *lrSyncIter) processBlocks(results []*blockResult, targetBN uint64) {
	blocks := blockResultsToBlocks(results)

	for i, block := range blocks {
		if err := verifyAndInsertBlock(lsi.bc, block); err != nil {
			lsi.logger.Warn().Err(err).Uint64("target block", targetBN).
				Uint64("block number", block.NumberU64()).
				Msg("insert blocks failed in long range")
			pl := lsi.d.promLabels()
			pl["error"] = err.Error()
			longRangeFailInsertedBlockCounterVec.With(pl).Inc()

			lsi.p.RemoveStream(results[i].stid)
			lsi.gbm.HandleInsertError(results, i)
			return
		}

		lsi.inserted++
		longRangeSyncedBlockCounterVec.With(lsi.d.promLabels()).Inc()
	}
	lsi.gbm.HandleInsertResult(results)
}

func (lsi *lrSyncIter) checkHaveEnoughStreams() error {
	numStreams := lsi.p.NumStreams()
	if numStreams < lsi.config.MinStreams {
		return fmt.Errorf("number of streams smaller than minimum: %v < %v",
			numStreams, lsi.config.MinStreams)
	}
	return nil
}

// getBlocksWorker does the request job
type getBlocksWorker struct {
	gbm      *getBlocksManager
	protocol syncProtocol
}

func (w *getBlocksWorker) workLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		batch := w.gbm.GetNextBatch()
		if len(batch) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		blocks, stid, err := w.doBatch(ctx, batch)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				w.protocol.RemoveStream(stid)
			}
			err = errors.Wrap(err, "request error")
			w.gbm.HandleRequestError(batch, err, stid)
		} else {
			w.gbm.HandleRequestResult(batch, blocks, stid)
		}
	}
}

func (w *getBlocksWorker) doBatch(ctx context.Context, bns []uint64) ([]*types.Block, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocks, stid, err := w.protocol.GetBlocksByNumber(ctx, bns)
	if err != nil {
		return nil, stid, err
	}
	if err := validateGetBlocksResult(bns, blocks); err != nil {
		return nil, stid, err
	}
	return blocks, stid, nil
}

// getBlocksManager is the helper structure for get blocks request management
type getBlocksManager struct {
	chain blockChain

	targetBN   uint64
	requesting map[uint64]struct{} // block numbers that have been assigned to workers but not received
	processing map[uint64]struct{} // block numbers received requests but not inserted
	retries    *prioritizedNumbers // requests where error happens
	rq         *resultQueue        // result queue wait to be inserted into blockchain

	resultC chan struct{}
	logger  zerolog.Logger
	lock    sync.Mutex
}

func newGetBlocksManager(chain blockChain, targetBN uint64, logger zerolog.Logger) *getBlocksManager {
	return &getBlocksManager{
		chain:      chain,
		targetBN:   targetBN,
		requesting: make(map[uint64]struct{}),
		processing: make(map[uint64]struct{}),
		retries:    newPrioritizedNumbers(),
		rq:         newResultQueue(),
		resultC:    make(chan struct{}, 1),
		logger:     logger,
	}
}

// GetNextBatch get the next block numbers batch
func (gbm *getBlocksManager) GetNextBatch() []uint64 {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	cap := numBlocksByNumPerRequest

	bns := gbm.getBatchFromRetries(cap)
	cap -= len(bns)
	gbm.addBatchToRequesting(bns)

	if gbm.availableForMoreTasks() {
		addBNs := gbm.getBatchFromUnprocessed(cap)
		gbm.addBatchToRequesting(addBNs)
		bns = append(bns, addBNs...)
	}

	return bns
}

// HandleRequestError handles the error result
func (gbm *getBlocksManager) HandleRequestError(bns []uint64, err error, stid sttypes.StreamID) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	gbm.logger.Warn().Err(err).Str("stream", string(stid)).Msg("get blocks error")

	// add requested block numbers to retries
	for _, bn := range bns {
		delete(gbm.requesting, bn)
		gbm.retries.push(bn)
	}

	// remove results from result queue by the stream and add back to retries
	removed := gbm.rq.removeResultsByStreamID(stid)
	for _, bn := range removed {
		delete(gbm.processing, bn)
		gbm.retries.push(bn)
	}
}

// HandleRequestResult handles get blocks result
func (gbm *getBlocksManager) HandleRequestResult(bns []uint64, blocks []*types.Block, stid sttypes.StreamID) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	for i, bn := range bns {
		delete(gbm.requesting, bn)
		if blocks[i] == nil {
			gbm.retries.push(bn)
		} else {
			gbm.processing[bn] = struct{}{}
		}
	}
	gbm.rq.addBlockResults(blocks, stid)
	select {
	case gbm.resultC <- struct{}{}:
	default:
	}
}

// HandleInsertResult handle the insert result
func (gbm *getBlocksManager) HandleInsertResult(inserted []*blockResult) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	for _, block := range inserted {
		delete(gbm.processing, block.getBlockNumber())
	}
}

// HandleInsertError handles the error during InsertChain
func (gbm *getBlocksManager) HandleInsertError(results []*blockResult, n int) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	var (
		inserted  []*blockResult
		errResult *blockResult
		abandoned []*blockResult
	)
	inserted = results[:n]
	errResult = results[n]
	if n != len(results) {
		abandoned = results[n+1:]
	}

	for _, res := range inserted {
		delete(gbm.processing, res.getBlockNumber())
	}
	for _, res := range abandoned {
		gbm.rq.addBlockResults([]*types.Block{res.block}, res.stid)
	}

	delete(gbm.processing, errResult.getBlockNumber())
	gbm.retries.push(errResult.getBlockNumber())

	removed := gbm.rq.removeResultsByStreamID(errResult.stid)
	for _, bn := range removed {
		delete(gbm.processing, bn)
		gbm.retries.push(bn)
	}
}

// PullContinuousBlocks pull continuous blocks from request queue
func (gbm *getBlocksManager) PullContinuousBlocks(cap int) []*blockResult {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	expHeight := gbm.chain.CurrentBlock().NumberU64() + 1
	results, stales := gbm.rq.popBlockResults(expHeight, cap)
	// For stale blocks, we remove them from processing
	for _, bn := range stales {
		delete(gbm.processing, bn)
	}
	return results
}

// getBatchFromRetries get the block number batch to be requested from retries.
func (gbm *getBlocksManager) getBatchFromRetries(cap int) []uint64 {
	var (
		requestBNs []uint64
		curHeight  = gbm.chain.CurrentBlock().NumberU64()
	)
	for cnt := 0; cnt < cap; cnt++ {
		bn := gbm.retries.pop()
		if bn == 0 {
			break // no more retries
		}
		if bn <= curHeight {
			continue
		}
		requestBNs = append(requestBNs, bn)
	}
	return requestBNs
}

// getBatchFromRetries get the block number batch to be requested from unprocessed.
func (gbm *getBlocksManager) getBatchFromUnprocessed(cap int) []uint64 {
	var (
		requestBNs []uint64
		curHeight  = gbm.chain.CurrentBlock().NumberU64()
	)
	bn := curHeight + 1
	// TODO: this algorithm can be potentially optimized.
	for cnt := 0; cnt < cap && bn <= gbm.targetBN; cnt++ {
		for bn <= gbm.targetBN {
			_, ok1 := gbm.requesting[bn]
			_, ok2 := gbm.processing[bn]
			if !ok1 && !ok2 {
				requestBNs = append(requestBNs, bn)
				bn++
				break
			}
			bn++
		}
	}
	return requestBNs
}

func (gbm *getBlocksManager) availableForMoreTasks() bool {
	return gbm.rq.results.Len() < softQueueCap
}

func (gbm *getBlocksManager) addBatchToRequesting(bns []uint64) {
	for _, bn := range bns {
		gbm.requesting[bn] = struct{}{}
	}
}

func validateGetBlocksResult(requested []uint64, result []*types.Block) error {
	if len(result) != len(requested) {
		return fmt.Errorf("unexpected number of blocks delivered: %v / %v", len(result), len(requested))
	}
	for i, block := range result {
		if block != nil && block.NumberU64() != requested[i] {
			return fmt.Errorf("block with unexpected number delivered: %v / %v", block.NumberU64(), requested[i])
		}
	}
	return nil
}

// computeBlockNumberByMaxVote compute the target block number by max vote.
func computeBlockNumberByMaxVote(votes map[sttypes.StreamID]uint64) uint64 {
	var (
		nm     = make(map[uint64]int)
		res    uint64
		maxCnt int
	)
	for _, bn := range votes {
		_, ok := nm[bn]
		if !ok {
			nm[bn] = 0
		}
		nm[bn]++
		cnt := nm[bn]

		if cnt > maxCnt || (cnt == maxCnt && bn > res) {
			res = bn
			maxCnt = cnt
		}
	}
	return res
}
