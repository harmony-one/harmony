package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/hmy"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
)

const (
	// DefaultGasPrice ..
	DefaultGasPrice = denominations.Nano
	// maxNumOfConstructionOps ..
	maxNumOfConstructionOps = 2
)

// ConstructAPI implements the server.ConstructAPIServicer interface.
type ConstructAPI struct {
	hmy *hmy.Harmony
}

// NewConstructionAPI creates a new instance of a ConstructAPI.
func NewConstructionAPI(hmy *hmy.Harmony) server.ConstructionAPIServicer {
	return &ConstructAPI{
		hmy: hmy,
	}
}

// ConstructionDerive implements the /construction/derive endpoint.
func (s *ConstructAPI) ConstructionDerive(
	ctx context.Context, request *types.ConstructionDeriveRequest,
) (*types.ConstructionDeriveResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	if request.PublicKey.CurveType != common.CurveType {
		return nil, common.NewError(common.UnsupportedCurveTypeError, map[string]interface{}{
			"message": fmt.Sprintf("currently only support %v", common.CurveType),
		})
	}
	address, rosettaError := getAddressFromPublicKeyBytes(request.PublicKey.Bytes)
	if rosettaError != nil {
		return nil, rosettaError
	}
	accountID, rosettaError := newAccountIdentifier(*address)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.ConstructionDeriveResponse{
		Address:  accountID.Address,
		Metadata: accountID.Metadata,
	}, nil
}

// ConstructMetadataOptions is constructed by ConstructionPreprocess for ConstructionMetadata options
type ConstructMetadataOptions struct {
	TransactionMetadata *TransactionMetadata `json:"transaction_metadata"`
	OperationComponents *OperationComponents `json:"operation_components"`
	GasPriceMultiplier  *float64             `json:"gas_price_multiplier,omitempty"`
}

// UnmarshalFromInterface ..
// TODO (dm): add unit tests as options are added
func (m *ConstructMetadataOptions) UnmarshalFromInterface(blockArgs interface{}) error {
	var args ConstructMetadataOptions
	dat, err := json.Marshal(blockArgs)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &args); err != nil {
		return err
	}
	*m = args
	return nil
}

// ConstructionPreprocess implements the /construction/preprocess endpoint.
// Note that `request.MaxFee` is never considered for this construction implementation.
func (s *ConstructAPI) ConstructionPreprocess(
	ctx context.Context, request *types.ConstructionPreprocessRequest,
) (*types.ConstructionPreprocessResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	txMetadata := &TransactionMetadata{}
	if err := txMetadata.UnmarshalFromInterface(request.Metadata); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid transaction metadata"),
		})
	}
	if txMetadata.FromShardID != nil && *txMetadata.FromShardID != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("expect from shard ID to be %v", s.hmy.ShardID),
		})
	}
	opComponents, rosettaError := getOperationComponents(request.Operations)
	if rosettaError != nil {
		return nil, rosettaError
	}
	options, err := types.MarshalMap(ConstructMetadataOptions{
		TransactionMetadata: txMetadata,
		OperationComponents: opComponents,
		GasPriceMultiplier:  request.SuggestedFeeMultiplier,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionPreprocessResponse{
		Options: options,
	}, nil
}

// ConstructMetadata contains all data to construct a valid transaction
type ConstructMetadata struct {
	Nonce               uint64               `json:"nonce"`
	GasPrice            *big.Int             `json:"gas_price"`
	TransactionMetadata *TransactionMetadata `json:"transaction_metadata"`
	OperationComponents *OperationComponents `json:"operation_components"`
}

// UnmarshalFromInterface ..
// TODO (dm): add unit tests as options are added
func (m *ConstructMetadata) UnmarshalFromInterface(blockArgs interface{}) error {
	var args ConstructMetadata
	dat, err := json.Marshal(blockArgs)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &args); err != nil {
		return err
	}
	*m = args
	return nil
}

