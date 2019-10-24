package staking

import (
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	isValidatorKeyStr = "Harmony/IsValidator/v0"
	isValidatorStr    = "Harmony/IsAValidator/v0"
)

// keys used to retrieve staking related informatio
var (
	IsValidatorKey = crypto.Keccak256Hash([]byte(isValidatorKeyStr))
	IsValidator    = crypto.Keccak256Hash([]byte(isValidatorStr))
)
