package stagedstreamsync

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/rs/zerolog"
)

type DownloadDetails struct {
	workerID int
	streamID sttypes.StreamID
	hash     common.Hash
}

// downloadManager is the helper structure for get blocks request management
type downloadManager struct {
	chain      blockChain
	targetBN   uint64
	requesting map[uint64]struct{}         // block numbers that have been assigned to workers but not received
	processing map[uint64]struct{}         // block numbers received requests but not inserted
	retries    *prioritizedNumbers         // requests where error happens
	rq         *resultQueue                // result queue wait to be inserted into blockchain
	details    map[uint64]*DownloadDetails // details about how this block was downloaded
	batchSize  int
	curNumber  uint64
	logger     zerolog.Logger
	lock       sync.Mutex
}

func newDownloadManager(chain blockChain, currHeight uint64, targetBN uint64, batchSize int, logger zerolog.Logger) *downloadManager {
	return &downloadManager{
		chain:      chain,
		targetBN:   targetBN,
		requesting: make(map[uint64]struct{}),
		processing: make(map[uint64]struct{}),
		retries:    newPrioritizedNumbers(),
		rq:         newResultQueue(),
		details:    make(map[uint64]*DownloadDetails),
		batchSize:  batchSize,
		curNumber:  currHeight,
		logger: logger.With().
			Str("sub-module", "download manager").
			Logger(),
	}
}

// GetNextBatch get the next block numbers batch
func (dm *downloadManager) GetNextBatch() []uint64 {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	cap := dm.batchSize

	bns := dm.getBatchFromRetries(cap)
	if len(bns) > 0 {
		cap -= len(bns)
		dm.addBatchToRequesting(bns)
	}

	if cap <= 0 {
		return bns
	}

	if dm.availableForMoreTasks() {
		addBNs := dm.getBatchFromUnprocessed(cap)
		dm.addBatchToRequesting(addBNs)
		bns = append(bns, addBNs...)
	}

	return bns
}

// HandleRequestError handles the error result
func (dm *downloadManager) HandleRequestError(bns []uint64, err error, streamID sttypes.StreamID) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	// add requested block numbers to retries
	for _, bn := range bns {
		delete(dm.requesting, bn)
		dm.retries.push(bn)
	}
}

// HandleRequestResult handles get blocks result
func (dm *downloadManager) HandleRequestResult(bns []uint64, blockBytes [][]byte, sigBytes [][]byte, workerID int, streamID sttypes.StreamID) error {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for i, bn := range bns {
		delete(dm.requesting, bn)
		if indexExists(blockBytes, i) && len(blockBytes[i]) <= 1 {
			dm.retries.push(bn)
		} else {
			dm.processing[bn] = struct{}{}
			dm.details[bn] = &DownloadDetails{
				workerID: workerID,
				streamID: streamID,
			}
		}
	}
	return nil
}

// HandleRequestResult handles get blocks result
func (dm *downloadManager) HandleHashesRequestResult(bns []uint64) error {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for _, bn := range bns {
		delete(dm.requesting, bn)
		dm.processing[bn] = struct{}{}
		dm.details[bn] = &DownloadDetails{}
	}
	return nil
}

func indexExists[T any](slice []T, index int) bool {
	return index >= 0 && index < len(slice)
}

// SetDownloadDetails sets the download details for a batch of blocks
func (dm *downloadManager) SetDownloadDetails(bns []uint64, workerID int, streamID sttypes.StreamID) error {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for _, bn := range bns {
		// Clean up any existing details for this block to prevent duplicates
		delete(dm.details, bn)
		dm.details[bn] = &DownloadDetails{
			workerID: workerID,
			streamID: streamID,
		}
	}
	return nil
}

// GetDownloadDetails returns the download details for a block
func (dm *downloadManager) GetDownloadDetails(blockNumber uint64) (workerID int, streamID sttypes.StreamID, err error) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	if dm, exist := dm.details[blockNumber]; exist {
		return dm.workerID, dm.streamID, nil
	}
	return 0, sttypes.StreamID(fmt.Sprint(0)), fmt.Errorf("there is no download details for the block number: %d", blockNumber)
}

// SetRootHash sets the root hash for a specific block
func (dm *downloadManager) SetRootHash(blockNumber uint64, root common.Hash) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	if _, exist := dm.details[blockNumber]; !exist {
		return
	}
	dm.details[blockNumber].hash = root
}

// GetRootHash returns the root hash for a specific block
func (dm *downloadManager) GetRootHash(blockNumber uint64) common.Hash {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	if _, exist := dm.details[blockNumber]; !exist {
		return common.Hash{}
	}

	return dm.details[blockNumber].hash
}

