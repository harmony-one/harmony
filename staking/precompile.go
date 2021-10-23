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
	    "name": "CollectRewards",
	    "outputs": [],
	    "inputs": [
	      {
	        "type": "uint8",
	        "name": "DirectiveUnused",
	        "indexed": false
	      },
	      {
	        "type": "address",
	        "name": "DelegatorAddress",
	        "indexed": false
	      }
	    ],
	    "constant": false,
	    "payable": false,
	    "type": "function"
	  },
	  {
	    "name": "DelegateOrUndelegate",
	    "outputs": [],
	    "inputs": [
	      {
	        "type": "uint8",
	        "name": "DirectiveUnused",
	        "indexed": false
	      },
	      {
	        "type": "address",
	        "name": "DelegatorAddress",
	        "indexed": false
	      },
	      {
	        "type": "address",
	        "name": "ValidatorAddress",
	        "indexed": false
	      },
	      {
	        "type": "uint256",
	        "name": "Amount",
	        "indexed": false
	      }
	    ],
	    "constant": false,
	    "payable": false,
	    "type": "function"
	  },
	  {
	    "name": "CreateValidator",
	    "outputs": [],
	    "inputs": [
	      {
	        "type": "uint8",
	        "name": "DirectiveUnused",
	        "indexed": false
	      },
	      {
	        "name": "ValidatorAddress",
	        "type": "address",
	        "indexed": false
	      },
	      {
	        "components": [
	          {
	            "name": "Name",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Identity",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Website",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "SecurityContact",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Details",
	            "type": "string",
	            "indexed": false
	          }
	        ],
	        "name": "Description",
	        "type": "tuple",
	        "indexed": false
	      },
	      {
	        "components": [
	          {
	            "name": "Rate",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "MaxRate",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "MaxChangeRate",
	            "type": "string",
	            "indexed": false
	          }
	        ],
	        "name": "CommissionRates",
	        "type": "tuple",
	        "indexed": false
	      },
	      {
	        "name": "MinSelfDelegation",
	        "type": "uint256",
	        "indexed": false
	      },
	      {
	        "name": "MaxTotalDelegation",
	        "type": "uint256",
	        "indexed": false
	      },
	      {
	        "name": "SlotPubKeys",
	        "type": "bytes[]",
	        "indexed": false
	      },
				{
	        "name": "SlotKeySigs",
	        "type": "bytes[]",
	        "indexed": false
	      },
	      {
	        "name": "Amount",
	        "type": "uint256",
	        "indexed": false
	      }
	    ],
	    "constant": false,
	    "payable": false,
	    "type": "function"
	  },
	  {
	    "name": "EditValidator",
	    "outputs": [],
	    "inputs": [
	      {
	        "type": "uint8",
	        "name": "DirectiveUnused",
	        "indexed": false
	      },
	      {
	        "name": "ValidatorAddress",
	        "type": "address",
	        "indexed": false
	      },
	      {
	        "components": [
	          {
	            "name": "Name",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Identity",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Website",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "SecurityContact",
	            "type": "string",
	            "indexed": false
	          },
	          {
	            "name": "Details",
	            "type": "string",
	            "indexed": false
	          }
	        ],
	        "name": "Description",
	        "type": "tuple",
	        "indexed": false
	      },
	      {
	        "name": "CommissionRate",
	        "type": "string",
	        "indexed": false
	      },
	      {
	        "name": "MinSelfDelegation",
	        "type": "uint256",
	        "indexed": false
	      },
	      {
	        "name": "MaxTotalDelegation",
	        "type": "uint256",
	        "indexed": false
	      },
	      {
	        "name": "SlotKeyToRemove",
	        "type": "bytes",
	        "indexed": false
	      },
	      {
	        "name": "SlotKeyToAdd",
	        "type": "bytes",
	        "indexed": false
	      },
				{
	        "name": "SlotKeyToAddSig",
	        "type": "bytes",
	        "indexed": false
	      }
	    ],
	    "constant": false,
	    "payable": false,
	    "type": "function"
	  }
	]
	`
	abiStaking, _ = abi.JSON(strings.NewReader(StakingABIJSON))
}

func UnpackFromStakingMethod(methodName string, args map[string]interface{}, input []byte) error {
	if method, ok := abiStaking.Methods[methodName]; ok {
		err := method.Inputs.UnpackIntoMap(args, input)
		return err
	} else {
		// this should never happen, unless you make a typo in the precompiled code
		return errors.Errorf("Key %s is not an ABI method", methodName)
	}
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

func ParseDescription(args map[string]interface{}) (stakingTypes.Description, error) {
	// mostly the same struct as stakingTypes.Description
	// except the JSON tag for SecurityContact is security-contact there
	// so type assert it through this one
	if description, ok := args["Description"].(struct {
		Name            string "json:\"Name\""
		Identity        string "json:\"Identity\""
		Website         string "json:\"Website\""
		SecurityContact string "json:\"SecurityContact\""
		Details         string "json:\"Details\""
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
			"Cannot parse Description from %v", args["Description"])
	}
}

func ParseCommissionRates(args map[string]interface{}) (stakingTypes.CommissionRates, error) {
	// CommissionRates here is parsed as containing 3 strings
	// whereas the actual stakingTypes.CommissionRates has numeric.Dec
	// create a new structure on the fly to allow type assertion
	if commissionRates, ok := args["CommissionRates"].(struct {
		Rate          string "json:\"Rate\""
		MaxRate       string "json:\"MaxRate\""
		MaxChangeRate string "json:\"MaxChangeRate\""
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
			"Cannot parse CommissionRates from %v", args["CommissionRates"])
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

func ParseSlotPubKeys(args map[string]interface{}) ([]bls.SerializedPublicKey, error) {
	// cast it into bytes
	pubKeys, ok := args["SlotPubKeys"].([][]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotPubKeys from %v", args["SlotPubKeys"])
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

func ParseSlotKeySigs(args map[string]interface{}) ([]bls.SerializedSignature, error) {
	// cast it into bytes
	sigs, ok := args["SlotKeySigs"].([][]byte)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse SlotKeySigs from %v", args["SlotKeySigs"])
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

func ParseCommissionRate(args map[string]interface{}) (*numeric.Dec, error) {
	// expect string
	commissionRate, ok := args["CommissionRate"].(string)
	if !ok {
		return nil, errors.Errorf(
			"Cannot parse CommissionRate from %v", args["CommissionRate"])
	} else {
		rate, err := numeric.NewDecFromStr(commissionRate)
		if err != nil {
			return nil, err
		} else {
			return &rate, nil
		}
	}
}
