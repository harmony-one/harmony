package staking

import (
	"github.com/harmony-one/harmony/crypto"
)

const (
	isValidatorKeyStr = "Harmony/IsValidator/Key/v1"
	isValidatorStr    = "Harmony/IsValidator/Value/v1"
	collectRewardsStr = "Harmony/CollectRewards"
)

// keys used to retrieve staking related informatio
var (
	IsValidatorKey      = crypto.Keccak256Hash([]byte(isValidatorKeyStr))
	IsValidator         = crypto.Keccak256Hash([]byte(isValidatorStr))
	CollectRewardsTopic = crypto.Keccak256Hash([]byte(collectRewardsStr))
)
