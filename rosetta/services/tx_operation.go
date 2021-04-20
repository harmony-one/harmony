package services

import (
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	rpcV2 "github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

const (
	SubAccountMetadataKey = "type"
	Delegation            = "delegation"
	UnDelegation          = "undelegation"
	UndelegationPayout    = "UndelegationPayout"
)

// GetNativeOperationsFromTransaction for one of the following transactions:
// contract creation, cross-shard sender, same-shard transfer with and without code execution.
// Native operations only include operations that affect the native currency balance of an account.
func GetNativeOperationsFromTransaction(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt, contractInfo *ContractInfo,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		senderAddress = FormatDefaultSenderAddress
	}
	accountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// All operations excepts for cross-shard tx payout expend gas
	gasExpended := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), tx.GasPrice())
	gasOperations := newNativeOperationsWithGas(gasExpended, accountID)
	startingOpIndex := gasOperations[0].OperationIdentifier.Index + 1

	// Handle based on tx type & available data.
	var txOperations []*types.Operation
	if tx.To() == nil {
		txOperations, rosettaError = getContractCreationNativeOperations(
			tx, receipt, senderAddress, contractInfo, &startingOpIndex,
		)
	} else if tx.ShardID() != tx.ToShardID() {
		txOperations, rosettaError = getCrossShardSenderTransferNativeOperations(
			tx, senderAddress, &startingOpIndex,
		)
	} else if contractInfo != nil && contractInfo.ExecutionResult != nil {
		txOperations, rosettaError = getContractTransferNativeOperations(
			tx, receipt, senderAddress, tx.To(), contractInfo, &startingOpIndex,
		)
	} else {
		txOperations, rosettaError = getBasicTransferNativeOperations(
			tx, receipt, senderAddress, tx.To(), &startingOpIndex,
		)
	}
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(gasOperations, txOperations...), nil
}

// GetNativeOperationsFromStakingTransaction for all staking directives
// Note that only native token operations can come from staking transactions.
func GetNativeOperationsFromStakingTransaction(
	tx *stakingTypes.StakingTransaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		senderAddress = FormatDefaultSenderAddress
	}
	accountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// All operations excepts for cross-shard tx payout expend gas
	gasExpended := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), tx.GasPrice())
	gasOperations := newNativeOperationsWithGas(gasExpended, accountID)

	// Format staking message for metadata using decimal numbers (hence usage of rpcV2)
	rpcStakingTx, err := rpcV2.NewStakingTransaction(tx, ethcommon.Hash{}, 0, 0, 0)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	metadata, err := types.MarshalMap(rpcStakingTx.Msg)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	// Set correct amount depending on staking message directive that apply balance changes INSTANTLY
	var amount *types.Amount
	switch tx.StakingType() {
	case stakingTypes.DirectiveCreateValidator:
		if amount, rosettaError = getAmountFromCreateValidatorMessage(tx.Data()); rosettaError != nil {
			return nil, rosettaError
		}
	case stakingTypes.DirectiveDelegate:
		if amount, rosettaError = getAmountFromDelegateMessage(receipt, tx.Data()); rosettaError != nil {
			return nil, rosettaError
		}
	case stakingTypes.DirectiveCollectRewards:
		if amount, rosettaError = getAmountFromCollectRewards(receipt, senderAddress); rosettaError != nil {
			return nil, rosettaError
		}
	default:
		amount = &types.Amount{
			Value:    "0", // All other staking transactions do not apply balance changes instantly or at all
			Currency: &common.NativeCurrency,
		}
	}

	operations := append(gasOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: gasOperations[0].OperationIdentifier.Index + 1,
		},
		Type:     tx.StakingType().String(),
		Status:   GetTransactionStatus(tx, receipt),
		Account:  accountID,
		Amount:   amount,
		Metadata: metadata,
	})

	// expose delegated balance
	if tx.StakingType() == stakingTypes.DirectiveDelegate {
		op2 := getDelegateOperationForSubAccount(tx, operations[1])
		return append(operations, op2), nil
	}

	if tx.StakingType() == stakingTypes.DirectiveUndelegate {
		op2 := getUndelegateOperationForSubAccount(tx, operations[1], receipt)
		return append(operations, op2), nil
	}

	return operations, nil

}

