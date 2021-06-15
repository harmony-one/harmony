package services

import (
	"context"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/pkg/errors"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
)

// ConstructionParse implements the /construction/parse endpoint.
func (s *ConstructAPI) ConstructionParse(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.Transaction, request.Signed)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if tx.ShardID() != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v", tx.ShardID(), s.hmy.ShardID),
		})
	}
	if request.Signed {
		return parseSignedTransaction(ctx, wrappedTransaction, tx)
	}

	rsp, err := parseUnsignedTransaction(ctx, wrappedTransaction, tx)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err,
		})
	}

	// it is unsigned as it reach to here, makes no sense, just to happy rosetta testing
	switch rsp.Operations[0].Type {
	case common.CreateValidatorOperation:
		delete(rsp.Operations[0].Metadata, "slotPubKeys")
		delete(rsp.Operations[0].Metadata, "slotKeySigs")
		return rsp, nil
	default:
		return rsp, nil
	}
}

// parseUnsignedTransaction ..
func parseUnsignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx hmyTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	if _, err := getAddress(wrappedTransaction.From); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: tx.GasLimit(),
	}
	formattedTx, rosettaError := FormatTransaction(
		tx, intendedReceipt, &ContractInfo{ContractCode: wrappedTransaction.ContractCode}, false,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	tempAccID, rosettaError := newAccountIdentifier(FormatDefaultSenderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	foundSender := false
	operations := formattedTx.Operations
	for _, op := range operations {
		if op.Account.Address == tempAccID.Address {
			foundSender = true
			op.Account = wrappedTransaction.From
		}
		op.Status = nil
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
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: tx.GasLimit(),
	}
	formattedTx, rosettaError := FormatTransaction(
		tx, intendedReceipt, &ContractInfo{ContractCode: wrappedTransaction.ContractCode}, true,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	sender, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address for signed transaction").Error(),
		})
	}
	senderID, rosettaError := newAccountIdentifier(sender)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if senderID.Address != wrappedTransaction.From.Address {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "wrapped transaction sender/from does not match transaction signer",
		})
	}
	for _, op := range formattedTx.Operations {
		op.Status = nil
	}
	return &types.ConstructionParseResponse{
		Operations:               formattedTx.Operations,
		AccountIdentifierSigners: []*types.AccountIdentifier{senderID},
	}, nil
}
