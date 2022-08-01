package stagedsync

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type ExecFunc func(firstCycle bool, invalidBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error

type StageHandler interface {
	// ExecFunc is the execution function for the stage to move forward.
	// * state - is the current state of the stage and contains stage data.
	// * unwinder - if the stage needs to cause unwinding, `unwinder` methods can be used.
	Exec(firstCycle bool, invalidBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) error

	// UnwindFunc is the unwinding logic of the stage.
	// * unwindState - contains information about the unwind itself.
	// * stageState - represents the state of this stage at the beginning of unwind.
	Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) error

	// PruneFunc is the execution function for the stage to prune old data.
	// * state - is the current state of the stage and contains stage data.
	Prune(firstCycle bool, p *PruneState, tx kv.RwTx) error
}

// Stage is a single sync stage in staged sync.
type Stage struct {
	// ID of the sync stage. Should not be empty and should be unique. It is recommended to prefix it with reverse domain to avoid clashes (`com.example.my-stage`).
	ID SyncStageID
	// Handler handles the logic for the stage
	Handler StageHandler
	// Description is a string that is shown in the logs.
	Description string
	// DisabledDescription shows in the log with a message if the stage is disabled. Here, you can show which command line flags should be provided to enable the page.
	DisabledDescription string
	// Disabled defines if the stage is disabled. It sets up when the stage is build by its `StageBuilder`.
	Disabled bool
}

var ErrStopped = errors.New("stopped")
var ErrUnwind = errors.New("unwound")

// StageState is the state of the stage.
type StageState struct {
	state       *StagedSync
	ID          SyncStageID
	BlockNumber uint64 // BlockNumber is the current block number of the stage at the beginning of the state execution.
}

func (s *StageState) LogPrefix() string { return s.state.LogPrefix() }

func (s *StageState) CurrentStageProgress(db kv.Getter) (uint64, error) {
	return GetStageProgress(db, s.ID, s.state.isBeacon)
}

func (s *StageState) StageProgress(db kv.Getter, id SyncStageID) (uint64, error) {
	return GetStageProgress(db, id, s.state.isBeacon)
}

// Update updates the stage state (current block number) in the database. Can be called multiple times during stage execution.
func (s *StageState) Update(db kv.Putter, newBlockNum uint64) error {
	// if m, ok := syncMetrics[s.ID]; ok {
	// 	m.Set(newBlockNum)
	// }
	return SaveStageProgress(db, s.ID, s.state.isBeacon, newBlockNum)
}
func (s *StageState) UpdatePrune(db kv.Putter, blockNum uint64) error {
	return SaveStagePruneProgress(db, s.ID, s.state.isBeacon, blockNum)
}

// Unwinder allows the stage to cause an unwind.
type Unwinder interface {
	// UnwindTo begins staged sync unwind to the specified block.
	UnwindTo(unwindPoint uint64, invalidBlock common.Hash)
}

// UnwindState contains the information about unwind.
type UnwindState struct {
	ID SyncStageID
	// UnwindPoint is the block to unwind to.
	UnwindPoint        uint64
	CurrentBlockNumber uint64
	// If unwind is caused by a bad block, this hash is not empty
	InvalidBlock common.Hash
	state        *StagedSync
}

func (u *UnwindState) LogPrefix() string { return u.state.LogPrefix() }

// Done updates the DB state of the stage.
func (u *UnwindState) Done(db kv.Putter) error {
	return SaveStageProgress(db, u.ID, u.state.isBeacon, u.UnwindPoint)
}

type PruneState struct {
	ID              SyncStageID
	ForwardProgress uint64 // progress of stage forward move
	PruneProgress   uint64 // progress of stage prune move. after sync cycle it become equal to ForwardProgress by Done() method
	state           *StagedSync
}

func (s *PruneState) LogPrefix() string { return s.state.LogPrefix() + " Prune" }
func (s *PruneState) Done(db kv.Putter) error {
	return SaveStagePruneProgress(db, s.ID, s.state.isBeacon, s.ForwardProgress)
}
func (s *PruneState) DoneAt(db kv.Putter, blockNum uint64) error {
	return SaveStagePruneProgress(db, s.ID, s.state.isBeacon, blockNum)
}

// PruneTable has `limit` parameter to avoid too large data deletes per one sync cycle - better delete by small portions to reduce db.FreeList size
func PruneTable(tx kv.RwTx, table string, pruneTo uint64, ctx context.Context, limit int) error {
	c, err := tx.RwCursor(table)

	if err != nil {
		return ErrPruningCursorCreationFail
	}
	defer c.Close()

	i := 0
	for k, _, err := c.First(); k != nil; k, _, err = c.Next() {
		if err != nil {
			return err
		}
		i++
		if i > limit {
			break
		}

		blockNum := binary.BigEndian.Uint64(k)
		if blockNum >= pruneTo {
			break
		}
		select {
		case <-ctx.Done():
			return ErrStopped
		default:
		}
		if err = c.DeleteCurrent(); err != nil {
			return fmt.Errorf("failed to remove for block %d: %w", blockNum, err)
		}
	}
	return nil
}

func PruneTableDupSort(tx kv.RwTx, table string, logPrefix string, pruneTo uint64, logEvery *time.Ticker, ctx context.Context) error {
	c, err := tx.RwCursorDupSort(table)
	if err != nil {
		return ErrPruningCursorCreationFail
	}
	defer c.Close()

	for k, _, err := c.First(); k != nil; k, _, err = c.NextNoDup() {
		if err != nil {
			return fmt.Errorf("failed to move %s cleanup cursor: %w", table, err)
		}
		blockNum := binary.BigEndian.Uint64(k)
		if blockNum >= pruneTo {
			break
		}
		select {
		case <-logEvery.C:
			utils.Logger().Info().
				Msgf("[STAGED_SYNC] [%s] table:%s block:%s", logPrefix, table, blockNum)
		case <-ctx.Done():
			return ErrStopped
		default:
		}
		if err = c.DeleteCurrentDuplicates(); err != nil {
			return fmt.Errorf("failed to remove for block %d: %w", blockNum, err)
		}
	}
	return nil
}
