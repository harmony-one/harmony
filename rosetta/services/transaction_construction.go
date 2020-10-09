package services

import (
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
)

// ConstructTransaction object (unsigned).
// TODO (dm): implement staking transaction construction
func ConstructTransaction(
	components *OperationComponents, metadata *ConstructMetadata, sourceShardID uint32,
) (response hmyTypes.PoolTransaction, rosettaError *types.Error) {
	if components == nil || metadata == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil components or metadata",
		})
	}
	if metadata.Transaction.FromShardID != nil && *metadata.Transaction.FromShardID != sourceShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("intended source shard %v != tx metadata source shard %v",
				sourceShardID, *metadata.Transaction.FromShardID),
		})
	}

	var tx hmyTypes.PoolTransaction
	switch components.Type {
	case common.CrossShardTransferOperation:
		if tx, rosettaError = constructCrossShardTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	case common.ContractCreationOperation:
		if tx, rosettaError = constructContractCreationTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	case common.TransferOperation:
		if tx, rosettaError = constructPlainTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	default:
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("cannot create transaction with component type %v", components.Type),
		})
	}
	return tx, nil
}

// constructCrossShardTransaction ..
func constructCrossShardTransaction(
	components *OperationComponents, metadata *ConstructMetadata, sourceShardID uint32,
) (hmyTypes.PoolTransaction, *types.Error) {
	if components.To == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "cross shard transfer requires a receiver",
		})
	}
	to, err := getAddress(components.To)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid sender address").Error(),
		})
	}
	if metadata.Transaction.FromShardID == nil || metadata.Transaction.ToShardID == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "cross shard transfer requires to & from shard IDs",
		})
	}
	if *metadata.Transaction.FromShardID != sourceShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("cannot send transaction for shard %v", *metadata.Transaction.FromShardID),
		})
	}
	if *metadata.Transaction.FromShardID == *metadata.Transaction.ToShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "cross-shard transfer cannot be within same shard",
		})
	}
	data := hexutil.Bytes{}
	if metadata.Transaction.Data != nil {
		if data, err = hexutil.Decode(*metadata.Transaction.Data); err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "improper data format for transaction data").Error(),
			})
		}
	}
	return hmyTypes.NewCrossShardTransaction(
		metadata.Nonce, &to, *metadata.Transaction.FromShardID, *metadata.Transaction.ToShardID,
		components.Amount, metadata.GasLimit, metadata.GasPrice, data,
	), nil
}

// constructContractCreationTransaction ..
func constructContractCreationTransaction(
	components *OperationComponents, metadata *ConstructMetadata, sourceShardID uint32,
) (hmyTypes.PoolTransaction, *types.Error) {
	if metadata.Transaction.Data == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "contract creation requires data, but none found in transaction metadata",
		})
	}
	data, err := hexutil.Decode(*metadata.Transaction.Data)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "improper data format for transaction data").Error(),
		})
	}
	return hmyTypes.NewContractCreation(
		metadata.Nonce, sourceShardID, components.Amount, metadata.GasLimit, metadata.GasPrice, data,
	), nil
}

// constructPlainTransaction ..
func constructPlainTransaction(
	components *OperationComponents, metadata *ConstructMetadata, sourceShardID uint32,
) (hmyTypes.PoolTransaction, *types.Error) {
	if components.To == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "cross shard transfer requires a receiver",
		})
	}
	to, err := getAddress(components.To)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid sender address").Error(),
		})
	}
	data := hexutil.Bytes{}
	if metadata.Transaction.Data != nil {
		if data, err = hexutil.Decode(*metadata.Transaction.Data); err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "improper data format for transaction data").Error(),
			})
		}
	}
	return hmyTypes.NewTransaction(
		metadata.Nonce, to, sourceShardID, components.Amount, metadata.GasLimit, metadata.GasPrice, data,
	), nil
}
