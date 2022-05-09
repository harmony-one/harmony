package staking

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts/abi"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var abiStaking abi.ABI
var abiRoStaking abi.ABI

func init() {
	StakingABIJSON := `
	[
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "delegatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "CollectRewards",
	    "outputs": [],
	    "stateMutability": "nonpayable",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "delegatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "uint256",
	        "name": "amount",
	        "type": "uint256"
	      }
	    ],
	    "name": "Delegate",
	    "outputs": [],
	    "stateMutability": "nonpayable",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "delegatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "uint256",
	        "name": "amount",
	        "type": "uint256"
	      }
	    ],
	    "name": "Undelegate",
	    "outputs": [],
	    "stateMutability": "nonpayable",
	    "type": "function"
	  }
	]
	`

	// ABI for migrate which is not enabled for precompile
	//,
	//{
	//	"inputs": [
	//{
	//"internalType": "address",
	//"name": "from",
	//"type": "address"
	//},
	//{
	//"internalType": "address",
	//"name": "to",
	//"type": "address"
	//}
	//],
	//"name": "Migrate",
	//"outputs": [],
	//"stateMutability": "nonpayable",
	//"type": "function"
	//}
	ReadOnlyStakingABIJSON := `
	[
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "delegatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "getBalanceAvailableForRedelegation",
	    "outputs": [
	      {
	        "internalType": "uint256",
	        "name": "",
	        "type": "uint256"
	      }
	    ],
	    "stateMutability": "view",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "delegatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "getDelegationByDelegatorAndValidator",
	    "outputs": [
	      {
	        "internalType": "uint256",
	        "name": "",
	        "type": "uint256"
	      }
	    ],
	    "stateMutability": "view",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      },
	      {
	        "internalType": "uint256",
	        "name": "blockNumber",
	        "type": "uint256"
	      }
	    ],
	    "name": "getSlashingHeightFromBlockForValidator",
	    "outputs": [
          {
            "internalType": "uint256",
            "name": "",
            "type": "uint256"
          }
        ],
	    "stateMutability": "view",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "getValidatorCommissionRate",
	    "outputs": [
	      {
	        "internalType": "uint256",
	        "name": "",
	        "type": "uint256"
	      }
	    ],
	    "stateMutability": "view",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "getValidatorMaxTotalDelegation",
	    "outputs": [
	      {
	        "internalType": "uint256",
	        "name": "",
	        "type": "uint256"
	      }
	    ],
	    "stateMutability": "view",
	    "type": "function"
	  },
	  {
	    "inputs": [
	      {
	        "internalType": "address",
	        "name": "validatorAddress",
	        "type": "address"
	      }
	    ],
	    "name": "getValidatorTotalDelegation",
	    "outputs": [
	      {
	        "internalType": "uint256",
	        "name": "",
	        "type": "uint256"
	      }
	    ],
	    "stateMutability": "view",
	    "type": "function"
	  }
	]
	`
	abiStaking, _ = abi.JSON(strings.NewReader(StakingABIJSON))
	var err error
	abiRoStaking, err = abi.JSON(strings.NewReader(ReadOnlyStakingABIJSON))
	if err != nil {
		panic(err)
	}
}