// ConstructionMetadata implements the /construction/metadata endpoint.
func (s *ConstructAPI) ConstructionMetadata(
	ctx context.Context, request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	options := &ConstructMetadataOptions{}
	if err := options.UnmarshalFromInterface(request.Options); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata option(s)"),
		})
	}

	senderAddr, err := internalCommon.Bech32ToAddress(options.OperationComponents.From.Address)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid sender address identifier"),
		})
	}
	nonce, err := s.hmy.GetPoolNonce(ctx, senderAddr)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	data := hexutil.Bytes{}
	if options.TransactionMetadata.Data != nil {
		var err error
		if data, err = hexutil.Decode(*options.TransactionMetadata.Data); err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "invalid tx data format"),
			})
		}
	}
	estGasUsed, err := rpc.EstimateGas(ctx, s.hmy, rpc.CallArgs{Data: &data}, nil)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid tx data"),
		})
	}
	gasMul := float64(1)
	if options.GasPriceMultiplier != nil && *options.GasPriceMultiplier > 1 {
		gasMul = *options.GasPriceMultiplier
	}
	suggestedFee, suggestedGasPrice := getSuggestedFeeAndPrice(gasMul, new(big.Int).SetUint64(estGasUsed))

	metadata, err := types.MarshalMap(ConstructMetadata{
		Nonce:               nonce,
		GasPrice:            suggestedGasPrice,
		TransactionMetadata: options.TransactionMetadata,
		OperationComponents: options.OperationComponents,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionMetadataResponse{
		Metadata:     metadata,
		SuggestedFee: suggestedFee,
	}, nil
}

// ConstructionPayloads implements the /construction/payloads endpoint.
func (s *ConstructAPI) ConstructionPayloads(
	ctx context.Context, request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ConstructionCombine implements the /construction/combine endpoint.
func (s *ConstructAPI) ConstructionCombine(
	ctx context.Context, request *types.ConstructionCombineRequest,
) (*types.ConstructionCombineResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ConstructionParse implements the /construction/parse endpoint.
func (s *ConstructAPI) ConstructionParse(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ConstructionHash implements the /construction/hash endpoint.
func (s *ConstructAPI) ConstructionHash(
	ctx context.Context, request *types.ConstructionHashRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ConstructionSubmit implements the /construction/submit endpoint.
func (s *ConstructAPI) ConstructionSubmit(
	ctx context.Context, request *types.ConstructionSubmitRequest,
) (*types.TransactionIdentifierResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	return nil, nil
}

// getAddressFromPublicKeyBytes assumes that data is a compressed secp256k1 public key
func getAddressFromPublicKeyBytes(
	data []byte,
) (*ethCommon.Address, *types.Error) {
	// Note that the underlying eth crypto lib uses secp256k1
	publicKey, err := crypto.DecompressPubkey(data)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	address := crypto.PubkeyToAddress(*publicKey)
	return &address, nil
}

// getSuggestedFeeAndPrice ..
func getSuggestedFeeAndPrice(
	gasMul float64, estGasUsed *big.Int,
) ([]*types.Amount, *big.Int) {
	gasPriceFloat := big.NewFloat(0).Mul(big.NewFloat(DefaultGasPrice), big.NewFloat(gasMul))
	gasPriceTruncated, _ := gasPriceFloat.Uint64()
	gasPrice := new(big.Int).SetUint64(gasPriceTruncated)
	return []*types.Amount{
		{
			Value:    fmt.Sprintf("%v", new(big.Int).Mul(gasPrice, estGasUsed)),
			Currency: &common.Currency,
		},
	}, gasPrice
}

// OperationComponents are components from a set of operations to construct a valid transaction
type OperationComponents struct {
	Type           string                   `json:"type"`
	From           *types.AccountIdentifier `json:"from"`
	To             *types.AccountIdentifier `json:"to"`
	Amount         *big.Int                 `json:"amount"`
	StakingMessage interface{}              `json:"staking_message,omitempty"`
}

// getOperationComponents ensures the provided operations creates a valid transaction and returns
// the OperationComponents of the resulting transaction. Note that providing a gas expenditure operation is INVALID.
//
// Note that all staking operations require metadata matching the operation type to be a valid. All other
// operations do not require metadata.
func getOperationComponents(
	operations []*types.Operation,
) (*OperationComponents, *types.Error) {
	if len(operations) > maxNumOfConstructionOps || len(operations) == 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("invalid number of operations, must <= %v & > 0", maxNumOfConstructionOps),
		})
	}

	if len(operations) == 2 {
		return getTransferOperationComponents(operations)
	}
	switch operations[0].Type {
	case common.CrossShardTransferOperation:
		return getCrossShardOperationComponents(operations[0])
	case common.ContractCreationOperation:
		return getContractCreationOperationComponents(operations[0])
	default:
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "unsupported/invalid operation type",
		})
	}
}

// getTransferOperationComponents ..
func getTransferOperationComponents(
	operations []*types.Operation,
) (*OperationComponents, *types.Error) {
	if len(operations) != 2 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "same shard transfers must be exactly 2 operations",
		})
	}
	op0, op1 := operations[0], operations[1]

	if op0.Type != common.TransferOperation || op1.Type != common.TransferOperation {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid operation type(s) for same shard transfer",
		})
	}
	if types.Hash(op0.Amount.Currency) != common.CurrencyHash ||
		types.Hash(op1.Amount.Currency) != common.CurrencyHash {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid currency for provided amounts",
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
	if new(big.Int).Add(val0, val1).Cmp(big.NewInt(0)) == 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "amount taken from sender is not exactly paid out to receiver for same shard transfer",
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

	txAccs := &OperationComponents{
		Type:   op0.Type,
		Amount: new(big.Int).Abs(val0),
	}
	if val0.Sign() != -1 {
		txAccs.From = op0.Account
		txAccs.To = op1.Account
	} else {
		txAccs.From = op1.Account
		txAccs.To = op0.Account
	}
	if txAccs.From == nil || txAccs.To == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "both operations must have account identifiers for same shard transfer",
		})
	}
	return txAccs, nil
}

// getCrossShardOperationComponents ..
func getCrossShardOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	// TODO: implement
	return nil, nil
}

// getContractCreationOperationComponents ..
func getContractCreationOperationComponents(
	operation *types.Operation,
) (*OperationComponents, *types.Error) {
	// TODO: implement
	return nil, nil
}
