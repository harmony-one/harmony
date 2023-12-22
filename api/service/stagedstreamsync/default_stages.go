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
		FullStateSync,
		States,
		LastMile,
		Finish,
	}

	StagesRevertOrder = RevertOrder{
		Finish,
		LastMile,
		States,
		FullStateSync,
		Receipts,
		BlockBodies,
		ShortRange,
		SyncEpoch,
		Heads,
	}

	StagesCleanUpOrder = CleanUpOrder{
		Finish,
		LastMile,
		States,
		FullStateSync,
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
	fullStateSyncCfg StageFullStateSyncCfg,
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
	handlerStageFullStateSync := NewStageFullStateSync(fullStateSyncCfg)
	handlerStageReceipts := NewStageReceipts(receiptsCfg)
	handlerStageLastMile := NewStageLastMile(lastMileCfg)
	handlerStageFinish := NewStageFinish(finishCfg)

	return []*Stage{
		{
			ID:                 Heads,
			Description:        "Retrieve Chain Heads",
			Handler:            handlerStageHeads,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChains,
		},
		{
			ID:                 SyncEpoch,
			Description:        "Sync only Last Block of Epoch",
			Handler:            handlerStageEpochSync,
			RangeMode:          OnlyShortRange,
			ChainExecutionMode: OnlyEpochChain,
		},
		{
			ID:                 ShortRange,
			Description:        "Short Range Sync",
			Handler:            handlerStageShortRange,
			RangeMode:          OnlyShortRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 BlockBodies,
			Description:        "Retrieve Block Bodies",
			Handler:            handlerStageBodies,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 States,
			Description:        "Update Blockchain State",
			Handler:            handlerStageStates,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 StateSync,
			Description:        "Retrieve States",
			Handler:            handlerStageStateSync,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 FullStateSync,
			Description:        "Retrieve Full States",
			Handler:            handlerStageFullStateSync,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 Receipts,
			Description:        "Retrieve Receipts",
			Handler:            handlerStageReceipts,
			RangeMode:          OnlyLongRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 LastMile,
			Description:        "update status for blocks after sync and update last mile blocks as well",
			Handler:            handlerStageLastMile,
			RangeMode:          LongRangeAndShortRange,
			ChainExecutionMode: AllChainsExceptEpochChain,
		},
		{
			ID:                 Finish,
			Description:        "Finalize Changes",
			Handler:            handlerStageFinish,
			RangeMode:          LongRangeAndShortRange,
			ChainExecutionMode: AllChains,
		},
	}
}
