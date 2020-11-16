package services

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

var (
	// FormatDefaultSenderAddress ..
	FormatDefaultSenderAddress = ethcommon.HexToAddress("0xEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE")
)

// FormatTransaction for staking, cross-shard sender, and plain transactions
func FormatTransaction(
	tx hmytypes.PoolTransaction, receipt *hmytypes.Receipt, contractCode []byte,
) (fmtTx *types.Transaction, rosettaError *types.Error) {
	var operations []*types.Operation
	var isCrossShard, isStaking, isContractCreation bool
	var toShard uint32

	switch tx.(type) {
	case *stakingTypes.StakingTransaction:
		isStaking = true
		stakingTx := tx.(*stakingTypes.StakingTransaction)
		operations, rosettaError = GetNativeOperationsFromStakingTransaction(stakingTx, receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard, isContractCreation = false, false
		toShard = stakingTx.ShardID()
	case *hmytypes.Transaction:
		isStaking = false
		plainTx := tx.(*hmytypes.Transaction)
		operations, rosettaError = GetNativeOperationsFromTransaction(plainTx, receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard = plainTx.ShardID() != plainTx.ToShardID()
		isContractCreation = tx.To() == nil
		toShard = plainTx.ToShardID()
	default:
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unknown transaction type",
		})
	}
	fromShard := tx.ShardID()
	txID := &types.TransactionIdentifier{Hash: tx.Hash().String()}

	// Set all possible metadata
	var txMetadata TransactionMetadata
	if isContractCreation {
		contractID, rosettaError := newAccountIdentifier(receipt.ContractAddress)
		if rosettaError != nil {
			return nil, rosettaError
		}
		txMetadata.ContractAccountIdentifier = contractID
	} else if len(contractCode) > 0 && tx.To() != nil {
		// Contract code was found, so receiving account must be the contract address
		contractID, rosettaError := newAccountIdentifier(*tx.To())
		if rosettaError != nil {
			return nil, rosettaError
		}
		txMetadata.ContractAccountIdentifier = contractID
	}
	if isCrossShard {
		txMetadata.CrossShardIdentifier = txID
		txMetadata.ToShardID = &toShard
		txMetadata.FromShardID = &fromShard
	}
	if len(tx.Data()) > 0 && !isStaking {
		hexData := hex.EncodeToString(tx.Data())
		txMetadata.Data = &hexData
		txMetadata.Logs = receipt.Logs
	}
	metadata, err := types.MarshalMap(txMetadata)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	return &types.Transaction{
		TransactionIdentifier: txID,
		Operations:            operations,
		Metadata:              metadata,
	}, nil
}

// FormatCrossShardReceiverTransaction for cross-shard payouts on destination shard
func FormatCrossShardReceiverTransaction(
	cxReceipt *hmytypes.CXReceipt,
) (txs *types.Transaction, rosettaError *types.Error) {
	ctxID := &types.TransactionIdentifier{Hash: cxReceipt.TxHash.String()}
	senderAccountID, rosettaError := newAccountIdentifier(cxReceipt.From)
	if rosettaError != nil {
		return nil, rosettaError
	}
	receiverAccountID, rosettaError := newAccountIdentifier(*cxReceipt.To)
	if rosettaError != nil {
		return nil, rosettaError
	}
	metadata, err := types.MarshalMap(TransactionMetadata{
		CrossShardIdentifier: ctxID,
		ToShardID:            &cxReceipt.ToShardID,
		FromShardID:          &cxReceipt.ShardID,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	opMetadata, err := types.MarshalMap(common.CrossShardTransactionOperationMetadata{
		From: senderAccountID,
		To:   receiverAccountID,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	return &types.Transaction{
		TransactionIdentifier: ctxID,
		Metadata:              metadata,
		Operations: []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0, // There is no gas expenditure for cross-shard transaction payout
				},
				Type:    common.NativeCrossShardTransferOperation,
				Status:  common.SuccessOperationStatus.Status,
				Account: receiverAccountID,
				Amount: &types.Amount{
					Value:    cxReceipt.Amount.String(),
					Currency: &common.NativeCurrency,
				},
				Metadata: opMetadata,
			},
		},
	}, nil
}

// negativeBigValue formats a transaction value as a string
func negativeBigValue(num *big.Int) string {
	value := "0"
	if num != nil && num.Cmp(big.NewInt(0)) != 0 {
		value = fmt.Sprintf("-%v", new(big.Int).Abs(num))
	}
	return value
}
