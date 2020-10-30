package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

const (
	// SignedPayloadLength is the required length of the ECDSA payload
	SignedPayloadLength = 65
)

// WrappedTransaction is a wrapper for a transaction that includes all relevant
// data to parse a transaction.
type WrappedTransaction struct {
	RLPBytes     []byte                   `json:"rlp_bytes"`
	IsStaking    bool                     `json:"is_staking"`
	ContractCode hexutil.Bytes            `json:"contract_code"`
	From         *types.AccountIdentifier `json:"from"`
}

// unpackWrappedTransactionFromString ..
func unpackWrappedTransactionFromString(
	str string,
) (*WrappedTransaction, hmyTypes.PoolTransaction, *types.Error) {
	wrappedTransaction := &WrappedTransaction{}
	if err := json.Unmarshal([]byte(str), wrappedTransaction); err != nil {
		return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unknown wrapped transaction format").Error(),
		})
	}
	if wrappedTransaction.RLPBytes == nil {
		return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "RLP encoded transactions are not found in wrapped transaction",
		})
	}
	if wrappedTransaction.From == nil {
		return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "from/sender not found in wrapped transaction",
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
	if len(request.PublicKeys) != 1 {
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
	wrappedTxMarshalledBytes, err := json.Marshal(WrappedTransaction{
		RLPBytes:     buf.Bytes(),
		From:         senderID,
		ContractCode: metadata.ContractCode,
		IsStaking:    components.IsStaking(),
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionPayloadsResponse{
		UnsignedTransaction: string(wrappedTxMarshalledBytes),
		Payloads:            []*types.SigningPayload{payload},
	}, nil
}

// getSigningPayload ..
func (s *ConstructAPI) getSigningPayload(
	tx hmyTypes.PoolTransaction, senderAccountID *types.AccountIdentifier,
) (*types.SigningPayload, *types.Error) {
	payload := &types.SigningPayload{
		AccountIdentifier: senderAccountID,
		SignatureType:     common.SignatureType,
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

// ConstructionCombine implements the /construction/combine endpoint.
func (s *ConstructAPI) ConstructionCombine(
	ctx context.Context, request *types.ConstructionCombineRequest,
) (*types.ConstructionCombineResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.UnsignedTransaction)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if len(request.Signatures) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require exactly 1 signature",
		})
	}

	sig := request.Signatures[0]
	if sig.SignatureType != common.SignatureType {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("invalid transaction type, currently only support %v", common.SignatureType),
		})
	}
	sigAddress, rosettaError := getAddressFromPublicKey(sig.PublicKey)
	if rosettaError != nil {
		return nil, rosettaError
	}
	sigAccountID, rosettaError := newAccountIdentifier(*sigAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if wrappedTransaction.From == nil || types.Hash(wrappedTransaction.From) != types.Hash(sigAccountID) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "signer public key does not match unsigned transaction's sender",
		})
	}
	txPayload, rosettaError := s.getSigningPayload(tx, sigAccountID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if sig.SigningPayload == nil || types.Hash(sig.SigningPayload) != types.Hash(txPayload) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "transaction signing payload does not match given signing payload",
		})
	}

	var err error
	var signedTx hmyTypes.PoolTransaction
	if len(sig.Bytes) != SignedPayloadLength {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("invalid signature byte length, require len %v got len %v",
				SignedPayloadLength, len(sig.Bytes)),
		})
	}
	if stakingTx, ok := tx.(*stakingTypes.StakingTransaction); ok && wrappedTransaction.IsStaking {
		signedTx, err = stakingTx.WithSignature(s.stakingSigner, sig.Bytes)
	} else if plainTx, ok := tx.(*hmyTypes.Transaction); ok && !wrappedTransaction.IsStaking {
		signedTx, err = plainTx.WithSignature(s.signer, sig.Bytes)
	} else {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "invalid/inconsistent type or unknown transaction type stored in wrapped transaction",
		})
	}
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to apply signature to transaction").Error(),
		})
	}
	senderAddress, err := signedTx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address with signed transaction").Error(),
		})
	}
	if *sigAddress != senderAddress {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "signature address does not match signed transaction sender address",
		})
	}

	buf := &bytes.Buffer{}
	if err := signedTx.EncodeRLP(buf); err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	wrappedTransaction.RLPBytes = buf.Bytes()
	wrappedTxMarshalledBytes, err := json.Marshal(wrappedTransaction)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionCombineResponse{SignedTransaction: string(wrappedTxMarshalledBytes)}, nil
}
