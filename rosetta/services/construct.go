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
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/common/denominations"
	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

const (
	// DefaultGasPrice ..
	DefaultGasPrice = denominations.Nano
)

// ConstructAPI implements the server.ConstructAPIServicer interface.
type ConstructAPI struct {
	hmy           *hmy.Harmony
	signer        hmyTypes.Signer
	stakingSigner stakingTypes.Signer
}

// NewConstructionAPI creates a new instance of a ConstructAPI.
func NewConstructionAPI(hmy *hmy.Harmony) server.ConstructionAPIServicer {
	return &ConstructAPI{
		hmy:           hmy,
		signer:        hmyTypes.NewEIP155Signer(new(big.Int).SetUint64(hmy.ChainID)),
		stakingSigner: stakingTypes.NewEIP155Signer(new(big.Int).SetUint64(hmy.ChainID)),
	}
}

// ConstructionDerive implements the /construction/derive endpoint.
func (s *ConstructAPI) ConstructionDerive(
	ctx context.Context, request *types.ConstructionDeriveRequest,
) (*types.ConstructionDeriveResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	address, rosettaError := getAddressFromPublicKey(request.PublicKey)
	if rosettaError != nil {
		return nil, rosettaError
	}
	accountID, rosettaError := newAccountIdentifier(*address)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.ConstructionDeriveResponse{
		AccountIdentifier: accountID,
	}, nil
}

// ConstructMetadataOptions is constructed by ConstructionPreprocess for ConstructionMetadata options
type ConstructMetadataOptions struct {
	TransactionMetadata *TransactionMetadata `json:"transaction_metadata"`
	GasPriceMultiplier  *float64             `json:"gas_price_multiplier,omitempty"`
}

