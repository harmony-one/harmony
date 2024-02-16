package stagedstreamsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"

	//sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type StageFullStateSync struct {
	configs StageFullStateSyncCfg
}

type StageFullStateSyncCfg struct {
	bc          core.BlockChain
	db          kv.RwDB
	concurrency int
	protocol    syncProtocol
	logger      zerolog.Logger
	logProgress bool
}

func NewStageFullStateSync(cfg StageFullStateSyncCfg) *StageFullStateSync {
	return &StageFullStateSync{
		configs: cfg,
	}
}

func NewStageFullStateSyncCfg(bc core.BlockChain,
	db kv.RwDB,
	concurrency int,
	protocol syncProtocol,
	logger zerolog.Logger,
	logProgress bool) StageFullStateSyncCfg {

	return StageFullStateSyncCfg{
		bc:          bc,
		db:          db,
		concurrency: concurrency,
		protocol:    protocol,
		logger:      logger,
		logProgress: logProgress,
	}
}

// Exec progresses States stage in the forward direction
func (sss *StageFullStateSync) Exec(ctx context.Context, bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) (err error) {

	// for short range sync, skip this step
	if !s.state.initSync {
		return nil
	}

	// shouldn't execute for epoch chain
	if sss.configs.bc.ShardID() == shard.BeaconChainShardID && !s.state.isBeaconNode {
		return nil
	}

	// if states are already synced, don't execute this stage
	if s.state.status.statesSynced {
		return
	}

	// only execute this stage in fast/snap sync mode and once we reach to pivot
	if s.state.status.pivotBlock == nil ||
		s.state.CurrentBlockNumber() != s.state.status.pivotBlock.NumberU64() ||
		s.state.status.statesSynced {
		return nil
	}

	// maxHeight := s.state.status.targetBN
	// currentHead := s.state.CurrentBlockNumber()
	// if currentHead >= maxHeight {
	// 	return nil
	// }
	// targetHeight := s.state.currentCycle.TargetHeight

	currProgress := uint64(0)
	if errV := CreateView(ctx, sss.configs.db, tx, func(etx kv.Tx) error {
		if currProgress, err = s.CurrentStageProgress(etx); err != nil {
			return err
		}
		return nil
	}); errV != nil {
		return errV
	}
	if currProgress >= s.state.status.pivotBlock.NumberU64() {
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

	// Fetch states from neighbors
	currentBlockRootHash := s.state.bc.CurrentFastBlock().Root()
	scheme := sss.configs.bc.TrieDB().Scheme()
	sdm := newFullStateDownloadManager(sss.configs.bc.ChainDb(), scheme, tx, sss.configs.bc, sss.configs.concurrency, s.state.logger)
	sdm.setRootHash(currentBlockRootHash)

	sdm.SyncStarted()
	var wg sync.WaitGroup
	for i := 0; i < s.state.config.Concurrency; i++ {
		wg.Add(1)
		go sss.runStateWorkerLoop(ctx, sdm, &wg, i, startTime, s)
	}
	wg.Wait()

	// insert block
	if err := sss.configs.bc.WriteHeadBlock(s.state.status.pivotBlock); err != nil {
		sss.configs.logger.Warn().Err(err).
			Uint64("pivot block number", s.state.status.pivotBlock.NumberU64()).
			Msg(WrapStagedSyncMsg("insert pivot block failed"))
		// TODO: panic("pivot block is failed to insert in chain.")
		return err
	}

	// states should be fully synced in this stage
	s.state.status.statesSynced = true

	if err := sss.saveProgress(s, tx); err != nil {
		sss.configs.logger.Warn().Err(err).
			Uint64("pivot block number", s.state.status.pivotBlock.NumberU64()).
			Msg(WrapStagedSyncMsg("save progress for statesync stage failed"))
	}

	/*
		gbm := s.state.gbm

		// Setup workers to fetch states from remote node
		var wg sync.WaitGroup
		curHeight := s.state.CurrentBlockNumber()

		for bn := curHeight + 1; bn <= gbm.targetBN; bn++ {
			root := gbm.GetRootHash(bn)
			if root == emptyHash {
				continue
			}
			sdm.setRootHash(root)
			for i := 0; i < s.state.config.Concurrency; i++ {
				wg.Add(1)
				go sss.runStateWorkerLoop(ctx, sdm, &wg, i, startTime, s)
			}
			wg.Wait()
		}
	*/

	if useInternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

// runStateWorkerLoop creates a work loop for download states
func (sss *StageFullStateSync) runStateWorkerLoop(ctx context.Context, sdm *FullStateDownloadManager, wg *sync.WaitGroup, loopID int, startTime time.Time, s *StageState) {

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		accountTasks, codes, storages, healtask, codetask, nTasks, err := sdm.GetNextBatch()
		if nTasks == 0 {
			utils.Logger().Debug().Msg("the state worker loop received no more tasks")
			return
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				return
			}
		}

		if accountTasks != nil && len(accountTasks) > 0 {

			task := accountTasks[0]
			origin := task.Next
			limit := task.Last
			root := task.root
			cap := task.cap
			retAccounts, proof, stid, err := sss.configs.protocol.GetAccountRange(ctx, root, origin, limit, uint64(cap))
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					sss.configs.protocol.StreamFailed(stid, "GetAccountRange failed")
				}
				utils.Logger().Error().
					Err(err).
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetAccountRange failed"))
				err = errors.Wrap(err, "request error")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			} else if retAccounts == nil || len(retAccounts) == 0 {
				utils.Logger().Warn().
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetAccountRange failed, received empty accounts"))
				//err := errors.New("GetAccountRange received empty slots")
				//sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			}
			if err := sdm.HandleAccountRequestResult(task, retAccounts, proof, origin[:], limit[:], loopID, stid); err != nil {
				utils.Logger().Error().
					Err(err).
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetAccountRange handle result failed"))
				err = errors.Wrap(err, "handle result error")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			}

		} else if codes != nil && len(codes) > 0 {

			stid, err := sss.downloadByteCodes(ctx, sdm, codes, loopID)
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					sss.configs.protocol.StreamFailed(stid, "downloadByteCodes failed")
				}
				utils.Logger().Error().
					Err(err).
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("downloadByteCodes failed"))
				err = errors.Wrap(err, "request error")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			}

		} else if storages != nil && len(storages.accounts) > 0 {

			root := storages.root
			roots := storages.roots
			accounts := storages.accounts
			cap := storages.cap
			origin := storages.origin
			limit := storages.limit
			mainTask := storages.mainTask
			subTask := storages.subtask

			slots, proof, stid, err := sss.configs.protocol.GetStorageRanges(ctx, root, accounts, origin, limit, uint64(cap))
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					sss.configs.protocol.StreamFailed(stid, "GetStorageRanges failed")
				}
				utils.Logger().Error().
					Err(err).
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetStorageRanges failed"))
				err = errors.Wrap(err, "request error")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			} else if slots == nil || len(slots) == 0 {
				utils.Logger().Warn().
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetStorageRanges failed, received empty slots"))
				err := errors.New("GetStorageRanges received empty slots")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			}
			if err := sdm.HandleStorageRequestResult(mainTask, subTask, accounts, roots, origin, limit, slots, proof, loopID, stid); err != nil {
				utils.Logger().Error().
					Err(err).
					Str("stream", string(stid)).
					Msg(WrapStagedSyncMsg("GetStorageRanges handle result failed"))
				err = errors.Wrap(err, "handle result error")
				sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
				return
			}

		} else {
			// assign trie node Heal Tasks
			if healtask != nil && len(healtask.hashes) > 0 {
				root := healtask.root
				task := healtask.task
				hashes := healtask.hashes
				pathsets := healtask.pathsets
				paths := healtask.paths
				bytes := healtask.bytes

				nodes, stid, err := sss.configs.protocol.GetTrieNodes(ctx, root, pathsets, uint64(bytes))
				if err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						sss.configs.protocol.StreamFailed(stid, "GetTrieNodes failed")
					}
					utils.Logger().Error().
						Err(err).
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetTrieNodes failed"))
					err = errors.Wrap(err, "request error")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				} else if nodes == nil || len(nodes) == 0 {
					utils.Logger().Warn().
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetTrieNodes failed, received empty nodes"))
					err := errors.New("GetTrieNodes received empty nodes")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				}
				if err := sdm.HandleTrieNodeHealRequestResult(task, paths, hashes, nodes, loopID, stid); err != nil {
					utils.Logger().Error().
						Err(err).
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetTrieNodes handle result failed"))
					err = errors.Wrap(err, "handle result error")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				}
			}

			if codetask != nil && len(codetask.hashes) > 0 {
				task := codetask.task
				hashes := codetask.hashes
				bytes := codetask.bytes
				retCodes, stid, err := sss.configs.protocol.GetByteCodes(ctx, hashes, uint64(bytes))
				if err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						sss.configs.protocol.StreamFailed(stid, "GetByteCodes failed")
					}
					utils.Logger().Error().
						Err(err).
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetByteCodes failed"))
					err = errors.Wrap(err, "request error")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				} else if retCodes == nil || len(retCodes) == 0 {
					utils.Logger().Warn().
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetByteCodes failed, received empty codes"))
					err := errors.New("GetByteCodes received empty codes")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				}
				if err := sdm.HandleBytecodeRequestResult(task, hashes, retCodes, loopID, stid); err != nil {
					utils.Logger().Error().
						Err(err).
						Str("stream", string(stid)).
						Msg(WrapStagedSyncMsg("GetByteCodes handle result failed"))
					err = errors.Wrap(err, "handle result error")
					sdm.HandleRequestError(accountTasks, codes, storages, healtask, codetask, stid, err)
					return
				}
			}
		}
	}
}

