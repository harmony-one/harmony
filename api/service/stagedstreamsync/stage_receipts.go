package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageReceipts struct {
	configs StageReceiptsCfg
}

type StageReceiptsCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	blockDBs    []kv.RwDB
	concurrency int
	protocol    syncProtocol
	isBeacon    bool
	logProgress bool
}

func NewStageReceipts(cfg StageReceiptsCfg) *StageReceipts {
	return &StageReceipts{
		configs: cfg,
	}
}

func NewStageReceiptsCfg(bc core.BlockChain, db kv.RwDB, blockDBs []kv.RwDB, concurrency int, protocol syncProtocol, isBeacon bool, logProgress bool) StageReceiptsCfg {
	return StageReceiptsCfg{
		bc:          bc,
		db:          db,
		blockDBs:    blockDBs,
		concurrency: concurrency,
		protocol:    protocol,
		isBeacon:    isBeacon,
		logProgress: logProgress,
	}
}

// Exec progresses receipts stage in the forward direction
func (r *StageReceipts) Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	// only execute this stage in fast/snap sync mode
	if s.state.status.cycleSyncMode == FullSync {
		return nil
	}

	// shouldn't execute for epoch chain
	if r.configs.bc.ShardID() == shard.BeaconChainShardID && !s.state.isBeaconNode {
		return nil
	}

	useInternalTx := tx == nil

	if invalidBlockRevert {
		return nil
	}

	// for short range sync, skip this stage
	if !s.state.initSync {
		return nil
	}

	maxHeight := s.state.status.targetBN
	currentHead := s.state.CurrentBlockNumber()
	if currentHead >= maxHeight {
		return nil
	}
	currProgress := uint64(0)
	targetHeight := s.state.currentCycle.TargetHeight

	if errV := CreateView(ctx, r.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress == 0 {
		currProgress = currentHead
	}

	if currProgress >= targetHeight {
		return nil
	}

	// size := uint64(0)
	startTime := time.Now()
	// startBlock := currProgress

	if r.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	if useInternalTx {
		var err error
		tx, err = r.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	for {
		// check if there is no any more to download break the loop
		curBn := s.state.CurrentBlockNumber()
		if curBn == targetHeight {
			break
		}

		// calculate the block numbers range to download
		toBn := curBn + uint64(ReceiptsPerRequest*s.state.config.Concurrency)
		if toBn > targetHeight {
			toBn = targetHeight
		}

		// Fetch receipts from connected peers
		rdm := newReceiptDownloadManager(tx, r.configs.bc, toBn, s.state.logger)

		// Setup workers to fetch blocks from remote node
		var wg sync.WaitGroup

		for i := 0; i < s.state.config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				// prepare db transactions
				txs := make([]kv.RwTx, r.configs.concurrency)
				for i := 0; i < r.configs.concurrency; i++ {
					txs[i], err = r.configs.blockDBs[i].BeginRw(ctx)
					if err != nil {
						return
					}
				}
				// rollback the transactions after worker loop
				defer func() {
					for i := 0; i < r.configs.concurrency; i++ {
						txs[i].Rollback()
					}
				}()

				r.runReceiptWorkerLoop(ctx, rdm, &wg, s, txs, startTime)
			}()
		}
		wg.Wait()
		// insert all downloaded blocks and receipts to chain
		if err := r.insertBlocksAndReceipts(ctx, rdm, toBn, s); err != nil {
			utils.Logger().Err(err).Msg(WrapStagedSyncMsg("InsertReceiptChain failed"))
		}
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func (r *StageReceipts) insertBlocksAndReceipts(ctx context.Context, rdm *receiptDownloadManager, toBn uint64, s *StageState) error {
	if len(rdm.received) == 0 {
		return nil
	}
	var (
		bns       []uint64
		blocks    []*types.Block
		receipts  []types.Receipts
		streamIDs []sttypes.StreamID
	)
	// populate blocks and receipts in separate array
	// this way helps to sort blocks and receipts by block number
	for bn := s.state.CurrentBlockNumber() + 1; bn <= toBn; bn++ {
		if received, ok := rdm.received[bn]; !ok {
			return errors.New("some blocks are missing")
		} else {
			bns = append(bns, bn)
			blocks = append(blocks, received.block)
			receipts = append(receipts, received.receipts)
			streamIDs = append(streamIDs, received.streamID)
		}
	}
	// insert sorted blocks and receipts to chain
	if inserted, err := r.configs.bc.InsertReceiptChain(blocks, receipts); err != nil {
		utils.Logger().Err(err).
			Interface("streams", streamIDs).
			Interface("block numbers", bns).
			Msg(WrapStagedSyncMsg("InsertReceiptChain failed"))
		rdm.HandleRequestError(bns, err)
		return fmt.Errorf("InsertReceiptChain failed: %s", err.Error())
	} else {
		if inserted != len(blocks) {
			utils.Logger().Warn().
				Interface("block numbers", bns).
				Int("inserted", inserted).
				Int("blocks to insert", len(blocks)).
				Msg(WrapStagedSyncMsg("InsertReceiptChain couldn't insert all downloaded blocks/receipts"))
		}
	}
	return nil
}

// runReceiptWorkerLoop creates a work loop for download receipts
func (r *StageReceipts) runReceiptWorkerLoop(ctx context.Context, rdm *receiptDownloadManager, wg *sync.WaitGroup, s *StageState, txs []kv.RwTx, startTime time.Time) {

	currentBlock := int(s.state.CurrentBlockNumber())
	gbm := s.state.gbm

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// get next batch of block numbers
		curHeight := s.state.CurrentBlockNumber()
		batch := rdm.GetNextBatch(curHeight)
		if len(batch) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				return
			}
		}
		// retrieve corresponding blocks from cache db
		var hashes []common.Hash
		var blocks []*types.Block

		for _, bn := range batch {
			blkKey := marshalData(bn)
			loopID, _, errBDD := gbm.GetDownloadDetails(bn)
			if errBDD != nil {
				utils.Logger().Warn().
					Err(errBDD).
					Interface("block numbers", bn).
					Msg(WrapStagedSyncMsg("get block download details failed"))
				return
			}
			blockBytes, err := txs[loopID].GetOne(BlocksBucket, blkKey)
			if err != nil {
				return
			}
			sigBytes, err := txs[loopID].GetOne(BlockSignaturesBucket, blkKey)
			if err != nil {
				return
			}
			sz := len(blockBytes)
			if sz <= 1 {
				return
			}
			var block *types.Block
			if err := rlp.DecodeBytes(blockBytes, &block); err != nil {
				return
			}
			if sigBytes != nil {
				block.SetCurrentCommitSig(sigBytes)
			}
			if block.NumberU64() != bn {
				return
			}
			if block.Header().ReceiptHash() == emptyHash {
				return
			}
			// receiptHash := s.state.currentCycle.ReceiptHashes[bn]
			gbm.SetRootHash(bn, block.Header().Root())
			hashes = append(hashes, block.Header().Hash())
			blocks = append(blocks, block)
		}

		// download receipts
		receipts, stid, err := r.downloadReceipts(ctx, hashes)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				r.configs.protocol.StreamFailed(stid, "downloadRawBlocks failed")
			}
			utils.Logger().Error().
				Err(err).
				Str("stream", string(stid)).
				Interface("block numbers", batch).
				Msg(WrapStagedSyncMsg("downloadRawBlocks failed"))
			err = errors.Wrap(err, "request error")
			rdm.HandleRequestError(batch, err)
		} else {
			// handle request result
			rdm.HandleRequestResult(batch, receipts, blocks, stid)
			// log progress
			if r.configs.logProgress {
				//calculating block download speed
				dt := time.Now().Sub(startTime).Seconds()
				speed := float64(0)
				if dt > 0 {
					speed = float64(len(rdm.rdd)) / dt
				}
				blockReceiptSpeed := fmt.Sprintf("%.2f", speed)

				fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
				fmt.Println("downloaded blocks and receipts:", currentBlock+len(rdm.rdd), "/", int(rdm.targetBN), "(", blockReceiptSpeed, "BlocksAndReceipts/s", ")")
			}
		}
	}
}

func (r *StageReceipts) downloadReceipts(ctx context.Context, hs []common.Hash) ([]types.Receipts, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	receipts, stid, err := r.configs.protocol.GetReceipts(ctx, hs)
	if err != nil {
		return nil, stid, err
	}
	if err := validateGetReceiptsResult(hs, receipts); err != nil {
		return nil, stid, err
	}
	return receipts, stid, nil
}

func validateGetReceiptsResult(requested []common.Hash, result []types.Receipts) error {
	// TODO: validate each receipt here

	return nil
}

func (r *StageReceipts) saveProgress(ctx context.Context, s *StageState, progress uint64, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = r.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, progress); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for receipt stage failed")
		return ErrSavingBodiesProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *StageReceipts) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = r.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return err
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *StageReceipts) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = r.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if useInternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
