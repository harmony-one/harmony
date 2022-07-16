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
	Bodies,
	// Stages below don't use Internet
	States,
	LastMile,
	Finish,
}

var DefaultUnwindOrder = UnwindOrder{
	Finish,
	LastMile,
	States,
	Bodies,
	BlockHashes,
	Headers,
}

var DefaultPruneOrder = PruneOrder{
	Finish,
	LastMile,
	States,
	Bodies,
	BlockHashes,
	Headers,
}

func DefaultStages(ctx context.Context,
	headersCfg StageHeadersCfg,
	blockHashesCfg StageBlockHashesCfg,
	bodiesCfg StageBodiesCfg,
	statesCfg StageStatesCfg,
	lastMileCfg StageLastMileCfg,
	finishCfg StageFinishCfg) []*Stage {

	handlerStageHeaders := NewStageHeders(headersCfg)
	handlerStageBlockHashes := NewStageBlockHashes(blockHashesCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handleStageStates := NewStageStates(statesCfg)
	handlerStageLastMile := NewStageLastMile(lastMileCfg)
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
			ID:          Bodies,
			Description: "Download block bodies",
			Handler:     handlerStageBodies,
		},
		{
			ID:          States,
			Description: "insert new blocks and update blockchain states",
			Handler:     handleStageStates,
		},
		{
			ID:          LastMile,
			Description: "update status for blocks after sync and update last mile blocks as well",
			Handler:     handlerStageLastMile,
		},
		{
			ID:          Finish,
			Description: "Final: update current block for the RPC API",
			Handler:     handlerStageFinish,
		},
	}
}
