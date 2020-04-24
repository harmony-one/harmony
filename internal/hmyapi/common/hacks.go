package common

import (
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
)

type setRawStakeHack interface {
	SetRawStake(oneAddr common.Address, d numeric.Dec)
}

// SetRawStake is a hack, return value is if was successful or not at setting
func SetRawStake(q quorum.Decider, oneAddr common.Address, d numeric.Dec) bool {
	if setter, ok := q.(setRawStakeHack); ok {
		setter.SetRawStake(oneAddr, d)
		return true
	}
	return false
}
