package services

import (
	"fmt"
	"math/big"

	common2 "github.com/harmony-one/harmony/internal/common"

	"github.com/coinbase/rosetta-sdk-go/types"

	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/pkg/errors"
)

const (
	// maxNumOfConstructionOps ..
	maxNumOfConstructionOps = 2

	// transferOperationCount ..
	transferOperationCount = 2
)

// OperationComponents are components from a set of operations to construct a valid transaction
type OperationComponents struct {
	Type           string                   `json:"type"`
	From           *types.AccountIdentifier `json:"from"`
	To             *types.AccountIdentifier `json:"to"`
	Amount         *big.Int                 `json:"amount"`
	StakingMessage interface{}              `json:"staking_message,omitempty"`
}

// IsStaking ..
func (s *OperationComponents) IsStaking() bool {
	return s.StakingMessage != nil
}

// GetOperationComponents ensures the provided operations creates a valid transaction and returns
// the OperationComponents of the resulting transaction.
//
// Providing a gas expenditure operation is INVALID.
// All staking & cross-shard operations require metadata matching the operation type to be a valid.
// All other operations do not require metadata.
func GetOperationComponents(
	operations []*types.Operation,
) (*OperationComponents, *types.Error) {
	if operations == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operations",
		})
	}
	if len(operations) > maxNumOfConstructionOps || len(operations) == 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("invalid number of operations, must <= %v & > 0", maxNumOfConstructionOps),
		})
	}

	if len(operations) == transferOperationCount {
		return getTransferOperationComponents(operations)
	}
	switch operations[0].Type {
	case common.NativeCrossShardTransferOperation:
		return getCrossShardOperationComponents(operations[0])
	case common.ContractCreationOperation:
		return getContractCreationOperationComponents(operations[0])
	case common.CreateValidatorOperation:
		return getCreateValidatorOperationComponents(operations[0])
	case common.EditValidatorOperation:
		return getEditValidatorOperationComponents(operations[0])
	case common.DelegateOperation:
		return getDelegateOperationComponents(operations[0])
	case common.UndelegateOperation:
		return getUndelegateOperationComponents(operations[0])
	case common.CollectRewardsOperation:
		return getCollectRewardsOperationComponents(operations[0])
	default:
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("%v is unsupported or invalid operation type", operations[0].Type),
		})
	}
}

// getTransferOperationComponents ..
func getTransferOperationComponents(
	operations []*types.Operation,
) (*OperationComponents, *types.Error) {
	if len(operations) != transferOperationCount {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "require exactly 2 operations",
		})
	}
	op0, op1 := operations[0], operations[1]
	if op0.Type != common.NativeTransferOperation || op1.Type != common.NativeTransferOperation {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid operation type(s) for same shard transfer",
		})
	}

	val0, err := types.AmountValue(op0.Amount)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	val1, err := types.AmountValue(op1.Amount)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if new(big.Int).Add(val0, val1).Cmp(big.NewInt(0)) != 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "amount taken from sender is not exactly paid out to receiver for same shard transfer",
		})
	}
	if types.Hash(op0.Amount.Currency) != common.NativeCurrencyHash ||
		types.Hash(op1.Amount.Currency) != common.NativeCurrencyHash {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid currency for provided amounts",
		})
	}

	if len(op1.RelatedOperations) == 1 &&
		op1.RelatedOperations[0].Index != op0.OperationIdentifier.Index {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "second operation is not related to the first operation for same shard transfer",
		})
	} else if len(op0.RelatedOperations) == 1 &&
		op0.RelatedOperations[0].Index != op1.OperationIdentifier.Index {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "first operation is not related to the second operation for same shard transfer",
		})
	} else if len(op0.RelatedOperations) > 1 || len(op1.RelatedOperations) > 1 ||
		len(op0.RelatedOperations)^len(op1.RelatedOperations) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operations must only relate to one another in one direction for same shard transfers",
		})
	}

	components := &OperationComponents{
		Type:   op0.Type,
		Amount: new(big.Int).Abs(val0),
	}
	if val0.Sign() != 1 {
		components.From = op0.Account
		components.To = op1.Account
	} else {
		components.From = op1.Account
		components.To = op0.Account
	}
	if components.From == nil || components.To == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "both operations must have account identifiers for same shard transfer",
		})
	}
	return components, nil
}

