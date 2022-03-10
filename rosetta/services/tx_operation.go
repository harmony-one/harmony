package services

import (
	"math/big"

	"github.com/harmony-one/harmony/hmy/tracers"

	"github.com/harmony-one/harmony/internal/bech32"
	internalCommon "github.com/harmony-one/harmony/internal/common"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	hmytypes "github.com/harmony-one/harmony/core/types"
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
			tx, receipt, senderAddress, &startingOpIndex,
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
	tx *stakingTypes.StakingTransaction, receipt *hmytypes.Receipt, signed bool,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		senderAddress = FormatDefaultSenderAddress
	}
	accountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	var operations []*types.Operation

	if signed {
		// All operations excepts for cross-shard tx payout expend gas
		gasExpended := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), tx.GasPrice())
		operations = newNativeOperationsWithGasNoSubAccount(gasExpended, accountID)
	}

	// Format staking message for metadata using decimal numbers (hence usage of rpcV2)
	rpcStakingTx, err := rpcV2.NewStakingTransaction(tx, ethcommon.Hash{}, 0, 0, 0, signed)
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
	case stakingTypes.DirectiveUndelegate:
		if amount, rosettaError = getAmountFromUnDelegateMessage(receipt, tx.Data()); rosettaError != nil {
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

	if len(operations) > 0 {
		operations = append(operations, &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: operations[0].OperationIdentifier.Index + 1,
			},
			Type:     tx.StakingType().String(),
			Status:   GetTransactionStatus(tx, receipt),
			Account:  accountID,
			Amount:   amount,
			Metadata: metadata,
		})
	} else {
		operations = []*types.Operation{{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0,
			},
			Type:     tx.StakingType().String(),
			Status:   GetTransactionStatus(tx, receipt),
			Account:  accountID,
			Amount:   amount,
			Metadata: metadata,
		}}
	}

	if signed {

		// expose delegated balance
		if tx.StakingType() == stakingTypes.DirectiveDelegate {
			ops := getDelegateOperationForSubAccount(tx, receipt, operations[1])
			return append(operations, ops...), nil
		}

		if tx.StakingType() == stakingTypes.DirectiveUndelegate {
			op2 := getUndelegateOperationForSubAccount(tx, operations[1], receipt)
			return append(operations, op2), nil
		}
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

	amount := &types.Amount{
		Value:    positiveStringValue(delegateOperation.Amount.Value),
		Currency: delegateOperation.Amount.Currency,
		Metadata: delegateOperation.Amount.Metadata,
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
		Amount:   amount,
		Metadata: delegateOperation.Metadata,
	}

	return undelegateion
}

