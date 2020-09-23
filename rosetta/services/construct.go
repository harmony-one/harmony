package services

import (
	"bytes"
	"context"
	"crypto/ecdsa"
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
	internalCommon "github.com/harmony-one/harmony/internal/common"
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

	// tempSignerPrivateKey for unsigned transactions to satisfy assumption of
	// signed transaction for transaction processing/formatting
	tempSignerPrivateKey *ecdsa.PrivateKey
}

// NewConstructionAPI creates a new instance of a ConstructAPI.
func NewConstructionAPI(hmy *hmy.Harmony) server.ConstructionAPIServicer {
	return &ConstructAPI{
		hmy:                  hmy,
		signer:               hmyTypes.NewEIP155Signer(new(big.Int).SetUint64(hmy.ChainID)),
		stakingSigner:        stakingTypes.NewEIP155Signer(new(big.Int).SetUint64(hmy.ChainID)),
		tempSignerPrivateKey: internalCommon.MustGeneratePrivateKey(),
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

// WrappedTransaction is a wrapper for a transactions that includes all relevant
// data to parse the transaction in ConstructionParse
type WrappedTransaction struct {
	RLPBytes         []byte                   `json:"rlp_bytes"`
	From             *types.AccountIdentifier `json:"from"`
	EstimatedGasUsed uint64                   `json:"estimated_gas_used"`
	IsStaking        bool                     `json:"is_staking"`
}

// unpackWrappedTransactionFromHexString ..
func unpackWrappedTransactionFromHexString(
	hex string,
) (*WrappedTransaction, hmyTypes.PoolTransaction, *types.Error) {
	wrappedTransaction := &WrappedTransaction{}
	wrappedTxBytes, err := hexutil.Decode(hex)
	if err != nil {
		return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if err := json.Unmarshal(wrappedTxBytes, wrappedTransaction); err != nil {
		return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unknown unsigned transaction format").Error(),
		})
	}

	var tx hmyTypes.PoolTransaction
	stream := rlp.NewStream(bytes.NewBuffer(wrappedTransaction.RLPBytes), 0)
	if wrappedTransaction.IsStaking {
		stakingTx := &stakingTypes.StakingTransaction{}
		if err := stakingTx.DecodeRLP(stream); err != nil {
			return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "rlp encoding error for staking transaction").Error(),
			})
		}
		tx = stakingTx
	} else {
		plainTx := &hmyTypes.Transaction{}
		if err := plainTx.DecodeRLP(stream); err != nil {
			return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "rlp encoding error for plain transaction").Error(),
			})
		}
		tx = plainTx
	}
	return wrappedTransaction, tx, nil
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
	payload, rosettaError := s.getSigningPayload(unsignedTx, senderID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	buf := &bytes.Buffer{}
	if err := unsignedTx.EncodeRLP(buf); err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	marshalledBytes, err := json.Marshal(WrappedTransaction{
		RLPBytes:         buf.Bytes(),
		From:             senderID,
		EstimatedGasUsed: metadata.GasLimit,
		IsStaking:        components.IsStaking(),
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: string(marshalledBytes),
		Payloads:            []*types.SigningPayload{payload},
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
		return parseUnsignedTransaction(ctx, request.Transaction, s.tempSignerPrivateKey, s.signer, s.stakingSigner)
	}
	return parseSignedTransaction(ctx, request.Transaction)
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
) (*types.SigningPayload, *types.Error) {
	payload := &types.SigningPayload{
		AccountIdentifier: senderAccountID,
		SignatureType:     types.Ecdsa,
	}
	switch tx.(type) {
	case *stakingTypes.StakingTransaction:
		payload.Bytes = s.stakingSigner.Hash(tx.(*stakingTypes.StakingTransaction)).Bytes()
	case *hmyTypes.Transaction:
		payload.Bytes = s.signer.Hash(tx.(*hmyTypes.Transaction)).Bytes()
	default:
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "constructed unknown or unsupported transaction",
		})
	}
	return payload, nil
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
	ctx context.Context, wrappedTransactionHex string,
	tempPrivateKey *ecdsa.PrivateKey, signer hmyTypes.Signer, stakingSigner stakingTypes.Signer,
) (*types.ConstructionParseResponse, *types.Error) {
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromHexString(wrappedTransactionHex)
	if rosettaError != nil {
		return nil, rosettaError
	}

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
			"message": "unknown transaction type when parsing unwrapped transaction",
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

	operations := formattedTx.Operations
	for _, op := range operations {
		if amount, _ := types.AmountValue(op.Amount); amount != nil && amount.Sign() == -1 {
			op.Account = wrappedTransaction.From
			break
		}
	}
	tempB32Address := internalCommon.MustAddressToBech32(crypto.PubkeyToAddress(tempPrivateKey.PublicKey))
	for _, op := range operations {
		if op.Account.Address == tempB32Address {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": "constructed operations has 0 or 2+ senders - only 1 account can loose funds",
			})
		}
	}
	return &types.ConstructionParseResponse{
		Operations: operations,
	}, nil
}

// parseSignedTransaction ..
func parseSignedTransaction(
	ctx context.Context, wrappedTransactionHex string,
) (*types.ConstructionParseResponse, *types.Error) {
	return nil, nil
}
