package services

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	types2 "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
)

// TransactionMetadata contains all (optional) information for a transaction.
type TransactionMetadata struct {
	// CrossShardIdentifier is the transaction identifier on the from/source shard
	CrossShardIdentifier *types.TransactionIdentifier `json:"cross_shard_transaction_identifier,omitempty"`
	ToShardID            *uint32                      `json:"to_shard,omitempty"`
	FromShardID          *uint32                      `json:"from_shard,omitempty"`
	// ContractAccountIdentifier is the 'main' contract account ID associated with a transaction
	ContractAccountIdentifier *types.AccountIdentifier `json:"contract_account_identifier,omitempty"`
	Data                      *string                  `json:"data,omitempty"`
	Logs                      []*hmyTypes.Log          `json:"logs,omitempty"`
	// SlotPubKeys SlotPubKeyToAdd SlotPubKeyToRemove are all hex representation of bls public key
	SlotPubKeys        []string `json:"slot_pub_keys,omitempty"`
	SlotKeySigs        []string `json:"slot_key_sigs,omitempty"`
	SlotKeyToAddSig    string   `json:"slot_key_to_add_sig,omitempty"`
	SlotPubKeyToAdd    string   `json:"slot_pub_key_to_add,omitempty"`
	SlotPubKeyToRemove string   `json:"slot_pub_key_to_remove,omitempty"`
}

// UnmarshalFromInterface ..
func (t *TransactionMetadata) UnmarshalFromInterface(metaData interface{}) error {
	var args TransactionMetadata
	dat, err := json.Marshal(metaData)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &args); err != nil {
		return err
	}
	*t = args
	return nil
}

// ConstructTransaction object (unsigned).
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
	case common.NativeCrossShardTransferOperation:
		if tx, rosettaError = constructCrossShardTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	case common.ContractCreationOperation:
		if tx, rosettaError = constructContractCreationTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	case common.NativeTransferOperation:
		if tx, rosettaError = constructPlainTransaction(components, metadata, sourceShardID); rosettaError != nil {
			return nil, rosettaError
		}
	case common.CreateValidatorOperation:
		if tx, rosettaError = constructCreateValidatorTransaction(components, metadata); rosettaError != nil {
			return nil, rosettaError
		}
	case common.EditValidatorOperation:
		if tx, rosettaError = constructEditValidatorTransaction(components, metadata); rosettaError != nil {
			return nil, rosettaError
		}
	case common.DelegateOperation:
		if tx, rosettaError = constructDelegateTransaction(components, metadata); rosettaError != nil {
			return nil, rosettaError
		}
	case common.UndelegateOperation:
		if tx, rosettaError = constructUndelegateTransaction(components, metadata); rosettaError != nil {
			return nil, rosettaError
		}
	case common.CollectRewardsOperation:
		if tx, rosettaError = constructCollectRewardsTransaction(components, metadata); rosettaError != nil {
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

func constructCreateValidatorTransaction(
	components *OperationComponents, metadata *ConstructMetadata,
) (hmyTypes.PoolTransaction, *types.Error) {
	createValidatorMsg := components.StakingMessage.(common.CreateValidatorOperationMetadata)
	validatorAddr, err := common2.Bech32ToAddress(createValidatorMsg.ValidatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert validator address error").Error(),
		})
	}
	var slotPubKeys []bls.SerializedPublicKey
	for _, slotPubKey := range metadata.Transaction.SlotPubKeys {
		var pubKey bls.SerializedPublicKey
		key, err := hexutil.Decode(slotPubKey)
		if err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "decode slot public key error").Error(),
			})
		}
		copy(pubKey[:], key)
		slotPubKeys = append(slotPubKeys, pubKey)
	}
	var slotKeySigs []bls.SerializedSignature
	for _, slotKeySig := range metadata.Transaction.SlotKeySigs {
		var keySig bls.SerializedSignature
		sig, err := hexutil.Decode(slotKeySig)
		if err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "decode slot key sig error").Error(),
			})
		}
		copy(keySig[:], sig)
		slotKeySigs = append(slotKeySigs, keySig)
	}
	stakePayloadMaker := func() (types2.Directive, interface{}) {
		return types2.DirectiveCreateValidator, types2.CreateValidator{
			Description: types2.Description{
				Name:            createValidatorMsg.Name,
				Identity:        createValidatorMsg.Identity,
				Website:         createValidatorMsg.Website,
				SecurityContact: createValidatorMsg.SecurityContact,
				Details:         createValidatorMsg.Details,
			},
			CommissionRates: types2.CommissionRates{
				Rate:          numeric.Dec{Int: createValidatorMsg.CommissionRate},
				MaxRate:       numeric.Dec{Int: createValidatorMsg.MaxCommissionRate},
				MaxChangeRate: numeric.Dec{Int: createValidatorMsg.MaxChangeRate},
			},
			MinSelfDelegation:  new(big.Int).Mul(createValidatorMsg.MinSelfDelegation, big.NewInt(1e18)),
			MaxTotalDelegation: new(big.Int).Mul(createValidatorMsg.MaxTotalDelegation, big.NewInt(1e18)),
			ValidatorAddress:   validatorAddr,
			SlotPubKeys:        slotPubKeys,
			SlotKeySigs:        slotKeySigs,
			Amount:             new(big.Int).Mul(createValidatorMsg.Amount, big.NewInt(1e18)),
		}
	}

	stakingTransaction, err := types2.NewStakingTransaction(metadata.Nonce, metadata.GasLimit, metadata.GasPrice, stakePayloadMaker)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "new staking transaction error").Error(),
		})
	}

	return stakingTransaction, nil
}

