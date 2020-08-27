package common

import (
	"github.com/coinbase/rosetta-sdk-go/types"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	// ExpendGasOperation ..
	ExpendGasOperation = "Gas"

	// TransferOperation ..
	TransferOperation = "Transfer"

	// CrossShardTransferOperation ..
	CrossShardTransferOperation = "CrossShardTransfer"

	// ContractCreationOperation ..
	ContractCreationOperation = "ContractCreation"

	// GenesisFundsOperation ..
	GenesisFundsOperation = "Genesis"

	// PreStakingEraBlockRewardOperation ..
	PreStakingEraBlockRewardOperation = "PreStakingBlockReward"
)

var (
	// PlainOperationTypes ..
	PlainOperationTypes = []string{
		ExpendGasOperation,
		TransferOperation,
		CrossShardTransferOperation,
		ContractCreationOperation,
		GenesisFundsOperation,
		PreStakingEraBlockRewardOperation,
	}

	// StakingOperationTypes ..
	StakingOperationTypes = []string{
		staking.DirectiveCreateValidator.String(),
		staking.DirectiveEditValidator.String(),
		staking.DirectiveDelegate.String(),
		staking.DirectiveUndelegate.String(),
		staking.DirectiveCollectRewards.String(),
	}
)

var (
	// SuccessOperationStatus for tx operations who's amount affects the account
	SuccessOperationStatus = &types.OperationStatus{
		Status:     "success",
		Successful: true,
	}

	// ContractFailureOperationStatus for tx operations who's amount does not affect the account
	// due to a contract call failure (but still incurs gas).
	ContractFailureOperationStatus = &types.OperationStatus{
		Status:     "contract_failure",
		Successful: false,
	}

	// FailureOperationStatus ..
	FailureOperationStatus = &types.OperationStatus{
		Status:     "failure",
		Successful: false,
	}
)
