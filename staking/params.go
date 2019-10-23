package staking

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	validatorMapKeyStr = "Harmony/StakingValidatorMapKey/v0"
	isValidatorKeyStr  = "Harmony/IsValidator/v0"
	isValidatorStr     = "Harmony/IsAValidator/v0"
	isNotValidatorStr  = "Harmony/IsNotAValidator/v0"
)

// keys used to retrieve staking related informatio
var (
	validatorMapKey     = crypto.Keccak256Hash([]byte(validatorMapKeyStr))
	ValidatorMapAddress = common.BytesToAddress(validatorMapKey[12:])
	IsValidatorKey      = crypto.Keccak256Hash([]byte(isValidatorKeyStr))
	IsValidator         = crypto.Keccak256Hash([]byte(isValidatorStr))
	IsNotValidator      = crypto.Keccak256Hash([]byte(isNotValidatorStr))
)
