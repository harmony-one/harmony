package stagedstreamsync

import (
	"errors"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/rs/zerolog"
)

// lastMileCache keeps the last 50 number blocks in memory cache
const lastMileCap = 50

type (
	// beaconHelper is the helper for the beacon downloader. The beaconHelper is only started
	// when node is running on side chain, listening to beacon client pub-sub message and
	// insert the latest blocks to the beacon chain.
	beaconHelper struct {
		bc     blockChain
		blockC <-chan *types.Block
		// TODO: refactor this hook to consensus module. We'd better put it in
		//   consensus module under a subscription.
		insertHook func()

		lastMileCache *blocksByNumber
		insertC       chan insertTask
		closeC        chan struct{}
		logger        zerolog.Logger

		lock sync.RWMutex
	}

	insertTask struct {
		doneC chan struct{}
	}
)

func newBeaconHelper(bc blockChain, logger zerolog.Logger, blockC <-chan *types.Block, insertHook func()) *beaconHelper {
	return &beaconHelper{
		bc:            bc,
		blockC:        blockC,
		insertHook:    insertHook,
		lastMileCache: newBlocksByNumber(lastMileCap),
		insertC:       make(chan insertTask, 1),
		closeC:        make(chan struct{}),
		logger: logger.With().
			Str("sub-module", "beacon helper").
			Logger(),
	}
}

func (bh *beaconHelper) start() {
	go bh.loop()
}

func (bh *beaconHelper) close() {
	bh.lock.Lock()
	defer bh.lock.Unlock()

	select {
	case <-bh.closeC:
		// Already closed, do nothing
	default:
		close(bh.closeC)
	}
}

func (bh *beaconHelper) loop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			bh.insertAsync()

		case b, ok := <-bh.blockC: // for side chain, it receives last block of each epoch
			if !ok {
				return // blockC closed. Node exited
			}
			if b == nil {
				continue
			}
			bh.lock.Lock()
			bh.lastMileCache.push(b)
			bh.lock.Unlock()
			bh.insertAsync()

		case it := <-bh.insertC:
			inserted, bn, err := bh.insertLastMileBlocks()
			if err != nil {
				bh.logger.Error().Err(err).
					Msg(WrapStagedSyncMsg("insert last mile blocks error"))
				close(it.doneC)
				continue
			}
			if inserted > 0 {
				numBlocksInsertedBeaconHelperCounter.Add(float64(inserted))
				bh.logger.Info().Int("inserted", inserted).
					Uint64("end height", bn).
					Uint32("shard", bh.bc.ShardID()).
					Msg(WrapStagedSyncMsg("insert last mile blocks"))
			}
			close(it.doneC)

		case <-bh.closeC:
			return
		}
	}
}

// insertAsync triggers the insert last mile without blocking
func (bh *beaconHelper) insertAsync() {
	select {
	case <-bh.closeC:
		// Do nothing if closed
		return
	case bh.insertC <- insertTask{
		doneC: make(chan struct{}),
	}:
	default:
	}
}

// insertSync triggers the insert last mile while blocking
func (bh *beaconHelper) insertSync() {
	task := insertTask{
		doneC: make(chan struct{}),
	}
	select {
	case <-bh.closeC:
		// Do nothing if closed
		return
	case bh.insertC <- task:
	}
	<-task.doneC
}

func (bh *beaconHelper) insertLastMileBlocks() (inserted int, bn uint64, err error) {
	bh.lock.Lock()
	defer bh.lock.Unlock()

	bn = bh.bc.CurrentBlock().NumberU64() + 1
	for {
		b := bh.getNextBlock(bn)
		if b == nil {
			bn--
			return
		}
		// TODO: Instruct the beacon helper to verify signatures. This may require some forks
		//       in pub-sub message (add commit sigs in node.block.sync messages)
		_, err = bh.bc.InsertChain(types.Blocks{b}, true)
		if err != nil && !errors.Is(err, core.ErrKnownBlock) {
			bn--
			return
		}
		bh.logger.Info().
			Uint64("number", b.NumberU64()).
			Msg(WrapStagedSyncMsg("Inserted block from beacon pub-sub"))

		if bh.insertHook != nil {
			bh.insertHook()
		}
		inserted++
		bn++
	}
}

func (bh *beaconHelper) getNextBlock(expBN uint64) *types.Block {
	for bh.lastMileCache.len() > 0 {
		b := bh.lastMileCache.pop()
		if b == nil {
			return nil
		}
		if b.NumberU64() < expBN {
			continue
		}
		return b
	}
	return nil
}
