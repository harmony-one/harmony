package staking

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts/abi"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var abiStaking abi.ABI

func init() {
	// for commission rates => solidity does not support floats directly
	// so send commission rates as string
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
	abiStaking, _ = abi.JSON(strings.NewReader(StakingABIJSON))
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
			validatorAddress, err := abi.ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			amount, err := abi.ParseBigIntFromKey(args, "amount")
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
			validatorAddress, err := abi.ParseAddressFromKey(args, "validatorAddress")
			if err != nil {
				return nil, err
			}
			// this type assertion is needed by Golang
			amount, err := abi.ParseBigIntFromKey(args, "amount")
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
	address, err := abi.ParseAddressFromKey(args, key)
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