func getUndelegateOperationForSubAccount(
	tx *stakingTypes.StakingTransaction, delegateOperation *types.Operation, receipt *hmytypes.Receipt,
) *types.Operation {
	// set sub account
	validatorAddress := delegateOperation.Metadata["validatorAddress"]
	delegateOperation.Account.SubAccount = &types.SubAccountIdentifier{
		Address: validatorAddress.(string),
		Metadata: map[string]interface{}{
			SubAccountMetadataKey: Delegation,
		},
	}

	undelegateion := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: delegateOperation.OperationIdentifier.Index + 1,
		},
		Type:   tx.StakingType().String(),
		Status: GetTransactionStatus(tx, receipt),
		Account: &types.AccountIdentifier{
			Address: delegateOperation.Account.Address,
			SubAccount: &types.SubAccountIdentifier{
				Address: validatorAddress.(string),
				Metadata: map[string]interface{}{
					SubAccountMetadataKey: UnDelegation,
				},
			},
			Metadata: delegateOperation.Account.Metadata,
		},
		Amount:   delegateOperation.Amount,
		Metadata: delegateOperation.Metadata,
	}

	return undelegateion
}

func getDelegateOperationForSubAccount(
	tx *stakingTypes.StakingTransaction, delegateOperation *types.Operation,
) *types.Operation {
	amt, _ := new(big.Int).SetString(delegateOperation.Amount.Value, 10)
	delegateAmt := new(big.Int).Sub(new(big.Int).SetUint64(0), amt)
	validatorAddress := delegateOperation.Metadata["validatorAddress"]
	delegation := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: delegateOperation.OperationIdentifier.Index + 1,
		},
		RelatedOperations: []*types.OperationIdentifier{
			{
				Index: delegateOperation.OperationIdentifier.Index,
			},
		},
		Type:   tx.StakingType().String(),
		Status: delegateOperation.Status,
		Account: &types.AccountIdentifier{
			Address: delegateOperation.Account.Address,
			SubAccount: &types.SubAccountIdentifier{
				Address: validatorAddress.(string),
				Metadata: map[string]interface{}{
					SubAccountMetadataKey: Delegation,
				},
			},
			Metadata: delegateOperation.Account.Metadata,
		},
		Amount: &types.Amount{
			Value:    delegateAmt.String(),
			Currency: delegateOperation.Amount.Currency,
			Metadata: delegateOperation.Amount.Metadata,
		},
		Metadata: delegateOperation.Metadata,
	}

	return delegation

}

// getSideEffectOperationsFromUndelegationPayouts from the given payouts.
// If the startingOperationIndex is provided, all operations will be indexed starting from the given operation index.
func getSideEffectOperationsFromUndelegationPayouts(
	payouts *hmy.UndelegationPayouts, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	return getSideEffectOperationsFromUndelegationPayoutsMap(
		payouts, common.UndelegationPayoutOperation, startingOperationIndex,
	)
}

// GetSideEffectOperationsFromPreStakingRewards from the given rewards.
// If the startingOperationIndex is provided, all operations will be indexed starting from the given operation index.
func GetSideEffectOperationsFromPreStakingRewards(
	rewards hmy.PreStakingBlockRewards, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	return getSideEffectOperationsFromValueMap(
		rewards, common.PreStakingBlockRewardOperation, startingOperationIndex,
	)
}

// GetSideEffectOperationsFromGenesisSpec for the given spec.
// If the startingOperationIndex is provided, all operations will be indexed starting from the given operation index.
func GetSideEffectOperationsFromGenesisSpec(
	spec *core.Genesis, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	valueMap := map[ethcommon.Address]*big.Int{}
	for address, acc := range spec.Alloc {
		valueMap[address] = acc.Balance
	}
	return getSideEffectOperationsFromValueMap(
		valueMap, common.GenesisFundsOperation, startingOperationIndex,
	)
}

