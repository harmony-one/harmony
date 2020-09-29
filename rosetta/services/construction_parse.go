package services

import (
	"context"
	"crypto/ecdsa"
	"github.com/pkg/errors"

	"github.com/coinbase/rosetta-sdk-go/types"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// ConstructionParse implements the /construction/parse endpoint.
func (s *ConstructAPI) ConstructionParse(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromHexString(request.Transaction)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if !request.Signed {
		return parseUnsignedTransaction(ctx, wrappedTransaction, tx, s.tempSignerPrivateKey, s.signer, s.stakingSigner)
	}
	return parseSignedTransaction(ctx, wrappedTransaction, tx)
}

// parseUnsignedTransaction ..
func parseUnsignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx hmyTypes.PoolTransaction,
	tempPrivateKey *ecdsa.PrivateKey, signer hmyTypes.Signer, stakingSigner stakingTypes.Signer,
) (*types.ConstructionParseResponse, *types.Error) {
	if stakingTx, ok := tx.(*stakingTypes.StakingTransaction); ok {
		stakingTx, err := stakingTypes.Sign(stakingTx, stakingSigner, tempPrivateKey)
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		tx = stakingTx
	} else if plainTx, ok := tx.(*hmyTypes.Transaction); ok {
		plainTx, err := hmyTypes.SignTx(plainTx, signer, tempPrivateKey)
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		tx = plainTx
	} else {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unknown transaction type when parsing unwrapped & unsigned transaction",
		})
	}

	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: wrappedTransaction.EstimatedGasUsed,
	}
	formattedTx, rosettaError := formatTransaction(tx, intendedReceipt)
	if rosettaError != nil {
		return nil, rosettaError
	}
	tempAddress, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	tempAccID, rosettaError := newAccountIdentifier(tempAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	foundSender := false
	operations := formattedTx.Operations
	for _, op := range operations {
		if types.Hash(op.Account) == types.Hash(tempAccID) {
			foundSender = true
			op.Account = wrappedTransaction.From
		}
	}
	if !foundSender {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "temp sender not found in transaction operations",
		})
	}
	return &types.ConstructionParseResponse{
		Operations: operations,
	}, nil
}

// parseSignedTransaction ..
func parseSignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx hmyTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: wrappedTransaction.EstimatedGasUsed,
	}
	formattedTx, rosettaError := formatTransaction(tx, intendedReceipt)
	if rosettaError != nil {
		return nil, rosettaError
	}
	sender, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address, invalid signed transaction"),
		})
	}
	senderID, rosettaError := newAccountIdentifier(sender)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if types.Hash(senderID) != types.Hash(wrappedTransaction.From) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "wrapped transaction sender/from does not match transaction signer",
		})
	}
	return &types.ConstructionParseResponse{
		Operations:               formattedTx.Operations,
		AccountIdentifierSigners: []*types.AccountIdentifier{senderID},
	}, nil
}
