package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

var WriteCapablePrecompiledContractsStaking = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// Native Go contracts which are available as a precompile in the EVM
// These have the capability to alter the state (those in contracts.go do not)
type WriteCapablePrecompiledContract interface {
	// RequiredGas calculates the contract gas use
	RequiredGas(evm *EVM, input []byte) (uint64, error)
	// use a different name from read-only contracts to be safe
	RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
func RunWriteCapablePrecompiledContract(p WriteCapablePrecompiledContract, evm *EVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	// immediately error out if readOnly
	// do not calculate / consume gas
	if readOnly {
		return nil, errWriteProtection
	}
	// if gas can be calculated
	gas, err := p.RequiredGas(evm, input)
	if err != nil {
		return nil, err
	}
	// only then consume it
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
	if len(input) < 36 { // first 32 for size and next 4 for method, at a minimum
		return 0, errors.New("Input is malformed")
	}
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return 0, errors.New("Staking not supported on this shard")
		// we are not shard 0, so no processing
		// but do not fail silently
	}
	input = input[32:] // drop the word length
	method, err := staking.ParseStakingMethod(input)
	if err != nil {
		return 0, stakingTypes.ErrInvalidStakingKind
	}
	homestead := evm.ChainConfig().IsS3(evm.EpochNumber)
	istanbul := evm.ChainConfig().IsIstanbul(evm.EpochNumber)
	gas, err := IntrinsicGas(input,
		false, /* contractCreation */
		homestead,
		istanbul,
		method.Name == "CreateValidator" /* isValidatorCreation */)
	if err != nil {
		return 0, err
	}
	return gas, nil
}

func (c *stakingPrecompile) RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	// if we are here, we have checked for shard 0,
	// readOnly and length already
	input = input[32:] // drop the initial start of word information
	method, err := staking.ParseStakingMethod(input)
	if err != nil {
		return nil, stakingTypes.ErrInvalidStakingKind
	}
	input = input[4:] // drop the method selector
	// store passed information in map
	args := map[string]interface{}{}
	if err = method.Inputs.UnpackIntoMap(args, input); err != nil {
		return nil, err
	}
	switch method.Name {
	case "CreateValidator":
		{
			// a contract must not make anyone else a validator
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := staking.ParseBigIntFromKey(args, "amount")
			if err != nil {
				return nil, err
			}
			description, err := staking.ParseDescription(args, "description")
			if err != nil {
				return nil, err
			}
			commissionRates, err := staking.ParseCommissionRates(args, "commissionRates")
			if err != nil {
				return nil, err
			}
			minSelfDelegation, err := staking.ParseBigIntFromKey(args, "minSelfDelegation")
			if err != nil {
				return nil, err
			}
			maxTotalDelegation, err := staking.ParseBigIntFromKey(args, "maxTotalDelegation")
			if err != nil {
				return nil, err
			}
			slotPubKeys, err := staking.ParseSlotPubKeys(args, "slotPubKeys")
			if err != nil {
				return nil, err
			}
			slotKeySigs, err := staking.ParseSlotKeySigs(args, "slotKeySigs")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.CreateValidator{
				ValidatorAddress:   address,
				Amount:             amount,
				Description:        description,
				CommissionRates:    commissionRates,
				MinSelfDelegation:  minSelfDelegation,
				MaxTotalDelegation: maxTotalDelegation,
				SlotPubKeys:        slotPubKeys,
				SlotKeySigs:        slotKeySigs,
			}
			if err := evm.CreateValidator(evm.StateDB, stakeMsg); err != nil {
				return nil, err
			} else {
				evm.StakeMsgs = append(evm.StakeMsgs, stakeMsg)
				return nil, nil
			}
		}
	case "EditValidator":
		{
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			description, err := staking.ParseDescription(args, "description")
			if err != nil {
				return nil, err
			}
			commissionRate, err := staking.ParseCommissionRate(args, "commissionRate")
			if err != nil {
				return nil, err
			}
			minSelfDelegation, err := staking.ParseBigIntFromKey(args, "minSelfDelegation")
			if err != nil {
				return nil, err
			}
			maxTotalDelegation, err := staking.ParseBigIntFromKey(args, "maxTotalDelegation")
			if err != nil {
				return nil, err
			}
			slotKeyToRemove, err := staking.ParseSlotPubKeyFromKey(args, "slotKeyToRemove")
			if err != nil {
				return nil, err
			}
			slotKeyToAdd, err := staking.ParseSlotPubKeyFromKey(args, "slotKeyToAdd")
			if err != nil {
				return nil, err
			}
			slotKeyToAddSig, err := staking.ParseSlotKeySigFromKey(args, "slotKeyToAddSig")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.EditValidator{
				ValidatorAddress:   address,
				Description:        description,
				CommissionRate:     commissionRate,
				MinSelfDelegation:  minSelfDelegation,
				MaxTotalDelegation: maxTotalDelegation,
				SlotKeyToRemove:    slotKeyToRemove,
				SlotKeyToAdd:       slotKeyToAdd,
				SlotKeyToAddSig:    slotKeyToAddSig,
			}
			return nil, evm.EditValidator(evm.StateDB, stakeMsg)
		}
	case "Delegate":
		{
			// a contract should only delegate its own balance - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := staking.ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := staking.ParseBigIntFromKey(args, "amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Delegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount:           amount,
			}
			if err := evm.Delegate(evm.StateDB, stakeMsg); err != nil {
				return nil, err
			} else {
				evm.StakeMsgs = append(evm.StakeMsgs, stakeMsg)
				return nil, nil
			}
		}
	case "Undelegate":
		{
			// a contract should only delegate its own balance - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := staking.ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			// this type assertion is needed by Golang
			amount, err := staking.ParseBigIntFromKey(args, "amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Undelegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount:           amount,
			}
			return nil, evm.Undelegate(evm.StateDB, stakeMsg)
		}
	case "CollectRewards":
		{
			// a contract should only collect its own rewards - nobody else's
			address, err := staking.ValidateContractAddress(contract.Caller(), args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.CollectRewards{
				DelegatorAddress: address,
			}
			return nil, evm.CollectRewards(evm.StateDB, stakeMsg)
		}
	default:
		{
			return nil, stakingTypes.ErrInvalidStakingKind
		}
	}
	// return nil, nil -> this never reached
}