func (sss *StageFullStateSync) downloadByteCodes(ctx context.Context, sdm *FullStateDownloadManager, codeTasks []*byteCodeTasksBundle, loopID int) (stid sttypes.StreamID, err error) {
	for _, codeTask := range codeTasks {
		// try to get byte codes from remote peer
		// if any of them failed, the stid will be the id of the failed stream
		retCodes, stid, err := sss.configs.protocol.GetByteCodes(ctx, codeTask.hashes, uint64(codeTask.cap))
		if err != nil {
			return stid, err
		}
		if len(retCodes) == 0 {
			return stid, errors.New("empty codes array")
		}
		if err = sdm.HandleBytecodeRequestResult(codeTask.task, codeTask.hashes, retCodes, loopID, stid); err != nil {
			return stid, err
		}
	}
	return
}

// func (sss *StageFullStateSync) downloadStates(ctx context.Context,
// 	root common.Hash,
// 	origin common.Hash,
// 	accounts []*accountTask,
// 	codes []common.Hash,
// 	storages *storageTaskBundle) ([][]byte, sttypes.StreamID, error) {

// 	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
// 	defer cancel()

// 	// if there is any account task, first we have to complete that
// 	if len(accounts) > 0 {

// 	}
// 	// hashes := append(codes, nodes...)
// 	// data, stid, err := sss.configs.protocol.GetNodeData(ctx, hashes)
// 	// if err != nil {
// 	// 	return nil, stid, err
// 	// }
// 	// if err := validateGetNodeDataResult(hashes, data); err != nil {
// 	// 	return nil, stid, err
// 	// }
// 	return data, stid, nil
// }

func (stg *StageFullStateSync) insertChain(gbm *blockDownloadManager,
	protocol syncProtocol,
	lbls prometheus.Labels,
	targetBN uint64) {

}

func (stg *StageFullStateSync) saveProgress(s *StageState, tx kv.RwTx) (err error) {

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
	if err = s.Update(tx, s.state.status.pivotBlock.NumberU64()); err != nil {
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

func (stg *StageFullStateSync) Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) (err error) {
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

func (stg *StageFullStateSync) CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) (err error) {
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
