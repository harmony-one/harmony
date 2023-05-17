package vm

import (
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/accounts/abi"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// WriteCapablePrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the StakingPrecompileEpoch
// for now, we have only one contract at 252 or 0xfc - which is the staking precompile
var WriteCapablePrecompiledContractsStaking = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// WriteCapablePrecompiledContractsCrossXfer lists out the write capable precompiled contracts
// which are available after the CrossShardXferPrecompileEpoch
// It includes the staking precompile and the cross-shard transfer precompile
var WriteCapablePrecompiledContractsCrossXfer = map[common.Address]WriteCapablePrecompiledContract{
	// reserve 250 for read only staking precompile and 251 for epoch
	common.BytesToAddress([]byte{249}): &crossShardXferPrecompile{},
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
}

// WriteCapablePrecompiledContract represents the interface for Native Go contracts
// which are available as a precompile in the EVM
// As with (read-only) PrecompiledContracts, these need a RequiredGas function
// Note that these contracts have the capability to alter the state
// while those in contracts.go do not
type WriteCapablePrecompiledContract interface {
	// RequiredGas calculates the contract gas use
	RequiredGas(evm *EVM, contract *Contract, input []byte) (uint64, error)
	// use a different name from read-only contracts to be safe
	RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunWriteCapablePrecompiledContract runs and evaluates the output of a write capable precompiled contract.
func RunWriteCapablePrecompiledContract(
	p WriteCapablePrecompiledContract,
	evm *EVM,
	contract *Contract,
	input []byte,
	readOnly bool,
) ([]byte, error) {
	// immediately error out if readOnly
	if readOnly {
		return nil, errWriteProtection
	}
	gas, err := p.RequiredGas(evm, contract, input)
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
func (c *stakingPrecompile) RequiredGas(
	evm *EVM,
	contract *Contract,
	input []byte,
) (uint64, error) {
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

// RunWriteCapable runs the actual contract (that is it performs the staking)
func (c *stakingPrecompile) RunWriteCapable(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return nil, errors.New("Staking not supported on this shard")
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

var abiCrossShardXfer abi.ABI

func init() {
	// msg.Value is used for transfer and is also a parameter
	// otherwise it might be possible for a user to retrieve money from the precompile
	// that was sent by someone else prior to the hard fork
	// contract.Caller is used as fromAddress, not a parameter
	// originating ShardID is pulled from the EVM object, not a parameter
	crossShardXferABIJSON := `
	[
	  {
	    "inputs": [
		  {
	        "internalType": "uint256",
	        "name": "value",
	        "type": "uint256"
	      },
		  {
	        "internalType": "address",
	        "name": "to",
	        "type": "address"
	      },
		  {
	        "internalType": "uint64",
	        "name": "toShardID",
	        "type": "uint32"
	      }
	    ],
	    "name": "crossShardTransfer",
	    "outputs": [],
	    "stateMutability": "payable",
	    "type": "function"
	  }
	]`
	var err error
	abiCrossShardXfer, err = abi.JSON(strings.NewReader(crossShardXferABIJSON))
	if err != nil {
		// means an error in the code
		panic("Invalid cross shard transfer ABI JSON")
	}
}

type crossShardXferPrecompile struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *crossShardXferPrecompile) RequiredGas(
	evm *EVM,
	contract *Contract,
	input []byte,
) (uint64, error) {
	// multiple instances of the precompile in one transaction
	// are blocked, so there is no way for a smart contract
	// to subsidize this transaction by an EOA via delegatecall
	// therefore no need to charge any gas
	return 0, nil
}

// RunWriteCapable runs the actual contract
func (c *crossShardXferPrecompile) RunWriteCapable(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	// make sure that cxReceipt is already nil to
	// prevent multiple calls to the precompile
	// in the same transaction
	if evm.CXReceipt != nil {
		return nil, errors.New("cannot call cross shard precompile again in same tx")
	}
	fromAddress, toAddress, fromShardID, toShardID, value, err :=
		parseCrossShardXferData(evm, contract, input)
	if err != nil {
		return nil, err
	}
	// validate not a contract (toAddress can still be a contract)
	if len(evm.StateDB.GetCode(fromAddress)) > 0 && !evm.IsValidator(evm.StateDB, fromAddress) {
		return nil, errors.New("cross shard xfer not yet implemented for contracts")
	}
	// can't have too many shards
	if toShardID >= evm.Context.NumShards {
		return nil, errors.New("toShardId out of bounds")
	}
	// not for simple transfers
	if fromShardID == toShardID {
		return nil, errors.New("from and to shard id can't be equal")
	}
	// make sure nobody sends extra or less money
	if contract.Value().Cmp(value) != 0 {
		return nil, errors.New("argument value and msg.value not equal")
	}
	// now do the actual transfer
	// step 1 -> remove funds from the precompile address
	if !evm.CanTransfer(evm.StateDB, contract.Address(), value) {
		return nil, errors.New("not enough balance received")
	}
	evm.Transfer(evm.StateDB, contract.Address(), toAddress, value, types.SubtractionOnly)
	// step 2 -> make a cross link
	// note that the transaction hash is added by state_processor.go to this receipt
	// and that the receiving shard does not care about the `From` but we use the original
	// instead of the precompile address for consistency
	evm.CXReceipt = &types.CXReceipt{
		From:      fromAddress,
		To:        &toAddress,
		ShardID:   fromShardID,
		ToShardID: toShardID,
		Amount:    value,
	}
	return nil, nil
}

// parseCrossShardXferData does a simple parse with only data types validation
func parseCrossShardXferData(evm *EVM, contract *Contract, input []byte) (
	common.Address, common.Address, uint32, uint32, *big.Int, error) {
	method, err := abiCrossShardXfer.MethodById(input)
	if err != nil {
		return common.Address{}, common.Address{}, 0, 0, nil, err
	}
	input = input[4:]
	args := map[string]interface{}{}
	if err = method.Inputs.UnpackIntoMap(args, input); err != nil {
		return common.Address{}, common.Address{}, 0, 0, nil, err
	}
	value, err := abi.ParseBigIntFromKey(args, "value")
	if err != nil {
		return common.Address{}, common.Address{}, 0, 0, nil, err
	}
	toAddress, err := abi.ParseAddressFromKey(args, "to")
	if err != nil {
		return common.Address{}, common.Address{}, 0, 0, nil, err
	}
	toShardID, err := abi.ParseUint32FromKey(args, "toShardID")
	if err != nil {
		return common.Address{}, common.Address{}, 0, 0, nil, err
	}
	return contract.Caller(), toAddress, evm.ShardID, toShardID, value, nil
}
