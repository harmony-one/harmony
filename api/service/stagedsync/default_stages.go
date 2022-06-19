package stagedsync

import (
	"context"
)

// UnwindOrder represents the order in which the stages needs to be unwound.
// The unwind order is important and not always just stages going backwards.
// Let's say, there is tx pool can be unwound only after execution.
// It's ok to remove some stage from here to disable only unwind of stage
type ForwardOrder []SyncStage
type UnwindOrder []SyncStage
type PruneOrder []SyncStage

var DefaultForwardOrder = ForwardOrder{
	Headers,
	BlockHashes,
	TasksQueue,
	Bodies,

	// Stages below don't use Internet
	Finish,
}

var DefaultUnwindOrder = UnwindOrder{
	Finish,

	Bodies,
	TasksQueue,
	BlockHashes,
	Headers,
}

var DefaultPruneOrder = PruneOrder{
	Finish,

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
	finishCfg StageFinishCfg) []*Stage {

	handlerStageHeaders := NewStageHeders(headersCfg)
	handlerStageBlockHashes := NewStageBlockHashes(blockHashesCfg)
	handlerStageTaskQueue := NewStageTasksQueue(taskQueueCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
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
			ID:          Finish,
			Description: "Final: update current block for the RPC API",
			Handler:     handlerStageFinish,
		},
	}
}
