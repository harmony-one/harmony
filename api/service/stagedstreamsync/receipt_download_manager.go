package stagedstreamsync

import (
	"sync"

	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
)

type ReceiptDownloadDetails struct {
	loopID   int
	streamID sttypes.StreamID
}

// receiptDownloadManager is the helper structure for get receipts request management
type receiptDownloadManager struct {
	chain blockChain
	tx    kv.RwTx

	targetBN   uint64
	requesting map[uint64]struct{}               // receipt numbers that have been assigned to workers but not received
	processing map[uint64]struct{}               // receipt numbers received requests but not inserted
	retries    *prioritizedNumbers               // requests where error happens
	rdd        map[uint64]ReceiptDownloadDetails // details about how this receipt was downloaded

	logger zerolog.Logger
	lock   sync.Mutex
}

func newReceiptDownloadManager(tx kv.RwTx, chain blockChain, targetBN uint64, logger zerolog.Logger) *receiptDownloadManager {
	return &receiptDownloadManager{
		chain:      chain,
		tx:         tx,
		targetBN:   targetBN,
		requesting: make(map[uint64]struct{}),
		processing: make(map[uint64]struct{}),
		retries:    newPrioritizedNumbers(),
		rdd:        make(map[uint64]ReceiptDownloadDetails),
		logger:     logger,
	}
}

// GetNextBatch get the next receipt numbers batch
func (rdm *receiptDownloadManager) GetNextBatch() []uint64 {
	rdm.lock.Lock()
	defer rdm.lock.Unlock()

	cap := ReceiptsPerRequest

	bns := rdm.getBatchFromRetries(cap)
	if len(bns) > 0 {
		cap -= len(bns)
		rdm.addBatchToRequesting(bns)
	}

	if rdm.availableForMoreTasks() {
		addBNs := rdm.getBatchFromUnprocessed(cap)
		rdm.addBatchToRequesting(addBNs)
		bns = append(bns, addBNs...)
	}

	return bns
}

// HandleRequestError handles the error result
func (rdm *receiptDownloadManager) HandleRequestError(bns []uint64, err error, streamID sttypes.StreamID) {
	rdm.lock.Lock()
	defer rdm.lock.Unlock()

	// add requested receipt numbers to retries
	for _, bn := range bns {
		delete(rdm.requesting, bn)
		rdm.retries.push(bn)
	}
}

// HandleRequestResult handles get receipts result
func (rdm *receiptDownloadManager) HandleRequestResult(bns []uint64, receipts []types.Receipts, loopID int, streamID sttypes.StreamID) error {
	rdm.lock.Lock()
	defer rdm.lock.Unlock()

	for i, bn := range bns {
		delete(rdm.requesting, bn)
		if indexExists(receipts, i) {
			rdm.retries.push(bn)
		} else {
			rdm.processing[bn] = struct{}{}
			rdm.rdd[bn] = ReceiptDownloadDetails{
				loopID:   loopID,
				streamID: streamID,
			}
		}
	}
	return nil
}

// SetDownloadDetails sets the download details for a batch of blocks
func (rdm *receiptDownloadManager) SetDownloadDetails(bns []uint64, loopID int, streamID sttypes.StreamID) error {
	rdm.lock.Lock()
	defer rdm.lock.Unlock()

	for _, bn := range bns {
		rdm.rdd[bn] = ReceiptDownloadDetails{
			loopID:   loopID,
			streamID: streamID,
		}
	}
	return nil
}

// GetDownloadDetails returns the download details for a certain block number
func (rdm *receiptDownloadManager) GetDownloadDetails(blockNumber uint64) (loopID int, streamID sttypes.StreamID) {
	rdm.lock.Lock()
	defer rdm.lock.Unlock()

	return rdm.rdd[blockNumber].loopID, rdm.rdd[blockNumber].streamID
}

// getBatchFromRetries get the receipt number batch to be requested from retries.
func (rdm *receiptDownloadManager) getBatchFromRetries(cap int) []uint64 {
	var (
		requestBNs []uint64
		curHeight  = rdm.chain.CurrentBlock().NumberU64()
	)
	for cnt := 0; cnt < cap; cnt++ {
		bn := rdm.retries.pop()
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

// getBatchFromUnprocessed returns a batch of receipt numbers to be requested from unprocessed.
func (rdm *receiptDownloadManager) getBatchFromUnprocessed(cap int) []uint64 {
	var (
		requestBNs []uint64
		curHeight  = rdm.chain.CurrentBlock().NumberU64()
	)
	bn := curHeight + 1
	// TODO: this algorithm can be potentially optimized.
	for cnt := 0; cnt < cap && bn <= rdm.targetBN; cnt++ {
		for bn <= rdm.targetBN {
			_, ok1 := rdm.requesting[bn]
			_, ok2 := rdm.processing[bn]
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

func (rdm *receiptDownloadManager) availableForMoreTasks() bool {
	return len(rdm.requesting) < SoftQueueCap
}

func (rdm *receiptDownloadManager) addBatchToRequesting(bns []uint64) {
	for _, bn := range bns {
		rdm.requesting[bn] = struct{}{}
	}
}
