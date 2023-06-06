package stagedstreamsync

import (
	"sync"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
)

type BlockDownloadDetails struct {
	loopID   int
	streamID sttypes.StreamID
}

// blockDownloadManager is the helper structure for get blocks request management
type blockDownloadManager struct {
	chain blockChain
	tx    kv.RwTx

	targetBN   uint64
	requesting map[uint64]struct{}             // block numbers that have been assigned to workers but not received
	processing map[uint64]struct{}             // block numbers received requests but not inserted
	retries    *prioritizedNumbers             // requests where error happens
	rq         *resultQueue                    // result queue wait to be inserted into blockchain
	bdd        map[uint64]BlockDownloadDetails // details about how this block was downloaded

	logger zerolog.Logger
	lock   sync.Mutex
}

func newBlockDownloadManager(tx kv.RwTx, chain blockChain, targetBN uint64, logger zerolog.Logger) *blockDownloadManager {
	return &blockDownloadManager{
		chain:      chain,
		tx:         tx,
		targetBN:   targetBN,
		requesting: make(map[uint64]struct{}),
		processing: make(map[uint64]struct{}),
		retries:    newPrioritizedNumbers(),
		rq:         newResultQueue(),
		bdd:        make(map[uint64]BlockDownloadDetails),
		logger:     logger,
	}
}

// GetNextBatch get the next block numbers batch
func (gbm *blockDownloadManager) GetNextBatch() []uint64 {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	cap := BlocksPerRequest

	bns := gbm.getBatchFromRetries(cap)
	if len(bns) > 0 {
		cap -= len(bns)
		gbm.addBatchToRequesting(bns)
	}

	if gbm.availableForMoreTasks() {
		addBNs := gbm.getBatchFromUnprocessed(cap)
		gbm.addBatchToRequesting(addBNs)
		bns = append(bns, addBNs...)
	}

	return bns
}

// HandleRequestError handles the error result
func (gbm *blockDownloadManager) HandleRequestError(bns []uint64, err error, streamID sttypes.StreamID) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	// add requested block numbers to retries
	for _, bn := range bns {
		delete(gbm.requesting, bn)
		gbm.retries.push(bn)
	}
}

// HandleRequestResult handles get blocks result
func (gbm *blockDownloadManager) HandleRequestResult(bns []uint64, blockBytes [][]byte, sigBytes [][]byte, loopID int, streamID sttypes.StreamID) error {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	for i, bn := range bns {
		delete(gbm.requesting, bn)
		if indexExists(blockBytes, i) && len(blockBytes[i]) <= 1 {
			gbm.retries.push(bn)
		} else {
			gbm.processing[bn] = struct{}{}
			gbm.bdd[bn] = BlockDownloadDetails{
				loopID:   loopID,
				streamID: streamID,
			}
		}
	}
	return nil
}

func indexExists[T any](slice []T, index int) bool {
	return index >= 0 && index < len(slice)
}

// SetDownloadDetails sets the download details for a batch of blocks
func (gbm *blockDownloadManager) SetDownloadDetails(bns []uint64, loopID int, streamID sttypes.StreamID) error {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	for _, bn := range bns {
		gbm.bdd[bn] = BlockDownloadDetails{
			loopID:   loopID,
			streamID: streamID,
		}
	}
	return nil
}

// GetDownloadDetails returns the download details for a block
func (gbm *blockDownloadManager) GetDownloadDetails(blockNumber uint64) (loopID int, streamID sttypes.StreamID) {
	gbm.lock.Lock()
	defer gbm.lock.Unlock()

	return gbm.bdd[blockNumber].loopID, gbm.bdd[blockNumber].streamID
}

// getBatchFromRetries get the block number batch to be requested from retries.
func (gbm *blockDownloadManager) getBatchFromRetries(cap int) []uint64 {
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

// getBatchFromUnprocessed returns a batch of block numbers to be requested from unprocessed.
func (gbm *blockDownloadManager) getBatchFromUnprocessed(cap int) []uint64 {
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

func (gbm *blockDownloadManager) availableForMoreTasks() bool {
	return gbm.rq.results.Len() < SoftQueueCap
}

func (gbm *blockDownloadManager) addBatchToRequesting(bns []uint64) {
	for _, bn := range bns {
		gbm.requesting[bn] = struct{}{}
	}
}
