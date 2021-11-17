package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// WriteCapablePrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the StakingPrecompileEpoch
// for now, we have only one contract at 252 or 0xfc - which is the staking precompile
var WriteCapablePrecompiledContractsStaking = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// WriteCapablePrecompiledContract represents the interface for Native Go contracts
// which are available as a precompile in the EVM
// As with (read-only) PrecompiledContracts, these need a RequiredGas function
// Note that these contracts have the capability to alter the state
// while those in contracts.go do not
type WriteCapablePrecompiledContract interface {
	// RequiredGas calculates the contract gas use
	RequiredGas(evm *EVM, input []byte) uint64
	// use a different name from read-only contracts to be safe
	RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunWriteCapablePrecompiledContract runs and evaluates the output of a write capable precompiled contract.
func RunWriteCapablePrecompiledContract(p WriteCapablePrecompiledContract, evm *EVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	// immediately error out if readOnly
	if readOnly {
		return nil, errWriteProtection
	}
	gas := p.RequiredGas(evm, input)
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
func (c *stakingPrecompile) RequiredGas(evm *EVM, input []byte) uint64 {
	// If you check the tests, they contain a bunch of successful transactions
	// which are of the type Delegate / Undelegate / CollectRewards
	// these consume anywhere between 22-25k of gas, which is in line with
	// what would happen for a non-EVM staking tx
	// so no additional gas needs to be consumed here
	return 0
	// nb: failed transactions result in consumption of
	// all the gas that was passed, as per core/vm/evm.go
	// if err != ErrExecutionReverted {
	//	contract.UseGas(contract.Gas)
	// }
	// which is why the solidity lib passes 0 as first param
	// instead of the original gas(). if you do so, there will
	// be no gas left for subsequent calls to the precompile
	// (if you make multiple precompile calls in the same tx)
}

// RunWriteCapable runs the actual contract (that is it performs the staking)
func (c *stakingPrecompile) RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	// checks include input, shard, method
	// readOnly has already been checked by RunWriteCapablePrecompiledContract
	// first 32 for size and next 4 for method, at a minimum
	if len(input) < 36 {
		return nil, errors.New("Input is malformed")
	}
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return nil, errors.New("Staking not supported on this shard")
		// we are not shard 0, so no processing
		// but do not fail silently
	}
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
