package staking

import (
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	isValidatorKeyStr     = "Harmony/IsValidator/Key/v1"
	isValidatorStr        = "Harmony/IsValidator/Value/v1"
	collectRewardsStr     = "Harmony/CollectRewards"
	delegateStr           = "Harmony/Delegate"
	firstElectionEpochStr = "Harmony/FirstElectionEpoch/Key/v1"
)

// keys used to retrieve staking related informatio
var (
	IsValidatorKey        = crypto.Keccak256Hash([]byte(isValidatorKeyStr))
	IsValidator           = crypto.Keccak256Hash([]byte(isValidatorStr))
	CollectRewardsTopic   = crypto.Keccak256Hash([]byte(collectRewardsStr))
	DelegateTopic         = crypto.Keccak256Hash([]byte(delegateStr))
	FirstElectionEpochKey = crypto.Keccak256Hash([]byte(firstElectionEpochStr))
)
