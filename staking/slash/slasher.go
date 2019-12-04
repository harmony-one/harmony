package slash

import (
	"github.com/harmony-one/harmony/shard"
)

var (
	// MissedThresholdForInactive ..
	MissedThresholdForInactive = shard.Schedule.BlocksPerEpoch()
)

// Slasher ..
type Slasher interface {
	ShouldSlash(shard.BlsPublicKey) bool
}

// ThresholdDecider ..
type ThresholdDecider interface {
	SlashThresholdMet(shard.BlsPublicKey) bool
}

// Delinquent ..
type Delinquent struct {
	// Count is number of times this key did not sign
	Count uint64
	// MIA is the missing in action key
	MIA shard.BlsPublicKey
}

// DelinquentPerEpoch ..
type DelinquentPerEpoch struct {
	Epoch   uint64
	Missing []Delinquent
}

// Reset sets the missing count to 0 for a bls public key
func (d DelinquentPerEpoch) Reset(signer shard.BlsPublicKey) {
	for i := range d.Missing {
		if signer.Equal(d.Missing[i].MIA) {
			d.Missing[i].Count = 0
			return
		}
	}
}

// Increment increases the count, return if was able to bump the offender's count up
func (d DelinquentPerEpoch) Increment(mia shard.BlsPublicKey) bool {
	for i := range d.Missing {
		if mia.Equal(d.Missing[i].MIA) {
			d.Missing[i].Count++
			return true
		}
	}
	return false
}
