package services

import (
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/rosetta/common"
	rpcV2 "github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// GetNativeOperationsFromTransaction for one of the following transactions:
// contract creation, cross-shard sender, same-shard transfer.
// Native operations only include operations that affect the native currency balance of an account.
func GetNativeOperationsFromTransaction(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
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
	gasOperations := newNativeOperations(gasExpended, accountID)

	// Handle different cases of plain transactions
	var txOperations []*types.Operation
	if tx.To() == nil {
		txOperations, rosettaError = newContractCreationNativeOperations(
			gasOperations[0].OperationIdentifier, tx, receipt, senderAddress,
		)
	} else if tx.ShardID() != tx.ToShardID() {
		txOperations, rosettaError = newCrossShardSenderTransferNativeOperations(
			gasOperations[0].OperationIdentifier, tx, senderAddress,
		)
	} else {
		txOperations, rosettaError = newTransferNativeOperations(
			gasOperations[0].OperationIdentifier, tx, receipt, senderAddress,
		)
	}
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(gasOperations, txOperations...), nil
}

// GetOperationsFromStakingTransaction for all staking directives
// Note that only native operations can come from staking transactions.
func GetOperationsFromStakingTransaction(
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
	gasOperations := newNativeOperations(gasExpended, accountID)

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

	return append(gasOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: gasOperations[0].OperationIdentifier.Index + 1,
		},
		RelatedOperations: []*types.OperationIdentifier{
			gasOperations[0].OperationIdentifier,
		},
		Type:     tx.StakingType().String(),
		Status:   common.SuccessOperationStatus.Status,
		Account:  accountID,
		Amount:   amount,
		Metadata: metadata,
	}), nil
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
			"message": fmt.Sprintf("collect rewards amount not found for %v", senderAddress),
		})
	}
	return amount, nil
}

// newTransferNativeOperations extracts & formats the native operation(s) for plain transaction,
// including contract transactions.
func newTransferNativeOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt, senderAddress ethcommon.Address,
) ([]*types.Operation, *types.Error) {
	if tx.To() == nil {
		return nil, common.NewError(common.CatchAllError, nil)
	}
	receiverAddress := *tx.To()

	// Common elements
	opType := common.NativeTransferOperation
	opStatus := common.SuccessOperationStatus.Status
	if receipt.Status == hmytypes.ReceiptStatusFailed {
		if len(tx.Data()) > 0 {
			opStatus = common.ContractFailureOperationStatus.Status
		} else {
			// Should never see a failed non-contract related transaction on chain
			opStatus = common.FailureOperationStatus.Status
			utils.Logger().Warn().Msgf("Failed transaction on chain: %v", tx.Hash().String())
		}
	}

	// Subtraction operation elements
	subOperationID := &types.OperationIdentifier{
		Index: startingOperationID.Index + 1,
	}
	subRelatedID := []*types.OperationIdentifier{
		startingOperationID,
	}
	subAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	subAmount := &types.Amount{
		Value:    negativeBigValue(tx.Value()),
		Currency: &common.NativeCurrency,
	}

	// Addition operation elements
	addOperationID := &types.OperationIdentifier{
		Index: subOperationID.Index + 1,
	}
	addRelatedID := []*types.OperationIdentifier{
		subOperationID,
	}
	addAccountID, rosettaError := newAccountIdentifier(receiverAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	addAmount := &types.Amount{
		Value:    tx.Value().String(),
		Currency: &common.NativeCurrency,
	}

	return []*types.Operation{
		{
			OperationIdentifier: subOperationID,
			RelatedOperations:   subRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             subAccountID,
			Amount:              subAmount,
		},
		{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             addAccountID,
			Amount:              addAmount,
		},
	}, nil
}

// newCrossShardSenderTransferNativeOperations extracts & formats the native operation(s)
// for cross-shard-tx on the sender's shard.
func newCrossShardSenderTransferNativeOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, senderAddress ethcommon.Address,
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

	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOperationID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOperationID,
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

// newContractCreationNativeOperations extracts & formats the native operation(s) for a contract creation tx
func newContractCreationNativeOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, txReceipt *hmytypes.Receipt, senderAddress ethcommon.Address,
) ([]*types.Operation, *types.Error) {
	// TODO: correct the contract creation transaction...

	// Set execution status as necessary
	status := common.SuccessOperationStatus.Status
	if txReceipt.Status == hmytypes.ReceiptStatusFailed {
		status = common.ContractFailureOperationStatus.Status
	}
	contractAddressID, rosettaError := newAccountIdentifier(txReceipt.ContractAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// Subtraction operation elements
	subOperationID := &types.OperationIdentifier{
		Index: startingOperationID.Index + 1,
	}
	subRelatedID := []*types.OperationIdentifier{
		startingOperationID,
	}
	subAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	subAmount := &types.Amount{
		Value:    negativeBigValue(tx.Value()),
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
		Value:    tx.Value().String(),
		Currency: &common.NativeCurrency,
	}

	return []*types.Operation{
		{
			OperationIdentifier: subOperationID,
			RelatedOperations:   subRelatedID,
			Type:                common.ContractCreationOperation,
			Status:              status,
			Account:             subAccountID,
			Amount:              subAmount,
		},
		{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                common.ContractCreationOperation,
			Status:              status,
			Account:             contractAddressID,
			Amount:              addAmount,
		},
	}, nil
}

// newNativeOperations creates a new operation with the gas fee as the first operation.
// Note: the gas fee is gasPrice * gasUsed.
func newNativeOperations(
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
