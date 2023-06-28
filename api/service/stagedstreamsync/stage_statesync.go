package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type StageStateSync struct {
	configs StageStateSyncCfg
}

type StageStateSyncCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	concurrency int
	protocol    syncProtocol
	logger      zerolog.Logger
	logProgress bool
}

func NewStageStateSync(cfg StageStateSyncCfg) *StageStateSync {
	return &StageStateSync{
		configs: cfg,
	}
}

func NewStageStateSyncCfg(bc core.BlockChain,
	db kv.RwDB,
	concurrency int,
	protocol syncProtocol,
	logger zerolog.Logger,
	logProgress bool) StageStateSyncCfg {

	return StageStateSyncCfg{
		bc:          bc,
		db:          db,
		concurrency: concurrency,
		protocol:    protocol,
		logger:      logger,
		logProgress: logProgress,
	}
}

// Exec progresses States stage in the forward direction
func (sss *StageStateSync) Exec(ctx context.Context, bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	// for short range sync, skip this step
	if !s.state.initSync {
		return nil
	}

	maxHeight := s.state.status.targetBN
	currentHead := sss.configs.bc.CurrentBlock().NumberU64()
	if currentHead >= maxHeight {
		return nil
	}
	currProgress := sss.configs.bc.CurrentBlock().NumberU64()
	targetHeight := s.state.currentCycle.TargetHeight

	if errV := CreateView(ctx, sss.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress >= targetHeight {
		return nil
	}
	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = sss.configs.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// isLastCycle := targetHeight >= maxHeight
	startTime := time.Now()

	if sss.configs.logProgress {
		fmt.Print("\033[s") // save the cursor position
	}

	// Fetch blocks from neighbors
	root := sss.configs.bc.CurrentBlock().Root()
	sdm := newStateDownloadManager(tx, sss.configs.bc, root, sss.configs.concurrency, s.state.logger)

	// Setup workers to fetch blocks from remote node
	var wg sync.WaitGroup

	for i := 0; i != s.state.config.Concurrency; i++ {
		wg.Add(1)
		go sss.runStateWorkerLoop(ctx, sdm, &wg, i, startTime)
	}

	wg.Wait()

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// runStateWorkerLoop creates a work loop for download states
func (sss *StageStateSync) runStateWorkerLoop(ctx context.Context, sdm *StateDownloadManager, wg *sync.WaitGroup, loopID int, startTime time.Time) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		nodes, paths, codes := sdm.GetNextBatch()
		if len(nodes)+len(codes) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				return
			}
		}

		data, stid, err := sss.downloadStates(ctx, nodes, codes)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				sss.configs.protocol.StreamFailed(stid, "downloadStates failed")
			}
			utils.Logger().Error().
				Err(err).
				Str("stream", string(stid)).
				Msg(WrapStagedSyncMsg("downloadStates failed"))
			err = errors.Wrap(err, "request error")
			sdm.HandleRequestError(codes, paths, stid, err)
		} else if data == nil || len(data) == 0 {
			utils.Logger().Warn().
				Str("stream", string(stid)).
				Msg(WrapStagedSyncMsg("downloadStates failed, received empty data bytes"))
			err := errors.New("downloadStates received empty data bytes")
			sdm.HandleRequestError(codes, paths, stid, err)
		}
		sdm.HandleRequestResult(nodes, paths, data, loopID, stid)
		if sss.configs.logProgress {
			//calculating block download speed
			dt := time.Now().Sub(startTime).Seconds()
			speed := float64(0)
			if dt > 0 {
				speed = float64(len(data)) / dt
			}
			stateDownloadSpeed := fmt.Sprintf("%.2f", speed)

			fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
			fmt.Println("state download speed:", stateDownloadSpeed, "states/s")
		}
	}
}

func (sss *StageStateSync) downloadStates(ctx context.Context, nodes []common.Hash, codes []common.Hash) ([][]byte, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	hashes := append(codes, nodes...)
	data, stid, err := sss.configs.protocol.GetNodeData(ctx, hashes)
	if err != nil {
		return nil, stid, err
	}
	if err := validateGetNodeDataResult(hashes, data); err != nil {
		return nil, stid, err
	}
	return data, stid, nil
}

func validateGetNodeDataResult(requested []common.Hash, result [][]byte) error {
	if len(result) != len(requested) {
		return fmt.Errorf("unexpected number of nodes delivered: %v / %v", len(result), len(requested))
	}
	return nil
}

func (stg *StageStateSync) insertChain(gbm *blockDownloadManager,
	protocol syncProtocol,
	lbls prometheus.Labels,
	targetBN uint64) {

}

func (stg *StageStateSync) saveProgress(s *StageState, tx kv.RwTx) (err error) {

	useInternalTx := tx == nil
	if useInternalTx {
		var err error
		tx, err = stg.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, stg.configs.bc.CurrentBlock().NumberU64()); err != nil {
		utils.Logger().Error().
			Err(err).
			Msgf("[STAGED_SYNC] saving progress for block States stage failed")
		return ErrSaveStateProgressFail
	}

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (stg *StageStateSync) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = stg.configs.db.BeginRw(ctx)
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

func (stg *StageStateSync) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
	useInternalTx := tx == nil
	if useInternalTx {
		tx, err = stg.configs.db.BeginRw(ctx)
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
