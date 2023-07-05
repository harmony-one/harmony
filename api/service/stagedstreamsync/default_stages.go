package stagedstreamsync

import (
	"context"
)

type ForwardOrder []SyncStageID
type RevertOrder []SyncStageID
type CleanUpOrder []SyncStageID

var (
	StagesForwardOrder ForwardOrder
	StagesRevertOrder  RevertOrder
	StagesCleanUpOrder CleanUpOrder
)

func initStagesOrder(syncMode SyncMode) {
	switch syncMode {
	case FullSync:
		initFullSyncStagesOrder()
	case FastSync:
		initFastSyncStagesOrder()
	default:
		panic("not supported sync mode")
	}
}

func initFullSyncStagesOrder() {
	StagesForwardOrder = ForwardOrder{
		Heads,
		SyncEpoch,
		ShortRange,
		BlockBodies,
		States,
		LastMile,
		Finish,
	}

	StagesRevertOrder = RevertOrder{
		Finish,
		LastMile,
		States,
		BlockBodies,
		ShortRange,
		SyncEpoch,
		Heads,
	}

	StagesCleanUpOrder = CleanUpOrder{
		Finish,
		LastMile,
		States,
		BlockBodies,
		ShortRange,
		SyncEpoch,
		Heads,
	}
}

func initFastSyncStagesOrder() {
	StagesForwardOrder = ForwardOrder{
		Heads,
		SyncEpoch,
		ShortRange,
		BlockBodies,
		Receipts,
		StateSync,
		LastMile,
		Finish,
	}

	StagesRevertOrder = RevertOrder{
		Finish,
		LastMile,
		StateSync,
		Receipts,
		BlockBodies,
		ShortRange,
		SyncEpoch,
		Heads,
	}

	StagesCleanUpOrder = CleanUpOrder{
		Finish,
		LastMile,
		StateSync,
		Receipts,
		BlockBodies,
		ShortRange,
		SyncEpoch,
		Heads,
	}
}

func DefaultStages(ctx context.Context,
	headsCfg StageHeadsCfg,
	seCfg StageEpochCfg,
	srCfg StageShortRangeCfg,
	bodiesCfg StageBodiesCfg,
	stateSyncCfg StageStateSyncCfg,
	statesCfg StageStatesCfg,
	receiptsCfg StageReceiptsCfg,
	lastMileCfg StageLastMileCfg,
	finishCfg StageFinishCfg,
) []*Stage {

	handlerStageHeads := NewStageHeads(headsCfg)
	handlerStageShortRange := NewStageShortRange(srCfg)
	handlerStageEpochSync := NewStageEpoch(seCfg)
	handlerStageBodies := NewStageBodies(bodiesCfg)
	handlerStageStates := NewStageStates(statesCfg)
	handlerStageStateSync := NewStageStateSync(stateSyncCfg)
	handlerStageReceipts := NewStageReceipts(receiptsCfg)
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
			ID:          StateSync,
			Description: "Retrieve States",
			Handler:     handlerStageStateSync,
		},
		{
			ID:          Receipts,
			Description: "Retrieve Receipts",
			Handler:     handlerStageReceipts,
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
