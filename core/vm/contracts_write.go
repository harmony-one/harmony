package vm

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	staking "github.com/harmony-one/harmony/staking"
)

var WriteCapablePrecompiledContracts = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// Native Go contracts which are available as a precompile in the EVM
// These have the capability to alter the state (those in contracts.go do not)
type WriteCapablePrecompiledContract interface {
	RequiredGas(evm *EVM, input []byte) (uint64, error) // RequiredPrice calculates the contract gas use
  // use a different name from read-only contracts to be safe
	RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
func RunWriteCapablePrecompiledContract(p WriteCapablePrecompiledContract, evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	gas, err := p.RequiredGas(evm, input)
	if err != nil {
		return nil, err
	}
	if !contract.UseGas(gas) {
		return nil, ErrOutOfGas
	}
	return p.RunWriteCapable(evm, contract, input)
}

type stakingPrecompile struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *stakingPrecompile) RequiredGas(evm *EVM, input []byte) (uint64, error) {
	if len(input) < 64 {
		return 0, errors.New("Input is malformed")
	}
	homestead := evm.ChainConfig().IsS3(evm.EpochNumber)
	istanbul := evm.ChainConfig().IsIstanbul(evm.EpochNumber)
	gas, err := IntrinsicGas(input,
												 false, /* contractCreation */
												 homestead,
												 istanbul,
												 stakingTypes.Directive(getData(input, 63, 64)[0]) == stakingTypes.DirectiveCreateValidator)
	if err != nil {
		return 0, err
	}
	return gas, nil
}

func (c *stakingPrecompile) RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	// if evm.Context.ShardID != shard.BeaconChainShardID {
	// 	return nil, nil	// we are not shard 0, so this is not for us
	// }

	// need at least (1) initial 32 bytes for size, and (2) next 32 bytes for directive
	if len(input) < 64 {
		return nil, errors.New("Input is malformed")
	}
	// discard length of structure (first 32 members of the array)
	input = input[32:]
	// directive is a single byte
  var directive = stakingTypes.Directive(getData(input, 31, 32)[0])
	// store passed information in map
	args := map[string]interface{}{}
	switch directive {
		case stakingTypes.DirectiveCreateValidator: {
			if err := staking.UnpackFromStakingMethod("CreateValidator", args, input); err != nil {
				return nil, err
			}
			// a contract must not make anyone else a validator
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "ValidatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := staking.ParseBigIntFromKey(args, "Amount")
			if err != nil {
				return nil, err
			}
			description, err := staking.ParseDescription(args)
			if err != nil {
				return nil, err
			}
			commissionRates, err := staking.ParseCommissionRates(args)
			if err != nil {
				return nil, err
			}
			minSelfDelegation, err := staking.ParseBigIntFromKey(args, "MinSelfDelegation")
			if err != nil {
				return nil, err
			}
			maxTotalDelegation, err := staking.ParseBigIntFromKey(args, "MaxTotalDelegation")
			if err != nil {
				return nil, err
			}
			slotPubKeys, err := staking.ParseSlotPubKeys(args)
			if err != nil {
				return nil, err
			}
			slotKeySigs, err := staking.ParseSlotKeySigs(args)
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.CreateValidator{
				ValidatorAddress: address,
				Amount: amount,
				Description: description,
				CommissionRates: commissionRates,
				MinSelfDelegation: minSelfDelegation,
				MaxTotalDelegation: maxTotalDelegation,
				SlotPubKeys: slotPubKeys,
				SlotKeySigs: slotKeySigs,
			}
			if err := evm.CreateValidator(evm.StateDB, stakeMsg); err != nil {
				return nil, err
			} else {
				evm.StakeMsgs = append(evm.StakeMsgs, stakeMsg)
				return nil, nil
			}
		}
		case stakingTypes.DirectiveEditValidator: {
			if err := staking.UnpackFromStakingMethod("EditValidator", args, input); err != nil {
				return nil, err
			}
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "ValidatorAddress")
			if err != nil {
				return nil, err
			}
			description, err := staking.ParseDescription(args)
			if err != nil {
				return nil, err
			}
			commissionRate, err := staking.ParseCommissionRate(args)
			if err != nil {
				return nil, err
			}
			minSelfDelegation, err := staking.ParseBigIntFromKey(args, "MinSelfDelegation")
			if err != nil {
				return nil, err
			}
			maxTotalDelegation, err := staking.ParseBigIntFromKey(args, "MaxTotalDelegation")
			if err != nil {
				return nil, err
			}
			slotKeyToRemove, err := staking.ParseSlotPubKeyFromKey(args, "SlotKeyToRemove")
			if err != nil {
				return nil, err
			}
			slotKeyToAdd, err := staking.ParseSlotPubKeyFromKey(args, "SlotKeyToAdd")
			if err != nil {
				return nil, err
			}
			slotKeyToAddSig, err := staking.ParseSlotKeySigFromKey(args, "SlotKeyToAddSig")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.EditValidator{
				ValidatorAddress: address,
				Description: description,
				CommissionRate: commissionRate,
				MinSelfDelegation: minSelfDelegation,
				MaxTotalDelegation: maxTotalDelegation,
				SlotKeyToRemove: slotKeyToRemove,
				SlotKeyToAdd: slotKeyToAdd,
				SlotKeyToAddSig: slotKeyToAddSig,
			}
			return nil, evm.EditValidator(evm.StateDB, stakeMsg)
		}
		case stakingTypes.DirectiveDelegate: {
			if err := staking.UnpackFromStakingMethod("DelegateOrUndelegate", args, input); err != nil {
				return nil, err
			}
			// a contract should only delegate its own balance - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "DelegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := staking.ParseAddressFromKey(args, "ValidatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := staking.ParseBigIntFromKey(args, "Amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Delegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount: amount,
			}
			if err := evm.Delegate(evm.StateDB, stakeMsg); err != nil {
				return nil, err
			} else {
				evm.StakeMsgs = append(evm.StakeMsgs, stakeMsg)
				return nil, nil
			}
		}
		case stakingTypes.DirectiveUndelegate: {
			if err := staking.UnpackFromStakingMethod("DelegateOrUndelegate", args, input); err != nil {
				return nil, err
			}
			// a contract should only delegate its own balance - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "DelegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := staking.ParseAddressFromKey(args, "ValidatorAddress")
			if err != nil {
				return nil, err
			}
			// this type assertion is needed by Golang
			amount, err := staking.ParseBigIntFromKey(args, "Amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Undelegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount: amount,
			}
			return nil, evm.Undelegate(evm.StateDB, stakeMsg)
		}
		case stakingTypes.DirectiveCollectRewards: {
			if err := staking.UnpackFromStakingMethod("CollectRewards", args, input); err != nil {
				return nil, err
			}
			// a contract should only collect its own rewards - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "DelegatorAddress")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.CollectRewards{
				DelegatorAddress: address,
			}
			return nil, evm.CollectRewards(evm.StateDB, stakeMsg)
		}
		default: {
			return nil, stakingTypes.ErrInvalidStakingKind
		}
	}
  // return nil, nil -> this never reached
}
