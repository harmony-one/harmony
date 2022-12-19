package stagedsync

import (
	"context"
)

type ForwardOrder []SyncStageID
type RevertOrder []SyncStageID
type CleanUpOrder []SyncStageID

var DefaultForwardOrder = ForwardOrder{
	Heads,
	BlockHashes,
	BlockBodies,
	// Stages below don't use Internet
	States,
	LastMile,
	Finish,
}

var DefaultRevertOrder = RevertOrder{
	Finish,
	LastMile,
	States,
	BlockBodies,
	BlockHashes,
	Heads,
}

var DefaultCleanUpOrder = CleanUpOrder{
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

	handlerStageHeads := NewStageHeads(headsCfg)
	handlerStageBlockHashes := NewStageBlockHashes(blockHashesCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handleStageStates := NewStageStates(statesCfg)
	handlerStageLastMile := NewStageLastMile(lastMileCfg)
	handlerStageFinish := NewStageFinish(finishCfg)

	return []*Stage{
		{
			ID:          Heads,
			Description: "Retrieve Chain Heads",
			Handler:     handlerStageHeads,
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
			Description: "Insert new blocks and update blockchain states",
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