// contractCaller (and not Contract) is used here to avoid import cycle
func ParseStakeMsg(contractCaller common.Address, input []byte) (interface{}, error) {
	method, err := abiStaking.MethodById(input)
	if err != nil {
		return nil, err
	}
	input = input[4:]                // drop the method selector
	args := map[string]interface{}{} // store into map
	if err = method.Inputs.UnpackIntoMap(args, input); err != nil {
		return nil, err
	}
	switch method.Name {
	case "Delegate":
		{
			// in case of assembly call, a contract will delegate its own balance
			// in case of assembly delegatecall, contract.Caller() is msg.sender
			// which means an EOA can
			// (1) deploy a contract which receives delegations and amounts
			// (2) call the contract, which then performs the tx on behalf of the EOA
			address, err := ValidateContractAddress(contractCaller, args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := ParseBigIntFromKey(args, "amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Delegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount:           amount,
			}
			return stakeMsg, nil
		}
	case "Undelegate":
		{
			// same validation as above
			address, err := ValidateContractAddress(contractCaller, args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			validatorAddress, err := ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			// this type assertion is needed by Golang
			amount, err := ParseBigIntFromKey(args, "amount")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.Undelegate{
				DelegatorAddress: address,
				ValidatorAddress: validatorAddress,
				Amount:           amount,
			}
			return stakeMsg, nil
		}
	case "CollectRewards":
		{
			// same validation as above
			address, err := ValidateContractAddress(contractCaller, args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			stakeMsg := &stakingTypes.CollectRewards{
				DelegatorAddress: address,
			}
			return stakeMsg, nil
		}
	//case "Migrate":
	//	{
	//		from, err := ValidateContractAddress(contractCaller, args, "from")
	//		if err != nil {
	//			return nil, err
	//		}
	//		to, err := ParseAddressFromKey(args, "to")
	//		if err != nil {
	//			return nil, err
	//		}
	//		// no sanity check for migrating to same address, just do nothing
	//		return &stakingTypes.MigrationMsg{
	//			From: from,
	//			To:   to,
	//		}, nil
	//	}
	default:
		{
			return nil, errors.New("[StakingPrecompile] Invalid method name from ABI selector")
		}
	}
}

// used to ensure caller == delegatorAddress
func ValidateContractAddress(contractCaller common.Address, args map[string]interface{}, key string) (common.Address, error) {
	address, err := ParseAddressFromKey(args, key)
	if err != nil {
		return common.Address{}, err
	}
	if !bytes.Equal(contractCaller.Bytes(), address.Bytes()) {
		return common.Address{}, errors.Errorf(
			"[StakingPrecompile] Address mismatch, expected %s have %s",
			contractCaller.String(), address.String(),
		)
	} else {
		return address, nil
	}
}

// used for both delegatorAddress and validatorAddress
func ParseAddressFromKey(args map[string]interface{}, key string) (common.Address, error) {
	if address, ok := args[key].(common.Address); ok {
		return address, nil
	} else {
		return common.Address{}, errors.Errorf("Cannot parse address from %v", args[key])
	}
}

// used for amounts
func ParseBigIntFromKey(args map[string]interface{}, key string) (*big.Int, error) {
	bigInt, ok := args[key].(*big.Int)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse BigInt from %v", args[key])
	} else {
		return bigInt, nil
	}
}

func ParseUint64FromKey(args map[string]interface{}, key string) (uint64, error) {
	value, ok := args[key].(uint64)
	if !ok {
		return 0, errors.Errorf(
			"Cannot parse uint64 from %v", args[key])
	} else {
		return value, nil
	}
}

func ParseReadOnlyStakeMsg(input []byte) (*stakingTypes.ReadOnlyStakeMsg, error) {
	method, err := abiRoStaking.MethodById(input)
	if err != nil {
		return nil, err
	}
	input = input[4:]                // drop the method selector
	args := map[string]interface{}{} // store into map
	if err = method.Inputs.UnpackIntoMap(args, input); err != nil {
		return nil, err
	}
	switch method.Name {
	case "getDelegationByDelegatorAndValidator":
		{
			delegatorAddress, err := ParseAddressFromKey(args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			msg, err := makeRoStakeMsgWithValidator(
				args,
				method.Name[3:],
			)
			if err != nil {
				return nil, err
			}
			msg.DelegatorAddress = delegatorAddress
			return msg, nil
		}
	case "getValidatorCommissionRate":
		return makeRoStakeMsgWithValidator(args, method.Name[3:])
	case "getValidatorMaxTotalDelegation":
		return makeRoStakeMsgWithValidator(args, method.Name[3:])
	case "getValidatorTotalDelegation":
		return makeRoStakeMsgWithValidator(args, method.Name[3:])
	case "getSlashingHeightFromBlockForValidator":
		msg, err := makeRoStakeMsgWithValidator(
			args,
			method.Name[3:],
		)
		if err != nil {
			return nil, err
		}
		blockNumber, err := ParseBigIntFromKey(args, "blockNumber")
		if err != nil {
			return nil, err
		}
		msg.BlockNumber = blockNumber
		return msg, nil
	case "getBalanceAvailableForRedelegation":
		{
			delegatorAddress, err := ParseAddressFromKey(args, "delegatorAddress")
			if err != nil {
				return nil, err
			}
			return &stakingTypes.ReadOnlyStakeMsg{
				DelegatorAddress: delegatorAddress,
				What:             method.Name[3:],
			}, nil
		}
	}
	return nil, errors.New("invalid method name")
}

func makeRoStakeMsgWithValidator(args map[string]interface{}, what string) (
	*stakingTypes.ReadOnlyStakeMsg, error,
) {
	validatorAddress, err := ParseAddressFromKey(args, "validatorAddress")
	if err != nil {
		return nil, err
	}
	return &stakingTypes.ReadOnlyStakeMsg{
		ValidatorAddress: validatorAddress,
		What:             what,
	}, nil
}
