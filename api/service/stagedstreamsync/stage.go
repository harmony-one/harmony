package stagedstreamsync

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type ExecFunc func(firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error

type StageHandler interface {
	// Exec is the execution function for the stage to move forward.
	// * firstCycle - is it the first cycle of syncing.
	// * invalidBlockRevert - whether the execution is to solve the invalid block
	// * s - is the current state of the stage and contains stage data.
	// * reverter - if the stage needs to cause reverting, `reverter` methods can be used.
	Exec(ctx context.Context, firstCycle bool, invalidBlockRevert bool, s *StageState, reverter Reverter, tx kv.RwTx) error

	// Revert is the reverting logic of the stage.
	// * firstCycle - is it the first cycle of syncing.
	// * u - contains information about the revert itself.
	// * s - represents the state of this stage at the beginning of revert.
	Revert(ctx context.Context, firstCycle bool, u *RevertState, s *StageState, tx kv.RwTx) error

	// CleanUp is the execution function for the stage to prune old data.
	// * firstCycle - is it the first cycle of syncing.
	// * p - is the current state of the stage and contains stage data.
	CleanUp(ctx context.Context, firstCycle bool, p *CleanUpState, tx kv.RwTx) error
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

// StageState is the state of the stage.
type StageState struct {
	state       *StagedStreamSync
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
	return SaveStageProgress(db, s.ID, s.state.isBeacon, newBlockNum)
}
func (s *StageState) UpdateCleanUp(db kv.Putter, blockNum uint64) error {
	return SaveStageCleanUpProgress(db, s.ID, s.state.isBeacon, blockNum)
}

// Reverter allows the stage to cause an revert.
type Reverter interface {
	// RevertTo begins staged sync revert to the specified block.
	RevertTo(revertPoint uint64, invalidBlockNumber uint64, invalidBlockHash common.Hash, invalidBlockStreamID sttypes.StreamID)
}

// RevertState contains the information about revert.
type RevertState struct {
	ID          SyncStageID
	RevertPoint uint64 // RevertPoint is the block to revert to.
	state       *StagedStreamSync
}

func (u *RevertState) LogPrefix() string { return u.state.LogPrefix() }

// Done updates the DB state of the stage.
func (u *RevertState) Done(db kv.Putter) error {
	return SaveStageProgress(db, u.ID, u.state.isBeacon, u.RevertPoint)
}

// CleanUpState contains states of cleanup process for a specific stage
type CleanUpState struct {
	ID              SyncStageID
	ForwardProgress uint64 // progress of stage forward move
	CleanUpProgress uint64 // progress of stage prune move. after sync cycle it become equal to ForwardProgress by Done() method
	state           *StagedStreamSync
}

func (s *CleanUpState) LogPrefix() string { return s.state.LogPrefix() + " CleanUp" }
func (s *CleanUpState) Done(db kv.Putter) error {
	return SaveStageCleanUpProgress(db, s.ID, s.state.isBeacon, s.ForwardProgress)
}
func (s *CleanUpState) DoneAt(db kv.Putter, blockNum uint64) error {
	return SaveStageCleanUpProgress(db, s.ID, s.state.isBeacon, blockNum)
}