// GetTransactionStatus for any valid harmony transaction given its receipt.
func GetTransactionStatus(tx hmytypes.PoolTransaction, receipt *hmytypes.Receipt) string {
	if _, ok := tx.(*hmytypes.Transaction); ok {
		status := common.SuccessOperationStatus.Status
		if receipt.Status == hmytypes.ReceiptStatusFailed {
			if len(tx.Data()) == 0 && receipt.CumulativeGasUsed <= params.TxGas {
				status = common.FailureOperationStatus.Status
			} else {
				status = common.ContractFailureOperationStatus.Status
			}
		}
		return status
	} else if _, ok := tx.(*stakingTypes.StakingTransaction); ok {
		return common.SuccessOperationStatus.Status
	}
	// Type of tx unknown, so default to failure
	return common.FailureOperationStatus.Status
}

// getBasicTransferNativeOperations extracts & formats the basic native operations for non-staking transaction.
// Note that this does NOT include any contract related transfers (i.e: internal transactions).
func getBasicTransferNativeOperations(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt, senderAddress ethcommon.Address, toAddress *ethcommon.Address,
	startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	if toAddress == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "tx receiver not found",
		})
	}

	// Common operation elements
	status := GetTransactionStatus(tx, receipt)
	from, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	to, rosettaError := newAccountIdentifier(*toAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return newSameShardTransferNativeOperations(from, to, tx.Value(), status, startingOperationIndex), nil
}

// getContractTransferNativeOperations extracts & formats the native operations for any
// transaction involving a contract.
// Note that this will include any native tokens that were transferred from the contract (i.e: internal transactions).
func getContractTransferNativeOperations(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt, senderAddress ethcommon.Address, toAddress *ethcommon.Address,
	contractInfo *ContractInfo, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	basicOps, rosettaError := getBasicTransferNativeOperations(
		tx, receipt, senderAddress, toAddress, startingOperationIndex,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}

	status := GetTransactionStatus(tx, receipt)
	startingIndex := basicOps[len(basicOps)-1].OperationIdentifier.Index + 1
	internalTxOps, rosettaError := getContractInternalTransferNativeOperations(
		contractInfo.ExecutionResult, status, &startingIndex,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(basicOps, internalTxOps...), nil
}

// getContractCreationNativeOperations extracts & formats the native operations for a contract creation tx
func getContractCreationNativeOperations(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt, senderAddress ethcommon.Address, contractInfo *ContractInfo,
	startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	basicOps, rosettaError := getBasicTransferNativeOperations(
		tx, receipt, senderAddress, &receipt.ContractAddress, startingOperationIndex,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	for _, op := range basicOps {
		op.Type = common.ContractCreationOperation
	}

	status := GetTransactionStatus(tx, receipt)
	startingIndex := basicOps[len(basicOps)-1].OperationIdentifier.Index + 1
	internalTxOps, rosettaError := getContractInternalTransferNativeOperations(
		contractInfo.ExecutionResult, status, &startingIndex,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(basicOps, internalTxOps...), nil
}

var (
	// internalNativeTransferEvmOps are the EVM operations that can execute a native transfer
	// where the sender is a contract address. This is also known as ops for an 'internal' transaction.
	// All operations have at least 7 elements on the stack when executed.
	internalNativeTransferEvmOps = map[string]interface{}{
		vm.CALL.String():     struct{}{},
		vm.CALLCODE.String(): struct{}{},
	}
)

// getContractInternalTransferNativeOperations extracts & formats the native operations for a contract's internal
// native token transfers (i.e: the sender of a transaction is the contract).
func getContractInternalTransferNativeOperations(
	executionResult *hmy.ExecutionResult, status string,
	startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	ops := []*types.Operation{}
	if executionResult == nil {
		// No error since nil execution result implies empty StructLogs, which is not an error.
		return ops, nil
	}

	for _, log := range executionResult.StructLogs {
		if _, ok := internalNativeTransferEvmOps[log.Op]; ok {
			fromAccID, rosettaError := newAccountIdentifier(log.ContractAddress)
			if rosettaError != nil {
				return nil, rosettaError
			}
			topIndex := len(log.Stack) - 1
			toAccID, rosettaError := newAccountIdentifier(ethcommon.HexToAddress(log.Stack[topIndex-1]))
			if rosettaError != nil {
				return nil, rosettaError
			}
			value, ok := new(big.Int).SetString(log.Stack[topIndex-2], 16)
			if !ok {
				return nil, common.NewError(common.CatchAllError, map[string]interface{}{
					"message": fmt.Sprintf("unable to set value amount, raw: %v", log.Stack[topIndex-2]),
				})
			}

			ops = append(
				ops, newSameShardTransferNativeOperations(fromAccID, toAccID, value, status, startingOperationIndex)...,
			)
			nextOpIndex := ops[len(ops)-1].OperationIdentifier.Index + 1
			startingOperationIndex = &nextOpIndex
		}
	}

	return ops, nil
}

// getCrossShardSenderTransferNativeOperations extracts & formats the native operation(s)
// for cross-shard-tx on the sender's shard.
func getCrossShardSenderTransferNativeOperations(
	tx *hmytypes.Transaction, senderAddress ethcommon.Address,
	startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	if tx.To() == nil {
		return nil, common.NewError(common.CatchAllError, nil)
	}
	senderAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	receiverAccountID, rosettaError := newAccountIdentifier(*tx.To())
	if rosettaError != nil {
		return nil, rosettaError
	}
	metadata, err := types.MarshalMap(common.CrossShardTransactionOperationMetadata{
		From: senderAccountID,
		To:   receiverAccountID,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	var opIndex int64
	if startingOperationIndex != nil {
		opIndex = *startingOperationIndex
	}

	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: opIndex,
			},
			Type:    common.NativeCrossShardTransferOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: senderAccountID,
			Amount: &types.Amount{
				Value:    negativeBigValue(tx.Value()),
				Currency: &common.NativeCurrency,
			},
			Metadata: metadata,
		},
	}, nil
}

// delegator address => validator address => amount
func getSideEffectOperationsFromUndelegationPayoutsMap(
	undelegationPayouts *hmy.UndelegationPayouts, opType string, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	var opIndex int64
	operations := []*types.Operation{}
	if startingOperationIndex != nil {
		opIndex = *startingOperationIndex
	}

	for delegator, undelegationMap := range undelegationPayouts.Data {

		accID, rosettaError := newAccountIdentifier(delegator)
		if rosettaError != nil {
			return nil, rosettaError
		}

		receiverOp := &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: opIndex,
			},
			Type:    opType,
			Status:  common.SuccessOperationStatus.Status,
			Account: accID,
		}

		opIndex++
		operations = append(operations, receiverOp)
		receiverIndex := len(operations) - 1

		totalAmount, ops, err := getOperationAndTotalAmountFromUndelegationMap(delegator, &opIndex,
			receiverOp.OperationIdentifier, opType, undelegationMap)
		if err != nil {
			return nil, err
		}
		operations = append(operations, ops...)

		operations[receiverIndex].Amount = &types.Amount{
			Value:    totalAmount.String(),
			Currency: &common.NativeCurrency,
		}
	}

	return operations, nil
}

