package stagedstreamsync

import (
	"context"
)

type ForwardOrder []SyncStageID
type RevertOrder []SyncStageID
type CleanUpOrder []SyncStageID

var DefaultForwardOrder = ForwardOrder{
	Heads,
	SyncEpoch,
	ShortRange,
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
	ShortRange,
	SyncEpoch,
	Heads,
}

var DefaultCleanUpOrder = CleanUpOrder{
	Finish,
	LastMile,
	States,
	BlockBodies,
	ShortRange,
	SyncEpoch,
	Heads,
}

func DefaultStages(ctx context.Context,
	headsCfg StageHeadsCfg,
	seCfg StageEpochCfg,
	srCfg StageShortRangeCfg,
	bodiesCfg StageBodiesCfg,
	statesCfg StageStatesCfg,
	lastMileCfg StageLastMileCfg,
	finishCfg StageFinishCfg,
) []*Stage {

	handlerStageHeads := NewStageHeads(headsCfg)
	handlerStageShortRange := NewStageShortRange(srCfg)
	handlerStageEpochSync := NewStageEpoch(seCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handlerStageStates := NewStageStates(statesCfg)
	handlerStageLastMile := NewStageLastMile(lastMileCfg)
	handlerStageFinish := NewStageFinish(finishCfg)

	return []*Stage{
		{
			ID:          Heads,
			Description: "Retrieve Chain Heads",
			Handler:     handlerStageHeads,
		},
		{
			ID:          SyncEpoch,
			Description: "Sync only Last Block of Epoch",
			Handler:     handlerStageEpochSync,
		},
		{
			ID:          ShortRange,
			Description: "Short Range Sync",
			Handler:     handlerStageShortRange,
		},
		{
			ID:          BlockBodies,
			Description: "Retrieve Block Bodies",
			Handler:     handlerStageBodies,
		},
		{
			ID:          States,
			Description: "Update Blockchain State",
			Handler:     handlerStageStates,
		},
		{
			ID:          LastMile,
			Description: "update status for blocks after sync and update last mile blocks as well",
			Handler:     handlerStageLastMile,
		},
		{
			ID:          Finish,
			Description: "Finalize Changes",
			Handler:     handlerStageFinish,
		},
	}
}