// UnmarshalFromInterface ..
// TODO (dm): add unit tests as options are added
func (m *ConstructMetadataOptions) UnmarshalFromInterface(metadata interface{}) error {
	var T ConstructMetadataOptions
	dat, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &T); err != nil {
		return err
	}
	if T.TransactionMetadata == nil {
		return fmt.Errorf("transaction metadata is required")
	}
	*m = T
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
	if request.Metadata == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require transaction metadata",
		})
	}
	txMetadata := &TransactionMetadata{}
	if err := txMetadata.UnmarshalFromInterface(request.Metadata); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid transaction metadata").Error(),
		})
	}
	if txMetadata.FromShardID != nil && *txMetadata.FromShardID != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("expect from shard ID to be %v", s.hmy.ShardID),
		})
	}
	components, rosettaError := GetOperationComponents(request.Operations)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender address is not found for given operations",
		})
	}
	options, err := types.MarshalMap(ConstructMetadataOptions{
		TransactionMetadata: txMetadata,
		GasPriceMultiplier:  request.SuggestedFeeMultiplier,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionPreprocessResponse{
		Options: options,
		RequiredPublicKeys: []*types.AccountIdentifier{
			components.From,
		},
	}, nil
}

// ConstructMetadata with a set of operations will construct a valid transaction
type ConstructMetadata struct {
	Nonce       uint64               `json:"nonce"`
	GasLimit    uint64               `json:"gas_limit"`
	GasPrice    *big.Int             `json:"gas_price"`
	Transaction *TransactionMetadata `json:"transaction_metadata"`
}

// UnmarshalFromInterface ..
// TODO (dm): add unit tests as options are added
func (m *ConstructMetadata) UnmarshalFromInterface(blockArgs interface{}) error {
	var T ConstructMetadata
	dat, err := json.Marshal(blockArgs)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &T); err != nil {
		return err
	}
	if T.GasPrice == nil {
		return fmt.Errorf("gas price is required")
	}
	if T.Transaction == nil {
		return fmt.Errorf("transaction metadat is required")
	}
	*m = T
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
			"message": errors.WithMessage(err, "invalid metadata option(s)").Error(),
		})
	}

	if request.PublicKeys == nil || len(request.PublicKeys) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require sender public key only",
		})
	}
	senderAddr, rosettaError := getAddressFromPublicKey(request.PublicKeys[0])
	if rosettaError != nil {
		return nil, rosettaError
	}
	nonce, err := s.hmy.GetPoolNonce(ctx, *senderAddr)
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
				"message": errors.WithMessage(err, "invalid tx data format").Error(),
			})
		}
	}
	estGasUsed, err := rpc.EstimateGas(ctx, s.hmy, rpc.CallArgs{Data: &data}, nil)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid transaction data").Error(),
		})
	}
	gasMul := float64(1)
	if options.GasPriceMultiplier != nil && *options.GasPriceMultiplier > 1 {
		gasMul = *options.GasPriceMultiplier
	}
	suggestedFee, suggestedPrice := getSuggestedFeeAndPrice(gasMul, new(big.Int).SetUint64(estGasUsed))

	metadata, err := types.MarshalMap(ConstructMetadata{
		Nonce:       nonce,
		GasPrice:    suggestedPrice,
		GasLimit:    estGasUsed,
		Transaction: options.TransactionMetadata,
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

// UnsignedTransaction is a wrapper for an unsigned transaction that includes its indented sender
type UnsignedTransaction struct {
	RLPBytes []byte                   `json:"rlp_bytes"`
	From     *types.AccountIdentifier `json:"from"`
}

// ConstructionPayloads implements the /construction/payloads endpoint.
func (s *ConstructAPI) ConstructionPayloads(
	ctx context.Context, request *types.ConstructionPayloadsRequest,
) (*types.ConstructionPayloadsResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	if request.Metadata == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require metadata",
		})
	}
	metadata := &ConstructMetadata{}
	if err := metadata.UnmarshalFromInterface(request.Metadata); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata").Error(),
		})
	}
	if request.PublicKeys == nil || len(request.PublicKeys) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require sender public key only",
		})
	}
	senderAddr, rosettaError := getAddressFromPublicKey(request.PublicKeys[0])
	if rosettaError != nil {
		return nil, rosettaError
	}
	senderID, rosettaError := newAccountIdentifier(*senderAddr)
	if rosettaError != nil {
		return nil, rosettaError
	}
	components, rosettaError := GetOperationComponents(request.Operations)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender address is not found for given operations",
		})
	}
	if types.Hash(senderID) != types.Hash(components.From) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender account identifier from operations does not match account identifier from public key",
		})
	}

	unsignedTx, rosettaError := ConstructTransaction(components, metadata, s.hmy.ShardID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	rlpBytes, err := rlp.EncodeToBytes(unsignedTx)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	marshalledBytes, err := json.Marshal(UnsignedTransaction{
		RLPBytes: rlpBytes,
		From:     components.From,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	payloads, rosettaError := s.getSigningPayload(unsignedTx, senderID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: string(marshalledBytes),
		Payloads:            payloads,
	}, nil
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
	if !request.Signed {
		return parseUnsignedTransaction(ctx, request)
	}
	return parseSignedTransaction(ctx, request)
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

// getSigningPayload ..
func (s *ConstructAPI) getSigningPayload(
	tx hmyTypes.PoolTransaction, senderAccountID *types.AccountIdentifier,
) ([]*types.SigningPayload, *types.Error) {
	payloads := []*types.SigningPayload{
		{
			AccountIdentifier: senderAccountID,
			SignatureType:     types.Ecdsa,
		},
	}
	switch tx.(type) {
	case *stakingTypes.StakingTransaction:
		payloads[0].Bytes = s.stakingSigner.Hash(tx.(*stakingTypes.StakingTransaction)).Bytes()
	case *hmyTypes.Transaction:
		payloads[0].Bytes = s.signer.Hash(tx.(*hmyTypes.Transaction)).Bytes()
	default:
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "constructed unknown or unsupported transaction",
		})
	}
	return payloads, nil
}

// getAddressFromPublicKey assumes that data is a compressed secp256k1 public key
func getAddressFromPublicKey(
	key *types.PublicKey,
) (*ethCommon.Address, *types.Error) {
	if key.CurveType != common.CurveType {
		return nil, common.NewError(common.UnsupportedCurveTypeError, map[string]interface{}{
			"message": fmt.Sprintf("currently only support %v", common.CurveType),
		})
	}
	// underlying eth crypto lib uses secp256k1
	publicKey, err := crypto.DecompressPubkey(key.Bytes)
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

// parseUnsignedTransaction ..
func parseUnsignedTransaction(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	return nil, nil
}

// parseSignedTransaction ..
func parseSignedTransaction(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	return nil, nil
}