func constructEditValidatorTransaction(
	components *OperationComponents, metadata *ConstructMetadata,
) (hmyTypes.PoolTransaction, *types.Error) {
	editValidatorMsg := components.StakingMessage.(common.EditValidatorOperationMetadata)
	validatorAddr, err := common2.Bech32ToAddress(editValidatorMsg.ValidatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert validator address error").Error(),
		})
	}

	var slotKeyToAdd bls.SerializedPublicKey
	slotKeyToAddBytes, err := hexutil.Decode(metadata.Transaction.SlotPubKeyToAdd)
	copy(slotKeyToAdd[:], slotKeyToAddBytes)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "parse slotKeyToAdd error").Error(),
		})
	}

	var slotKeyToRemove bls.SerializedPublicKey
	slotKeyToRemoveBytes, err := hexutil.Decode(metadata.Transaction.SlotPubKeyToRemove)
	copy(slotKeyToRemove[:], slotKeyToRemoveBytes)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "parse slotKeyToRemove error").Error(),
		})
	}

	var slotKeyToAddSig bls.SerializedSignature
	SlotKeyToAddSigBytes, err := hexutil.Decode(metadata.Transaction.SlotKeyToAddSig)
	copy(slotKeyToAddSig[:], SlotKeyToAddSigBytes)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "parse slotKeyToAddSig error").Error(),
		})
	}

	stakePayloadMaker := func() (types2.Directive, interface{}) {
		return types2.DirectiveEditValidator, types2.EditValidator{
			ValidatorAddress: validatorAddr,
			Description: types2.Description{
				Name:            editValidatorMsg.Name,
				Identity:        editValidatorMsg.Identity,
				Website:         editValidatorMsg.Website,
				SecurityContact: editValidatorMsg.SecurityContact,
				Details:         editValidatorMsg.Details,
			},
			CommissionRate:     &numeric.Dec{Int: editValidatorMsg.CommissionRate},
			MinSelfDelegation:  new(big.Int).Mul(editValidatorMsg.MinSelfDelegation, big.NewInt(1e18)),
			MaxTotalDelegation: new(big.Int).Mul(editValidatorMsg.MaxTotalDelegation, big.NewInt(1e18)),
			SlotKeyToAdd:       &slotKeyToAdd,
			SlotKeyToRemove:    &slotKeyToRemove,
			SlotKeyToAddSig:    &slotKeyToAddSig,
		}
	}

	stakingTransaction, err := types2.NewStakingTransaction(metadata.Nonce, metadata.GasLimit, metadata.GasPrice, stakePayloadMaker)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "new staking transaction error").Error(),
		})
	}

	return stakingTransaction, nil
}

func constructDelegateTransaction(
	components *OperationComponents, metadata *ConstructMetadata,
) (hmyTypes.PoolTransaction, *types.Error) {
	delegaterMsg := components.StakingMessage.(common.DelegateOperationMetadata)
	delegatorAddr, err := common2.Bech32ToAddress(delegaterMsg.DelegatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert delegator address error").Error(),
		})
	}
	validatorAddr, err := common2.Bech32ToAddress(delegaterMsg.ValidatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert validator address error").Error(),
		})
	}

	stakePayloadMaker := func() (types2.Directive, interface{}) {
		return types2.DirectiveDelegate, types2.Delegate{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: validatorAddr,
			Amount:           new(big.Int).Mul(delegaterMsg.Amount, big.NewInt(1e18)),
		}
	}

	stakingTransaction, err := types2.NewStakingTransaction(metadata.Nonce, metadata.GasLimit, metadata.GasPrice, stakePayloadMaker)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "new staking transaction error").Error(),
		})
	}

	return stakingTransaction, nil
}

func constructUndelegateTransaction(
	components *OperationComponents, metadata *ConstructMetadata,
) (hmyTypes.PoolTransaction, *types.Error) {
	undelegaterMsg := components.StakingMessage.(common.UndelegateOperationMetadata)
	delegatorAddr, err := common2.Bech32ToAddress(undelegaterMsg.DelegatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert delegator address error").Error(),
		})
	}
	validatorAddr, err := common2.Bech32ToAddress(undelegaterMsg.ValidatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert validator address error").Error(),
		})
	}

	stakePayloadMaker := func() (types2.Directive, interface{}) {
		return types2.DirectiveUndelegate, types2.Undelegate{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: validatorAddr,
			Amount:           new(big.Int).Mul(undelegaterMsg.Amount, big.NewInt(1e18)),
		}
	}

	stakingTransaction, err := types2.NewStakingTransaction(metadata.Nonce, metadata.GasLimit, metadata.GasPrice, stakePayloadMaker)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "new staking transaction error").Error(),
		})
	}

	return stakingTransaction, nil
}

func constructCollectRewardsTransaction(
	components *OperationComponents, metadata *ConstructMetadata,
) (hmyTypes.PoolTransaction, *types.Error) {
	collectRewardsMsg := components.StakingMessage.(common.CollectRewardsMetadata)
	delegatorAddr, err := common2.Bech32ToAddress(collectRewardsMsg.DelegatorAddress)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "convert delegator address error").Error(),
		})
	}

	stakePayloadMaker := func() (types2.Directive, interface{}) {
		return types2.DirectiveCollectRewards, types2.CollectRewards{
			DelegatorAddress: delegatorAddr,
		}
	}

	stakingTransaction, err := types2.NewStakingTransaction(metadata.Nonce, metadata.GasLimit, metadata.GasPrice, stakePayloadMaker)
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "new staking transaction error").Error(),
		})
	}

	return stakingTransaction, nil
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