// getOperationAndTotalAmountFromUndelegationMap is a helper for getSideEffectOperationsFromUndelegationPayoutsMap which actually
// has some side effect(opIndex will be increased by this function) so be careful while using for other purpose
func getOperationAndTotalAmountFromUndelegationMap(
	delegator ethcommon.Address, opIndex *int64, relatedOpIdentifier *types.OperationIdentifier, opType string,
	undelegationMap map[ethcommon.Address]*big.Int,
) (*big.Int, []*types.Operation, *types.Error) {
	totalAmount := new(big.Int).SetUint64(0)
	var operations []*types.Operation
	for validator, amount := range undelegationMap {
		totalAmount = new(big.Int).Add(totalAmount, amount)
		subAccId, rosettaError := newAccountIdentifierWithSubAccount(delegator, validator, map[string]interface{}{
			SubAccountMetadataKey: UnDelegation,
		})
		if rosettaError != nil {
			return nil, nil, rosettaError
		}
		payoutOp := &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: *opIndex,
			},
			RelatedOperations: []*types.OperationIdentifier{
				relatedOpIdentifier,
			},
			Type:    opType,
			Status:  common.SuccessOperationStatus.Status,
			Account: subAccId,
			Amount: &types.Amount{
				Value:    negativeBigValue(amount),
				Currency: &common.NativeCurrency,
			},
		}
		operations = append(operations, payoutOp)
		*opIndex++
	}
	return totalAmount, operations, nil

}