// getBatchFromRetries get the block number batch to be requested from retries.
func (dm *downloadManager) getBatchFromRetries(cap int) []uint64 {
	var (
		requestBNs []uint64
	)
	for cnt := 0; cnt < cap; cnt++ {
		bn := dm.retries.pop()
		if bn == 0 {
			break // no more retries
		}
		requestBNs = append(requestBNs, bn)
	}
	return requestBNs
}

// getBatchFromUnprocessed returns a batch of block numbers to be requested from unprocessed.
func (dm *downloadManager) getBatchFromUnprocessed(cap int) []uint64 {
	var (
		requestBNs []uint64
	)
	bn := dm.curNumber + 1
	// TODO: this algorithm can be potentially optimized.
	for cnt := 0; cnt < cap && bn <= dm.targetBN; cnt++ {
		for bn <= dm.targetBN {
			dm.curNumber = bn
			_, ok1 := dm.requesting[bn]
			_, ok2 := dm.processing[bn]
			if !ok1 && !ok2 {
				requestBNs = append(requestBNs, bn)
				bn++
				break
			}
			// skip and go next block number
			bn++
		}
	}
	return requestBNs
}

func (dm *downloadManager) availableForMoreTasks() bool {
	return dm.rq.results.Len() < SoftQueueCap
}

func (dm *downloadManager) addBatchToRequesting(bns []uint64) {
	for _, bn := range bns {
		dm.requesting[bn] = struct{}{}
	}
}

// CleanupDetails removes download details for a specific block number
func (dm *downloadManager) CleanupDetails(blockNumber uint64) {
	dm.lock.Lock()
	defer dm.lock.Unlock()
	delete(dm.details, blockNumber)
}

// CleanupAllDetails removes all download details to prevent memory leaks
func (dm *downloadManager) CleanupAllDetails() {
	dm.lock.Lock()
	defer dm.lock.Unlock()
	dm.details = make(map[uint64]*DownloadDetails)
}

// CleanupProcessedDetails removes download details for blocks that have been processed
func (dm *downloadManager) CleanupProcessedDetails() {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	// Remove details for blocks that are no longer in processing
	for bn := range dm.details {
		if _, stillProcessing := dm.processing[bn]; !stillProcessing {
			delete(dm.details, bn)
		}
	}
}

// CleanupOldDetails removes download details for blocks that have been in processing for too long
// This helps prevent memory leaks from blocks that get stuck in processing
func (dm *downloadManager) CleanupOldDetails(maxAge time.Duration) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for bn := range dm.details {
		// If the block is still in processing, we can't clean it up yet
		if _, stillProcessing := dm.processing[bn]; stillProcessing {
			continue
		}
		// For now, we'll clean up all details that are not in processing
		// In a more sophisticated implementation, we could add timestamps to DownloadDetails
		delete(dm.details, bn)
	}
}

// GetDetailsSize returns the current size of the details map for monitoring
func (dm *downloadManager) GetDetailsSize() int {
	dm.lock.Lock()
	defer dm.lock.Unlock()
	return len(dm.details)
}

// RemoveFromProcessing removes a block from processing and cleans up its details
func (dm *downloadManager) RemoveFromProcessing(blockNumber uint64) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	delete(dm.processing, blockNumber)
	delete(dm.details, blockNumber)
}

// RemoveBatchFromProcessing removes multiple blocks from processing and cleans up their details
func (dm *downloadManager) RemoveBatchFromProcessing(blockNumbers []uint64) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for _, bn := range blockNumbers {
		delete(dm.processing, bn)
		delete(dm.details, bn)
	}
}

// MarkBlockCompleted marks a block as completed and safe to clean up
// This should be called after all stages have processed the block
func (dm *downloadManager) MarkBlockCompleted(blockNumber uint64) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	// Remove from processing and clean up details
	delete(dm.processing, blockNumber)
	delete(dm.details, blockNumber)
}

// MarkBatchCompleted marks multiple blocks as completed and safe to clean up
func (dm *downloadManager) MarkBatchCompleted(blockNumbers []uint64) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for _, bn := range blockNumbers {
		delete(dm.processing, bn)
		delete(dm.details, bn)
	}
}

// CleanupCompletedBlocks removes details for blocks that are no longer in processing
// This is a safer cleanup that only removes details for blocks that have been fully processed
func (dm *downloadManager) CleanupCompletedBlocks() {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	// Only clean up details for blocks that are no longer in processing
	for bn := range dm.details {
		if _, stillProcessing := dm.processing[bn]; !stillProcessing {
			delete(dm.details, bn)
		}
	}
}
