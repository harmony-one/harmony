package staking

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts/abi"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
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
	        "name": "validatorAddress",
	        "type": "address"
	      },
	      {
	        "components": [
	          {
	            "internalType": "string",
	            "name": "name",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "identity",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "website",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "securityContact",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "details",
	            "type": "string"
	          }
	        ],
	        "internalType": "struct Description",
	        "name": "description",
	        "type": "tuple"
	      },
	      {
	        "components": [
	          {
	            "internalType": "string",
	            "name": "rate",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "maxRate",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "maxChangeRate",
	            "type": "string"
	          }
	        ],
	        "internalType": "struct CommissionRate",
	        "name": "commissionRates",
	        "type": "tuple"
	      },
	      {
	        "internalType": "uint256",
	        "name": "minSelfDelegation",
	        "type": "uint256"
	      },
	      {
	        "internalType": "uint256",
	        "name": "maxTotalDelegation",
	        "type": "uint256"
	      },
	      {
	        "internalType": "bytes[]",
	        "name": "slotPubKeys",
	        "type": "bytes[]"
	      },
	      {
	        "internalType": "bytes[]",
	        "name": "slotKeySigs",
	        "type": "bytes[]"
	      },
	      {
	        "internalType": "uint256",
	        "name": "amount",
	        "type": "uint256"
	      }
	    ],
	    "name": "CreateValidator",
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
	        "name": "validatorAddress",
	        "type": "address"
	      },
	      {
	        "components": [
	          {
	            "internalType": "string",
	            "name": "name",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "identity",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "website",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "securityContact",
	            "type": "string"
	          },
	          {
	            "internalType": "string",
	            "name": "details",
	            "type": "string"
	          }
	        ],
	        "internalType": "struct Description",
	        "name": "description",
	        "type": "tuple"
	      },
	      {
	        "internalType": "string",
	        "name": "commissionRate",
	        "type": "string"
	      },
	      {
	        "internalType": "uint256",
	        "name": "minSelfDelegation",
	        "type": "uint256"
	      },
	      {
	        "internalType": "uint256",
	        "name": "maxTotalDelegation",
	        "type": "uint256"
	      },
	      {
	        "internalType": "bytes",
	        "name": "slotKeyToRemove",
	        "type": "bytes"
	      },
	      {
	        "internalType": "bytes",
	        "name": "slotKeyToAdd",
	        "type": "bytes"
	      },
	      {
	        "internalType": "bytes",
	        "name": "slotKeyToAddSig",
	        "type": "bytes"
	      }
	    ],
	    "name": "EditValidator",
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

func ParseDescription(args map[string]interface{}, key string) (stakingTypes.Description, error) {
	// mostly the same struct as stakingTypes.Description
	// except the JSON tag for SecurityContact is security-contact there
	// so type assert it through this one
	if description, ok := args[key].(struct {
		Name            string "json:\"name\""
		Identity        string "json:\"identity\""
		Website         string "json:\"website\""
		SecurityContact string "json:\"securityContact\""
		Details         string "json:\"details\""
	}); ok {
		return stakingTypes.Description{
			description.Name,
			description.Identity,
			description.Website,
			description.SecurityContact,
			description.Details,
		}, nil
	} else {
		return stakingTypes.Description{}, errors.Errorf(
			"Cannot parse Description from %v", args[key])
	}
}

func ParseCommissionRates(args map[string]interface{}, key string) (stakingTypes.CommissionRates, error) {
	// CommissionRates here is parsed as containing 3 strings
	// whereas the actual stakingTypes.CommissionRates has numeric.Dec
	// create a new structure on the fly to allow type assertion
	if commissionRates, ok := args[key].(struct {
		Rate          string "json:\"rate\""
		MaxRate       string "json:\"maxRate\""
		MaxChangeRate string "json:\"maxChangeRate\""
	}); ok {
		rate, err := numeric.NewDecFromStr(commissionRates.Rate)
		if err != nil {
			return stakingTypes.CommissionRates{}, err
		}
		maxRate, err := numeric.NewDecFromStr(commissionRates.MaxRate)
		if err != nil {
			return stakingTypes.CommissionRates{}, err
		}
		maxChangeRate, err := numeric.NewDecFromStr(commissionRates.MaxChangeRate)
		if err != nil {
			return stakingTypes.CommissionRates{}, err
		}
		return stakingTypes.CommissionRates{rate, maxRate, maxChangeRate}, nil
	} else {
		return stakingTypes.CommissionRates{}, errors.Errorf(
			"Cannot parse CommissionRates from %v", args[key])
	}
}

func ParseBigIntFromKey(args map[string]interface{}, key string) (*big.Int, error) {
	bigInt, ok := args[key].(*big.Int)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse BigInt from %v", args[key])
	} else {
		return bigInt, nil
	}
}

func ParseSlotPubKeys(args map[string]interface{}, key string) ([]bls.SerializedPublicKey, error) {
	// cast it into bytes
	pubKeys, ok := args[key].([][]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotPubKeys from %v", args[key])
	}
	result := make([]bls.SerializedPublicKey, len(pubKeys))
	for i, pubKey := range pubKeys {
		if serializedKey, err := bls.BytesToSerializedPublicKey(pubKey); err != nil {
			return nil, err
		} else {
			result[i] = serializedKey
		}
	}
	return result, nil
}

func ParseSlotKeySigs(args map[string]interface{}, key string) ([]bls.SerializedSignature, error) {
	// cast it into bytes
	sigs, ok := args[key].([][]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotKeySigs from %v", args[key])
	}
	result := make([]bls.SerializedSignature, len(sigs))
	for i, sig := range sigs {
		if serializedSig, err := bls.BytesToSerializedSignature(sig); err != nil {
			return nil, err
		} else {
			result[i] = serializedSig
		}
	}
	return result, nil
}

func ParseSlotPubKeyFromKey(args map[string]interface{}, key string) (*bls.SerializedPublicKey, error) {
	// convert to bytes
	pubKeyBytes, ok := args[key].([]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotPubKey from %v", args[key])
	}
	if result, err := bls.BytesToSerializedPublicKey(pubKeyBytes); err == nil {
		return &result, nil
	} else {
		return nil, err
	}
}

func ParseSlotKeySigFromKey(args map[string]interface{}, key string) (*bls.SerializedSignature, error) {
	// convert to bytes
	sigBytes, ok := args[key].([]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotKeySig from %v", args[key])
	}
	if result, err := bls.BytesToSerializedSignature(sigBytes); err == nil {
		return &result, nil
	} else {
		return nil, err
	}
}

func ParseCommissionRate(args map[string]interface{}, key string) (*numeric.Dec, error) {
	// expect string
	commissionRate, ok := args[key].(string)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse CommissionRate from %v", args[key])
	} else {
		rate, err := numeric.NewDecFromStr(commissionRate)
		if err != nil {
			return nil, err
		} else {
			return &rate, nil
		}
	}
}
