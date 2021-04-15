package services

import (
	"context"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// ConstructionHash implements the /construction/hash endpoint.
func (s *ConstructAPI) ConstructionHash(
	ctx context.Context, request *types.ConstructionHashRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	_, tx, rosettaError := unpackWrappedTransactionFromString(request.SignedTransaction, true)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil transaction",
		})
	}
	if tx.ShardID() != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v", tx.ShardID(), s.hmy.ShardID),
		})
	}
	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: tx.Hash().String()},
	}, nil
}

// ConstructionSubmit implements the /construction/submit endpoint.
func (s *ConstructAPI) ConstructionSubmit(
	ctx context.Context, request *types.ConstructionSubmitRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.SignedTransaction, true)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or nil unwrapped transaction",
		})
	}
	if tx.ShardID() != s.hmy.ShardID {
		return nil, common.NewError(common.StakingTransactionSubmissionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v", tx.ShardID(), s.hmy.ShardID),
		})
	}

	wrappedSenderAddress, err := getAddress(wrappedTransaction.From)
	if err != nil {
		return nil, common.NewError(common.StakingTransactionSubmissionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get address from wrapped transaction"),
		})
	}

	var signedTx hmyTypes.PoolTransaction
	if stakingTx, ok := tx.(*stakingTypes.StakingTransaction); ok && wrappedTransaction.IsStaking {
		signedTx = stakingTx
	} else if plainTx, ok := tx.(*hmyTypes.Transaction); ok && !wrappedTransaction.IsStaking {
		signedTx = plainTx
	} else {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "invalid/inconsistent type or unknown transaction type stored in wrapped transaction",
		})
	}

	txSenderAddress, err := signedTx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.StakingTransactionSubmissionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address from transaction").Error(),
		})
	}

	if wrappedSenderAddress != txSenderAddress {
		return nil, common.NewError(common.StakingTransactionSubmissionError, map[string]interface{}{
			"message": "transaction sender address does not match wrapped transaction sender address",
		})
	}

	if wrappedTransaction.IsStaking {
		if err := s.hmy.SendStakingTx(ctx, signedTx.(*stakingTypes.StakingTransaction)); err != nil {
			return nil, common.NewError(common.StakingTransactionSubmissionError, map[string]interface{}{
				"message": fmt.Sprintf("error is: %s, gas price is: %s, gas limit is: %d", err.Error(), signedTx.GasPrice().String(), signedTx.GasLimit()),
			})
		}
	} else {
		if err := s.hmy.SendTx(ctx, signedTx.(*hmyTypes.Transaction)); err != nil {
			return nil, common.NewError(common.TransactionSubmissionError, map[string]interface{}{
				"message": err.Error(),
			})
		}
	}

	return &types.TransactionIdentifierResponse{
		TransactionIdentifier: &types.TransactionIdentifier{Hash: tx.Hash().String()},
	}, nil
}