func getDelegateOperationForSubAccount(tx *stakingTypes.StakingTransaction, receipt *hmytypes.Receipt, delegateOperation *types.Operation) (ops []*types.Operation) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(tx.Data(), stakingTypes.DirectiveDelegate)
	if err != nil {
		return nil
	}
	stkMsg, ok := msg.(*stakingTypes.Delegate)
	if !ok {
		return nil
	}

	amt, _ := new(big.Int).SetString(delegateOperation.Amount.Value, 10)
	delegateAmt := new(big.Int).Sub(new(big.Int).SetUint64(0), amt)
	validatorAddress := delegateOperation.Metadata["validatorAddress"]
	idx := int64(0)

	logs := hmytypes.FindLogsWithTopic(receipt, staking.DelegateTopic)
	for _, log := range logs {
		if len(log.Data) > ethcommon.AddressLength && log.Address == stkMsg.DelegatorAddress {
			// add undelegated transaction
			subAccount := log.Data[:ethcommon.AddressLength]
			address := internalCommon.BytesToAddress(subAccount)
			b32Address, err := bech32.ConvertAndEncode(internalCommon.Bech32AddressHRP, address.Bytes())
			if err != nil {
				return nil
			}

			deductedAmt := new(big.Int).SetBytes(log.Data[ethcommon.AddressLength:])
			delegateAmt = stkMsg.Amount
			idx++
			ops = append(ops, &types.Operation{
				OperationIdentifier: &types.OperationIdentifier{
					Index: delegateOperation.OperationIdentifier.Index + idx,
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
						Address: b32Address,
						Metadata: map[string]interface{}{
							SubAccountMetadataKey: UnDelegation,
						},
					},
					Metadata: delegateOperation.Account.Metadata,
				},
				Amount: &types.Amount{
					Value:    negativeBigValue(deductedAmt),
					Currency: delegateOperation.Amount.Currency,
					Metadata: delegateOperation.Amount.Metadata,
				},
				Metadata: delegateOperation.Metadata,
			})
		}
	}

	idx++
	return append(ops, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: delegateOperation.OperationIdentifier.Index + idx,
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
	})

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
func GetTransactionStatus(tx hmytypes.PoolTransaction, receipt *hmytypes.Receipt) *string {
	if _, ok := tx.(*hmytypes.Transaction); ok {
		status := common.SuccessOperationStatus.Status
		if common.GetDefaultFix().IsForceTxSuccess(tx.Hash().Hex()) {
			return &status
		} else if common.GetDefaultFix().IsForceTxFailed(tx.Hash().Hex()) {
			status = common.FailureOperationStatus.Status
			return &status
		}

		if receipt.Status == hmytypes.ReceiptStatusFailed {
			if len(tx.Data()) == 0 && receipt.CumulativeGasUsed <= params.TxGas {
				status = common.FailureOperationStatus.Status
			} else {
				status = common.ContractFailureOperationStatus.Status
			}
		}
		return &status
	} else if _, ok := tx.(*stakingTypes.StakingTransaction); ok {
		return &common.SuccessOperationStatus.Status
	}
	// Type of tx unknown, so default to failure
	return &common.FailureOperationStatus.Status
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

	return newSameShardTransferNativeOperations(from, to, tx.Value(), *status, startingOperationIndex), nil
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
		contractInfo.ExecutionResult, *status, &startingIndex,
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
		contractInfo.ExecutionResult, *status, &startingIndex,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(basicOps, internalTxOps...), nil
}

// getContractInternalTransferNativeOperations extracts & formats the native operations for a contract's internal
// native token transfers (i.e: the sender of a transaction is the contract).
func getContractInternalTransferNativeOperations(
	executionResult []*tracers.RosettaLogItem, status string,
	startingOperationIndex *int64,
) ([]*types.Operation, *types.Error) {
	ops := []*types.Operation{}
	if executionResult == nil {
		// No error since nil execution result implies empty StructLogs, which is not an error.
		return ops, nil
	}

	for _, log := range executionResult {
		// skip meaningless information
		if log.Value.Cmp(big.NewInt(0)) != 0 {
			fromAccID, rosettaError := newRosettaAccountIdentifier(log.From)
			if rosettaError != nil {
				return nil, rosettaError
			}

			toAccID, rosettaError := newRosettaAccountIdentifier(log.To)
			if rosettaError != nil {
				return nil, rosettaError
			}

			txStatus := status
			if log.Reverted {
				txStatus = common.FailureOperationStatus.Status
			}

			ops = append(
				ops, newSameShardTransferNativeOperations(fromAccID, toAccID, log.Value, txStatus, startingOperationIndex)...,
			)
			nextOpIndex := ops[len(ops)-1].OperationIdentifier.Index + 1
			startingOperationIndex = &nextOpIndex
		}
	}

	return ops, nil
}

// getCrossShardSenderTransferNativeOperations extracts & formats the native operation(s)
// for cross-shard-tx on the sender's shard.
func getCrossShardSenderTransferNativeOperations(tx *hmytypes.Transaction, receipt *hmytypes.Receipt, senderAddress ethcommon.Address, startingOperationIndex *int64) ([]*types.Operation, *types.Error) {
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

	status := GetTransactionStatus(tx, receipt)
	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: opIndex,
			},
			Type:    common.NativeCrossShardTransferOperation,
			Status:  status,
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

	for validator, undelegationMap := range undelegationPayouts.Data {
		ops, err := getOperationAndTotalAmountFromUndelegationMap(validator, &opIndex, opType, undelegationMap)
		if err != nil {
			return nil, err
		}
		operations = append(operations, ops...)
	}

	return operations, nil
}