// getCrossShardOperationComponents ..
func getCrossShardOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.CrossShardTransactionOperationMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}
	amount, err := types.AmountValue(operation.Amount)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if amount.Sign() == 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender amount must not be positive for cross shard transfer",
		})
	}
	if types.Hash(operation.Amount.Currency) != common.NativeCurrencyHash {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid currency for provided amounts",
		})
	}

	components := &OperationComponents{
		Type:   operation.Type,
		To:     metadata.To,
		From:   metadata.From,
		Amount: new(big.Int).Abs(amount),
	}
	if components.From == nil || components.To == nil || operation.Account == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from & receiver/to identifiers for cross shard transfer",
		})
	}
	if operation.Account.Address != components.From.Address {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation account identifier does not match sender/from identifiers for cross shard transfer",
		})
	}
	return components, nil
}

// getContractCreationOperationComponents ..
func getContractCreationOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	amount, err := types.AmountValue(operation.Amount)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if amount.Sign() == 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender amount must not be positive for contract creation",
		})
	}
	if types.Hash(operation.Amount.Currency) != common.NativeCurrencyHash {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid currency for provided amounts",
		})
	}

	components := &OperationComponents{
		Type:   operation.Type,
		From:   operation.Account,
		Amount: new(big.Int).Abs(amount),
	}
	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for contract creation",
		})
	}
	return components, nil
}

func getCreateValidatorOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.CreateValidatorOperationMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}
	if metadata.ValidatorAddress == "" || !common2.IsBech32Address(metadata.ValidatorAddress) {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "validator address must not be empty or wrong format",
		})
	}
	if metadata.CommissionRate == nil || metadata.MaxCommissionRate == nil || metadata.MaxChangeRate == nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "commission rate & max commission rate & max change rate must not be nil",
		})
	}
	if metadata.MinSelfDelegation == nil || metadata.MaxTotalDelegation == nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "min self delegation & max total delegation much not be nil",
		})
	}
	if metadata.Amount == nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "amount must not be nil",
		})
	}
	if metadata.Name == "" || metadata.Website == "" || metadata.Identity == "" ||
		metadata.SecurityContact == "" || metadata.Details == "" {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "name & website & identity & security contract & details must no be empty",
		})
	}

	// slot public key would be add into
	// https://github.com/harmony-one/harmony/blob/3a8125666817149eaf9cea7870735e26cfe49c87/rosetta/services/tx_construction.go#L16
	// see https://github.com/harmony-one/harmony/issues/3431

	components := &OperationComponents{
		Type:           operation.Type,
		From:           operation.Account,
		StakingMessage: metadata,
	}

	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for creating validator",
		})
	}

	return components, nil

}

func getEditValidatorOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.EditValidatorOperationMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}
	if metadata.ValidatorAddress == "" || !common2.IsBech32Address(metadata.ValidatorAddress) {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "validator address must not be empty or wrong format",
		})
	}
	if metadata.CommissionRate == nil || metadata.MinSelfDelegation == nil || metadata.MaxTotalDelegation == nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "commission rate & max commission rate & max change rate must not be nil",
		})
	}
	if metadata.Name == "" || metadata.Website == "" || metadata.Identity == "" ||
		metadata.SecurityContact == "" || metadata.Details == "" {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": "name & website & identity & security contract & details must no be empty",
		})
	}

	components := &OperationComponents{
		Type:           operation.Type,
		From:           operation.Account,
		StakingMessage: metadata,
	}

	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for editing validator",
		})
	}

	return components, nil

}

func getDelegateOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.DelegateOperationMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}

	// validator and delegator and amount already got checked inside UnmarshalFromInterface
	components := &OperationComponents{
		Type:           operation.Type,
		From:           operation.Account,
		StakingMessage: metadata,
	}

	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for delegating",
		})
	}

	return components, nil

}

func getUndelegateOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.UndelegateOperationMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}

	// validator and delegator and amount already got checked inside UnmarshalFromInterface
	components := &OperationComponents{
		Type:           operation.Type,
		From:           operation.Account,
		StakingMessage: metadata,
	}

	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for undelegating",
		})
	}

	return components, nil

}

func getCollectRewardsOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	if operation == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil operation",
		})
	}
	metadata := common.CollectRewardsMetadata{}
	if err := metadata.UnmarshalFromInterface(operation.Metadata); err != nil {
		return nil, common.NewError(common.InvalidStakingConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}

	//delegator already got checked inside UnmarshalFromInterface

	components := &OperationComponents{
		Type:           operation.Type,
		From:           operation.Account,
		StakingMessage: metadata,
	}

	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "operation must have account sender/from identifier for collecting rewards",
		})
	}

	return components, nil
}
