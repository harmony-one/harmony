package stagedstreamsync

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/rs/zerolog"
)

type DownloadDetails struct {
	loopID   int
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
	logger     zerolog.Logger
	lock       sync.Mutex
}

func newDownloadManager(chain blockChain, targetBN uint64, batchSize int, logger zerolog.Logger) *downloadManager {
	return &downloadManager{
		chain:      chain,
		targetBN:   targetBN,
		requesting: make(map[uint64]struct{}),
		processing: make(map[uint64]struct{}),
		retries:    newPrioritizedNumbers(),
		rq:         newResultQueue(),
		details:    make(map[uint64]*DownloadDetails),
		batchSize:  batchSize,
		logger: logger.With().
			Str("sub-module", "download manager").
			Logger(),
	}
}

// GetNextBatch get the next block numbers batch
func (dm *downloadManager) GetNextBatch(curHeight uint64) []uint64 {
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
		addBNs := dm.getBatchFromUnprocessed(cap, curHeight)
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
func (dm *downloadManager) HandleRequestResult(bns []uint64, blockBytes [][]byte, sigBytes [][]byte, loopID int, streamID sttypes.StreamID) error {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for i, bn := range bns {
		delete(dm.requesting, bn)
		if indexExists(blockBytes, i) && len(blockBytes[i]) <= 1 {
			dm.retries.push(bn)
		} else {
			dm.processing[bn] = struct{}{}
			dm.details[bn] = &DownloadDetails{
				loopID:   loopID,
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
func (dm *downloadManager) SetDownloadDetails(bns []uint64, loopID int, streamID sttypes.StreamID) error {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	for _, bn := range bns {
		dm.details[bn] = &DownloadDetails{
			loopID:   loopID,
			streamID: streamID,
		}
	}
	return nil
}

// GetDownloadDetails returns the download details for a block
func (dm *downloadManager) GetDownloadDetails(blockNumber uint64) (loopID int, streamID sttypes.StreamID, err error) {
	dm.lock.Lock()
	defer dm.lock.Unlock()

	if dm, exist := dm.details[blockNumber]; exist {
		return dm.loopID, dm.streamID, nil
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
func (dm *downloadManager) getBatchFromUnprocessed(cap int, curHeight uint64) []uint64 {
	var (
		requestBNs []uint64
	)
	bn := curHeight + 1
	// TODO: this algorithm can be potentially optimized.
	for cnt := 0; cnt < cap && bn <= dm.targetBN; cnt++ {
		for bn <= dm.targetBN {
			_, ok1 := dm.requesting[bn]
			_, ok2 := dm.processing[bn]
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

func (dm *downloadManager) availableForMoreTasks() bool {
	return dm.rq.results.Len() < SoftQueueCap
}

func (dm *downloadManager) addBatchToRequesting(bns []uint64) {
	for _, bn := range bns {
		dm.requesting[bn] = struct{}{}
	}
}
