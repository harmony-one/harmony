package staking

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts/abi"
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
	abiStaking, _ = abi.JSON(strings.NewReader(StakingABIJSON))
}

func ParseStakingMethod(input []byte) (*abi.Method, error) {
	return abiStaking.MethodById(input)
}

// used to ensure caller == delegatorAddress
func ValidateContractAddress(contractCaller common.Address, args map[string]interface{}, key string) (common.Address, error) {
	address, err := ParseAddressFromKey(args, key)
	if err != nil {
		return common.Address{}, err
	}
	if contractCaller != address {
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
	if byteAddress, ok := args[key].([]byte); ok {
		address := common.BytesToAddress(byteAddress)
		return address, nil
	} else if address, ok := args[key].(common.Address); ok {
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
