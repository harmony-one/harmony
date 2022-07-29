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
	Heads,
	BlockHashes,
	BlockBodies,
	// Stages below don't use Internet
	States,
	LastMile,
	Finish,
}

var DefaultUnwindOrder = UnwindOrder{
	Finish,
	LastMile,
	States,
	BlockBodies,
	BlockHashes,
	Heads,
}

var DefaultPruneOrder = PruneOrder{
	Finish,
	LastMile,
	States,
	BlockBodies,
	BlockHashes,
	Heads,
}

func DefaultStages(ctx context.Context,
	headsCfg StageHeadsCfg,
	blockHashesCfg StageBlockHashesCfg,
	bodiesCfg StageBodiesCfg,
	statesCfg StageStatesCfg,
	lastMileCfg StageLastMileCfg,
	finishCfg StageFinishCfg) []*Stage {

	handlerStageHeaders := NewStageHeders(headsCfg)
	handlerStageBlockHashes := NewStageBlockHashes(blockHashesCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handleStageStates := NewStageStates(statesCfg)
	handlerStageLastMile := NewStageLastMile(lastMileCfg)
	handlerStageFinish := NewStageFinish(finishCfg)

	return []*Stage{
		{
			ID:          Heads,
			Description: "Retrieve Chain Heads",
			Handler:     handlerStageHeaders,
		},
		{
			ID:          BlockHashes,
			Description: "Download block hashes",
			Handler:     handlerStageBlockHashes,
		},
		{
			ID:          BlockBodies,
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
			Description: "Final stage to update current block for the RPC API",
			Handler:     handlerStageFinish,
		},
	}
}