// getOperationAndTotalAmountFromUndelegationMap is a helper for getSideEffectOperationsFromUndelegationPayoutsMap which actually
// has some side effect(opIndex will be increased by this function) so be careful while using for other purpose
func getOperationAndTotalAmountFromUndelegationMap(validator ethcommon.Address, opIndex *int64,
	opType string, undelegationMap map[ethcommon.Address]*big.Int) ([]*types.Operation, *types.Error) {
	var operations []*types.Operation
	for delegator, amount := range undelegationMap {
		subAccId, rosettaError := newAccountIdentifierWithSubAccount(delegator, validator, map[string]interface{}{
			SubAccountMetadataKey: UnDelegation,
		})
		if rosettaError != nil {
			return nil, rosettaError
		}

		accID, rosettaError := newAccountIdentifier(delegator)
		if rosettaError != nil {
			return nil, rosettaError
		}

		receiverOp := &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: *opIndex,
			},
			Type:    opType,
			Status:  &common.SuccessOperationStatus.Status,
			Account: accID,
			Amount: &types.Amount{
				Value:    amount.String(),
				Currency: &common.NativeCurrency,
			},
		}
		*opIndex++

		payoutOp := &types.Operation{
			OperationIdentifier: &types.OperationIdentifier{
				Index: *opIndex,
			},
			RelatedOperations: []*types.OperationIdentifier{
				receiverOp.OperationIdentifier,
			},
			Type:    opType,
			Status:  &common.SuccessOperationStatus.Status,
			Account: subAccId,
			Amount: &types.Amount{
				Value:    negativeBigValue(amount),
				Currency: &common.NativeCurrency,
			},
		}
		operations = append(operations, receiverOp, payoutOp)
		*opIndex++
	}
	return operations, nil

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
			Status:  &common.SuccessOperationStatus.Status,
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

func getAmountFromUnDelegateMessage(receipt *hmytypes.Receipt, data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveUndelegate)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.Undelegate)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for unDelegate tx",
		})
	}

	deductedAmt := stkMsg.Amount
	logs := hmytypes.FindLogsWithTopic(receipt, staking.UnDelegateTopic)
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

	var subOperationID *types.OperationIdentifier
	var op []*types.Operation

	if from != nil {
		subOperationID = &types.OperationIdentifier{
			Index: opIndex,
		}

		subAmount := &types.Amount{
			Value:    negativeBigValue(amount),
			Currency: &common.NativeCurrency,
		}

		op = append(op, &types.Operation{
			OperationIdentifier: subOperationID,
			Type:                common.NativeTransferOperation,
			Status:              &status,
			Account:             from,
			Amount:              subAmount,
		})
	}
	if to != nil {
		var addOperationID *types.OperationIdentifier
		var addRelatedID []*types.OperationIdentifier
		if subOperationID == nil {
			addOperationID = &types.OperationIdentifier{
				Index: opIndex,
			}
		} else {
			addOperationID = &types.OperationIdentifier{
				Index: subOperationID.Index + 1,
			}
			addRelatedID = []*types.OperationIdentifier{
				subOperationID,
			}
		}

		addAmount := &types.Amount{
			Value:    amount.String(),
			Currency: &common.NativeCurrency,
		}

		op = append(op, &types.Operation{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                common.NativeTransferOperation,
			Status:              &status,
			Account:             to,
			Amount:              addAmount,
		})
	}

	return op
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
			Type:   common.ExpendGasOperation,
			Status: &common.SuccessOperationStatus.Status,
			Account: &types.AccountIdentifier{
				Address:  accountID.Address,
				Metadata: accountID.Metadata,
			},
			Amount: &types.Amount{
				Value:    negativeBigValue(gasFeeInATTO),
				Currency: &common.NativeCurrency,
			},
		},
	}
}

// newNativeOperationsWithGasNoSubAccount creates a new operation with the gas fee as the first operation.
// Note: the gas fee is gasPrice * gasUsed.
func newNativeOperationsWithGasNoSubAccount(
	gasFeeInATTO *big.Int, accountID *types.AccountIdentifier,
) []*types.Operation {
	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0, // gas operation is always first
			},
			Type:   common.ExpendGasOperation,
			Status: &common.SuccessOperationStatus.Status,
			Account: &types.AccountIdentifier{
				Address:  accountID.Address,
				Metadata: accountID.Metadata,
			},
			Amount: &types.Amount{
				Value:    negativeBigValue(gasFeeInATTO),
				Currency: &common.NativeCurrency,
			},
		},
	}
}
