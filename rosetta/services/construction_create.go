package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"

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
	str string, signed bool,
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
		index := 0
		if signed {
			index = 1
		}
		stakingTx := &stakingTypes.StakingTransaction{}
		if err := stakingTx.DecodeRLP(stream); err != nil {
			return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "rlp encoding error for staking transaction").Error(),
			})
		}
		intendedReceipt := &hmyTypes.Receipt{
			GasUsed: stakingTx.GasLimit(),
		}
		formattedTx, rosettaError := FormatTransaction(
			stakingTx, intendedReceipt, &ContractInfo{ContractCode: wrappedTransaction.ContractCode}, signed,
		)
		if rosettaError != nil {
			return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": rosettaError,
			})
		}

		var stakingTransaction *stakingTypes.StakingTransaction
		switch stakingTx.StakingType() {
		case stakingTypes.DirectiveCreateValidator:
			var createValidatorMsg common.CreateValidatorOperationMetadata
			// to solve deserialization error
			slotPubKeys := formattedTx.Operations[index].Metadata["slotPubKeys"]
			delete(formattedTx.Operations[index].Metadata, "slotPubKeys")
			slotKeySigs := formattedTx.Operations[index].Metadata["slotKeySigs"]
			delete(formattedTx.Operations[index].Metadata, "slotKeySigs")

			err := createValidatorMsg.UnmarshalFromInterface(formattedTx.Operations[index].Metadata)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			validatorAddr, err := common2.Bech32ToAddress(createValidatorMsg.ValidatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			stakePayloadMaker := func() (stakingTypes.Directive, interface{}) {
				return stakingTypes.DirectiveCreateValidator, stakingTypes.CreateValidator{
					Description: stakingTypes.Description{
						Name:            createValidatorMsg.Name,
						Identity:        createValidatorMsg.Identity,
						Website:         createValidatorMsg.Website,
						SecurityContact: createValidatorMsg.SecurityContact,
						Details:         createValidatorMsg.Details,
					},
					CommissionRates: stakingTypes.CommissionRates{
						Rate:          numeric.Dec{Int: createValidatorMsg.CommissionRate},
						MaxRate:       numeric.Dec{Int: createValidatorMsg.MaxCommissionRate},
						MaxChangeRate: numeric.Dec{Int: createValidatorMsg.MaxChangeRate},
					},
					MinSelfDelegation:  createValidatorMsg.MinSelfDelegation,
					MaxTotalDelegation: createValidatorMsg.MaxTotalDelegation,
					ValidatorAddress:   validatorAddr,
					Amount:             createValidatorMsg.Amount,
					SlotPubKeys:        slotPubKeys.([]bls.SerializedPublicKey),
					SlotKeySigs:        slotKeySigs.([]bls.SerializedSignature),
				}
			}
			stakingTransaction, _ = stakingTypes.NewStakingTransaction(stakingTx.Nonce(), stakingTx.GasLimit(), stakingTx.GasPrice(), stakePayloadMaker)
		case stakingTypes.DirectiveEditValidator:
			var editValidatorMsg common.EditValidatorOperationMetadata
			// to solve deserialization error
			slotKeyToRemove := formattedTx.Operations[index].Metadata["slotPubKeyToRemove"]
			delete(formattedTx.Operations[index].Metadata, "slotPubKeyToRemove")
			slotKeyToAdd := formattedTx.Operations[index].Metadata["slotPubKeyToAdd"]
			delete(formattedTx.Operations[index].Metadata, "slotPubKeyToAdd")
			slotKeySigs := formattedTx.Operations[index].Metadata["slotKeyToAddSig"]
			delete(formattedTx.Operations[index].Metadata, "slotKeyToAddSig")
			err := editValidatorMsg.UnmarshalFromInterface(formattedTx.Operations[index].Metadata)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			validatorAddr, err := common2.Bech32ToAddress(editValidatorMsg.ValidatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			stakePayloadMaker := func() (stakingTypes.Directive, interface{}) {
				return stakingTypes.DirectiveEditValidator, stakingTypes.EditValidator{
					ValidatorAddress: validatorAddr,
					Description: stakingTypes.Description{
						Name:            editValidatorMsg.Name,
						Identity:        editValidatorMsg.Identity,
						Website:         editValidatorMsg.Website,
						SecurityContact: editValidatorMsg.SecurityContact,
						Details:         editValidatorMsg.Details,
					},
					CommissionRate:     &numeric.Dec{Int: editValidatorMsg.CommissionRate},
					MinSelfDelegation:  editValidatorMsg.MinSelfDelegation,
					MaxTotalDelegation: editValidatorMsg.MaxTotalDelegation,
					SlotKeyToAdd:       slotKeyToAdd.(*bls.SerializedPublicKey),
					SlotKeyToRemove:    slotKeyToRemove.(*bls.SerializedPublicKey),
					SlotKeyToAddSig:    slotKeySigs.(*bls.SerializedSignature),
				}
			}
			stakingTransaction, _ = stakingTypes.NewStakingTransaction(stakingTx.Nonce(), stakingTx.GasLimit(), stakingTx.GasPrice(), stakePayloadMaker)
		case stakingTypes.DirectiveDelegate:
			var delegateMsg common.DelegateOperationMetadata
			err := delegateMsg.UnmarshalFromInterface(formattedTx.Operations[index].Metadata)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			validatorAddr, err := common2.Bech32ToAddress(delegateMsg.ValidatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			delegatorAddr, err := common2.Bech32ToAddress(delegateMsg.DelegatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			stakePayloadMaker := func() (stakingTypes.Directive, interface{}) {
				return stakingTypes.DirectiveDelegate, stakingTypes.Delegate{
					ValidatorAddress: validatorAddr,
					DelegatorAddress: delegatorAddr,
					Amount:           delegateMsg.Amount,
				}
			}
			stakingTransaction, _ = stakingTypes.NewStakingTransaction(stakingTx.Nonce(), stakingTx.GasLimit(), stakingTx.GasPrice(), stakePayloadMaker)
		case stakingTypes.DirectiveUndelegate:
			var undelegateMsg common.UndelegateOperationMetadata
			err := undelegateMsg.UnmarshalFromInterface(formattedTx.Operations[index].Metadata)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			validatorAddr, err := common2.Bech32ToAddress(undelegateMsg.ValidatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			delegatorAddr, err := common2.Bech32ToAddress(undelegateMsg.DelegatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			stakePayloadMaker := func() (stakingTypes.Directive, interface{}) {
				return stakingTypes.DirectiveUndelegate, stakingTypes.Undelegate{
					ValidatorAddress: validatorAddr,
					DelegatorAddress: delegatorAddr,
					Amount:           undelegateMsg.Amount,
				}
			}
			stakingTransaction, _ = stakingTypes.NewStakingTransaction(stakingTx.Nonce(), stakingTx.GasLimit(), stakingTx.GasPrice(), stakePayloadMaker)
		case stakingTypes.DirectiveCollectRewards:
			var collectRewardsMsg common.CollectRewardsMetadata
			err := collectRewardsMsg.UnmarshalFromInterface(formattedTx.Operations[index].Metadata)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			delegatorAddr, err := common2.Bech32ToAddress(collectRewardsMsg.DelegatorAddress)
			if err != nil {
				return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
					"message": err,
				})
			}
			stakePayloadMaker := func() (stakingTypes.Directive, interface{}) {
				return stakingTypes.DirectiveCollectRewards, stakingTypes.CollectRewards{
					DelegatorAddress: delegatorAddr,
				}
			}
			stakingTransaction, _ = stakingTypes.NewStakingTransaction(stakingTx.Nonce(), stakingTx.GasLimit(), stakingTx.GasPrice(), stakePayloadMaker)
		default:
			return nil, nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": "staking type error",
			})
		}
		stakingTransaction.SetRawSignature(stakingTx.RawSignatureValues())
		tx = stakingTransaction
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
	if senderID.Address != components.From.Address {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender account identifier from operations does not match account identifier from public key",
		})
	}
	if metadata.Transaction.FromShardID != nil && *metadata.Transaction.FromShardID != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v",
				*metadata.Transaction.FromShardID, s.hmy.ShardID,
			),
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
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.UnsignedTransaction, false)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if len(request.Signatures) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require exactly 1 signature",
		})
	}
	if tx.ShardID() != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v", tx.ShardID(), s.hmy.ShardID),
		})
	}

	sig := request.Signatures[0]
	if sig.SignatureType != common.SignatureType {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("invalid signature type, currently only support %v", common.SignatureType),
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
	if wrappedTransaction.From == nil || wrappedTransaction.From.Address != sigAccountID.Address {
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
			"message": errors.WithMessage(err, "bad signature payload").Error(),
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
