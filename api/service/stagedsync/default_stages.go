package stagedsync

import (
	"context"
)

// UnwindOrder represents the order in which the stages needs to be unwound.
// The unwind order is important and not always just stages going backwards.
// Let's say, there is tx pool can be unwound only after execution.
// It's ok to remove some stage from here to disable only unwind of stage
type ForwardOrder []SyncStageID
type UnwindOrder []SyncStageID
type PruneOrder []SyncStageID

var DefaultForwardOrder = ForwardOrder{
	Headers,
	BlockHashes,
	TasksQueue,
	Bodies,
	// Stages below don't use Internet
	States,
	Finish,
}

var DefaultUnwindOrder = UnwindOrder{
	Finish,
	States,
	Bodies,
	TasksQueue,
	BlockHashes,
	Headers,
}

var DefaultPruneOrder = PruneOrder{
	Finish,
	States,
	Bodies,
	TasksQueue,
	BlockHashes,
	Headers,
}

func DefaultStages(ctx context.Context,
	headersCfg StageHeadersCfg,
	blockHashesCfg StageBlockHashesCfg,
	taskQueueCfg StageTasksQueueCfg,
	bodiesCfg StageBodiesCfg,
	statesCfg StageStatesCfg,
	finishCfg StageFinishCfg) []*Stage {

	handlerStageHeaders := NewStageHeders(headersCfg)
	handlerStageBlockHashes := NewStageBlockHashes(blockHashesCfg)
	handlerStageTaskQueue := NewStageTasksQueue(taskQueueCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handleStageStates := NewStageStates(statesCfg)
	handlerStageFinish := NewStageFinish(finishCfg)

	return []*Stage{
		{
			ID:          Headers,
			Description: "Download headers",
			Handler:     handlerStageHeaders,
		},
		{
			ID:          BlockHashes,
			Description: "Write block hashes",
			Handler:     handlerStageBlockHashes,
		},
		{
			ID:          TasksQueue,
			Description: "Generate tasks queue",
			Handler:     handlerStageTaskQueue,
		},
		{
			ID:          Bodies,
			Description: "Download block bodies",
			Handler:     handlerStageBodies,
		},
		{
			ID:          States,
			Description: "Final: insert new blocks and update blockchain states",
			Handler:     handleStageStates,
		},
		{
			ID:          Finish,
			Description: "Final: update current block for the RPC API",
			Handler:     handlerStageFinish,
		},
	}
}
