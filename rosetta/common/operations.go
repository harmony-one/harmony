package common

import (
	"encoding/json"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"

	rpcV2 "github.com/harmony-one/harmony/rpc/v2"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Invariant: A transaction can only contain 1 type of native operation(s) other than gas expenditure.
const (
	// ExpendGasOperation is an operation that only affects the native currency.
	ExpendGasOperation = "Gas"

	// NativeTransferOperation is an operation that only affects the native currency.
	NativeTransferOperation = "NativeTransfer"

	// NativeCrossShardTransferOperation is an operation that only affects the native currency.
	NativeCrossShardTransferOperation = "NativeCrossShardTransfer"

	// ContractCreationOperation is an operation that only affects the native currency.
	ContractCreationOperation = "ContractCreation"

	// GenesisFundsOperation is a special operation for genesis block only.
	// Note that no transaction can be constructed with this operation.
	GenesisFundsOperation = "Genesis"

	// PreStakingBlockRewardOperation is a special operation for pre-staking era only.
	// Note that no transaction can be constructed with this operation.
	PreStakingBlockRewardOperation = "PreStakingBlockReward"

	// UndelegationPayoutOperation is a special operation for committee election block only.
	// Note that no transaction can be constructed with this operation.
	UndelegationPayoutOperation = "UndelegationPayout"
)

var (
	// PlainOperationTypes ..
	PlainOperationTypes = []string{
		ExpendGasOperation,
		NativeTransferOperation,
		NativeCrossShardTransferOperation,
		ContractCreationOperation,
		GenesisFundsOperation,
		PreStakingBlockRewardOperation,
		UndelegationPayoutOperation,
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

// CreateValidatorOperationMetadata ..
type CreateValidatorOperationMetadata rpcV2.CreateValidatorMsg

// EditValidatorOperationMetadata ..
type EditValidatorOperationMetadata rpcV2.EditValidatorMsg

// DelegateOperationMetadata ..
type DelegateOperationMetadata rpcV2.DelegateMsg

// UndelegateOperationMetadata ..
type UndelegateOperationMetadata rpcV2.UndelegateMsg

// CollectRewardsMetadata ..
type CollectRewardsMetadata rpcV2.CollectRewardsMsg

// CrossShardTransactionOperationMetadata ..
type CrossShardTransactionOperationMetadata struct {
	From *types.AccountIdentifier `json:"from"`
	To   *types.AccountIdentifier `json:"to"`
}

// UnmarshalFromInterface ..
func (s *CrossShardTransactionOperationMetadata) UnmarshalFromInterface(data interface{}) error {
	var T CrossShardTransactionOperationMetadata
	dat, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &T); err != nil {
		return err
	}
	if T.To == nil || T.From == nil {
		return fmt.Errorf("expected to & from to be present for CrossShardTransactionOperationMetadata")
	}
	*s = T
	return nil
}
