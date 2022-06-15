package vm

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// StateDependentPrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the StakingPrecompileEpoch
var StateDependentPrecompiledContractsStaking = map[common.Address]StateDependentPrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// StateDependentPrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the ROStakingPrecompileEpoch
var StateDependentPrecompiledContractsRoStaking = map[common.Address]StateDependentPrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
	common.BytesToAddress([]byte{250}): &roStakingPrecompile{},
}

// StateDependentPrecompiledContract represents the interface for Native Go contracts
// which are available as a precompile in the EVM
// As with (other) PrecompiledContracts, these need a RequiredGas function
// These contracts may alter the state and/or read from it
type StateDependentPrecompiledContract interface {
	// RequiredGas calculates the contract gas use
	RequiredGas(evm *EVM, contract *Contract, input []byte, readOnly bool) (uint64, error)
	// use a different name from read-only contracts to be safe
	RunStateDependent(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunStateDependentPrecompiledContract runs and evaluates the output of a write capable precompiled contract.
func RunStateDependentPrecompiledContract(
	p StateDependentPrecompiledContract,
	evm *EVM,
	contract *Contract,
	input []byte,
	readOnly bool,
) ([]byte, error) {
	gas, err := p.RequiredGas(evm, contract, input, readOnly)
	if err != nil {
		return nil, err
	}
	if !contract.UseGas(gas) {
		return nil, ErrOutOfGas
	}
	return p.RunStateDependent(evm, contract, input)
}

type stakingPrecompile struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *stakingPrecompile) RequiredGas(
	evm *EVM,
	contract *Contract,
	input []byte,
	readOnly bool,
) (uint64, error) {
	// immediately error out if readOnly
	if readOnly {
		return 0, errWriteProtection
	}
	// if invalid data or invalid shard
	// set payload to blank and charge minimum gas
	var payload []byte = make([]byte, 0)
	// availability of staking and precompile has already been checked
	if evm.Context.ShardID == shard.BeaconChainShardID {
		// check that input is well formed
		// meaning all the expected parameters are available
		// and that we are only trying to perform staking tx
		// on behalf of the correct entity
		stakeMsg, err := staking.ParseStakeMsg(contract.Caller(), input)
		if err == nil {
			// otherwise charge similar to a regular staking tx
			if migrationMsg, ok := stakeMsg.(*stakingTypes.MigrationMsg); ok {
				// charge per delegation to migrate
				return evm.CalculateMigrationGas(evm.StateDB,
					migrationMsg,
					evm.ChainConfig().IsS3(evm.EpochNumber),
					evm.ChainConfig().IsIstanbul(evm.EpochNumber),
				)
			} else if encoded, err := rlp.EncodeToBytes(stakeMsg); err == nil {
				payload = encoded
			}
		}
	}
	if gas, err := IntrinsicGas(
		payload,
		false,                                   // contractCreation
		evm.ChainConfig().IsS3(evm.EpochNumber), // homestead
		evm.ChainConfig().IsIstanbul(evm.EpochNumber), // istanbul
		false, // isValidatorCreation
	); err != nil {
		return 0, err // ErrOutOfGas occurs when gas payable > uint64
	} else {
		return gas, nil
	}
}

// RunStateDependent runs the actual contract (that is it performs the staking)
func (c *stakingPrecompile) RunStateDependent(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return nil, errors.New("precompile is non-payable")
	}
	// after the ro staking precompile fork, ensure msg.value is not sent
	// the other option was to enforce sending msg.value, but that does
	// not work well with redelegation of locked tockens
	if evm.chainRules.IsROStakingPrecompile {
		if contract.Value().Cmp(common.Big0) > 0 {
			return nil, errors.New("this precompile cannot accept funds")
		}
	}
	stakeMsg, err := staking.ParseStakeMsg(contract.Caller(), input)
	if err != nil {
		return nil, err
	}

	var rosettaBlockTracer RosettaTracer
	if tmpTracker, ok := evm.vmConfig.Tracer.(RosettaTracer); ok {
		rosettaBlockTracer = tmpTracker
	}

	if delegate, ok := stakeMsg.(*stakingTypes.Delegate); ok {
		if err := evm.Delegate(evm.StateDB, rosettaBlockTracer, delegate); err != nil {
			return nil, err
		} else {
			evm.StakeMsgs = append(evm.StakeMsgs, delegate)
			return nil, nil
		}
	}
	if undelegate, ok := stakeMsg.(*stakingTypes.Undelegate); ok {
		return nil, evm.Undelegate(evm.StateDB, rosettaBlockTracer, undelegate)
	}
	if collectRewards, ok := stakeMsg.(*stakingTypes.CollectRewards); ok {
		return nil, evm.CollectRewards(evm.StateDB, rosettaBlockTracer, collectRewards)
	}
	// Migrate is not supported in precompile and will be done in a batch hard fork
	//if migrationMsg, ok := stakeMsg.(*stakingTypes.MigrationMsg); ok {
	//	stakeMsgs, err := evm.MigrateDelegations(evm.StateDB, migrationMsg)
	//	if err != nil {
	//		return nil, err
	//	} else {
	//		for _, stakeMsg := range stakeMsgs {
	//			if delegate, ok := stakeMsg.(*stakingTypes.Delegate); ok {
	//				evm.StakeMsgs = append(evm.StakeMsgs, delegate)
	//			} else {
	//				return nil, errors.New("[StakingPrecompile] Received incompatible stakeMsg from evm.MigrateDelegations")
	//			}
	//		}
	//		return nil, nil
	//	}
	//}
	return nil, errors.New("[StakingPrecompile] Received incompatible stakeMsg from staking.ParseStakeMsg")
}

type roStakingPrecompile struct{}

func (c *roStakingPrecompile) RequiredGas(
	evm *EVM,
	_ *Contract,
	input []byte,
	_ bool,
) (uint64, error) {
	return evm.RoStakingGas(evm.StateDB, input)
}

func (c *roStakingPrecompile) RunStateDependent(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	if contract.Value().Cmp(common.Big0) != 0 {
		return nil, errors.New("precompile is non payable")
	}
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return nil, errors.New("staking not supported on this shard")
	}
	roStakeMsg, err := staking.ParseReadOnlyStakeMsg(input)
	if err != nil {
		return nil, err
	}
	return evm.RoStakingInfo(evm.StateDB, roStakeMsg)
}