// getSideEffectOperationsFromValueMap is a helper for side effect operation construction from a address to value map.
func getSideEffectOperationsFromValueMap(
	valueMap map[ethcommon.Address]*big.Int, opType string, startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	var opIndex int64
	operations := []*types.Operation{}
	if startingOperationIndex != nil {
		opIndex = *startingOperationIndex
	} else {
		opIndex = 0
	}
	for address, value := range valueMap {
		accID, rosettaError := newAccountIdentifier(address)
		if rosettaError != nil {
			return nil, rosettaError
		}
		operations = append(operations, &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: opIndex,
			},
			Type:    opType,
			Status:  common.SuccessOperationStatus.Status,
			Account: accID,
			Amount: &types.Amount{
				Value:    value.String(),
				Currency: &common.NativeCurrency,
			},
		})
		opIndex++
	}
	return operations, nil
}

func getAmountFromCreateValidatorMessage(data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveCreateValidator)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.CreateValidator)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for create validator tx",
		})
	}
	return &types.Amount{
		Value:    negativeBigValue(stkMsg.Amount),
		Currency: &common.NativeCurrency,
	}, nil
}

func getAmountFromDelegateMessage(receipt *hmytypes.Receipt, data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveDelegate)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.Delegate)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for delegate tx",
		})
	}

	deductedAmt := stkMsg.Amount
	logs := hmytypes.FindLogsWithTopic(receipt, staking.DelegateTopic)
	for _, log := range logs {
		if len(log.Data) > ethcommon.AddressLength && log.Address == stkMsg.DelegatorAddress {
			// Remove re-delegation amount as funds were never credited to account's balance.
			deductedAmt = new(big.Int).Sub(deductedAmt, new(big.Int).SetBytes(log.Data[ethcommon.AddressLength:]))
		}
	}
	return &types.Amount{
		Value:    negativeBigValue(deductedAmt),
		Currency: &common.NativeCurrency,
	}, nil
}

func getAmountFromCollectRewards(
	receipt *hmytypes.Receipt, senderAddress ethcommon.Address,
) (*types.Amount, *types.Error) {
	var amount *types.Amount
	logs := hmytypes.FindLogsWithTopic(receipt, staking.CollectRewardsTopic)
	for _, log := range logs {
		if log.Address == senderAddress {
			amount = &types.Amount{
				Value:    big.NewInt(0).SetBytes(log.Data).String(),
				Currency: &common.NativeCurrency,
			}
			break
		}
	}
	if amount == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": fmt.Sprintf("collect rewards amount not found for %v", senderAddress.String()),
		})
	}
	return amount, nil
}

// newSameShardTransferNativeOperations creates a new slice of operations for a native transfer on the same shard.
func newSameShardTransferNativeOperations(
	from, to *types.AccountIdentifier, amount *big.Int, status string,
	startingOperationIndex *int64,
) []*types.Operation {
	// Subtraction operation elements
	var opIndex int64
	if startingOperationIndex != nil {
		opIndex = *startingOperationIndex
	} else {
		opIndex = 0
	}
	subOperationID := &types.OperationIdentifier{
		Index: opIndex,
	}
	subAmount := &types.Amount{
		Value:    negativeBigValue(amount),
		Currency: &common.NativeCurrency,
	}

	// Addition operation elements
	addOperationID := &types.OperationIdentifier{
		Index: subOperationID.Index + 1,
	}
	addRelatedID := []*types.OperationIdentifier{
		subOperationID,
	}
	addAmount := &types.Amount{
		Value:    amount.String(),
		Currency: &common.NativeCurrency,
	}

	return []*types.Operation{
		{
			OperationIdentifier: subOperationID,
			Type:                common.NativeTransferOperation,
			Status:              status,
			Account:             from,
			Amount:              subAmount,
		},
		{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                common.NativeTransferOperation,
			Status:              status,
			Account:             to,
			Amount:              addAmount,
		},
	}
}

// newNativeOperationsWithGas creates a new operation with the gas fee as the first operation.
// Note: the gas fee is gasPrice * gasUsed.
func newNativeOperationsWithGas(
	gasFeeInATTO *big.Int, accountID *types.AccountIdentifier,
) []*types.Operation {
	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0, // gas operation is always first
			},
			Type:    common.ExpendGasOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: accountID,
			Amount: &types.Amount{
				Value:    negativeBigValue(gasFeeInATTO),
				Currency: &common.NativeCurrency,
			},
		},
	}
}
