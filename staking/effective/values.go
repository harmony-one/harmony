package effective

import "github.com/harmony-one/harmony/numeric"

const (
	// ValidatorsPerShard ..
	ValidatorsPerShard = 400
	// AllValidatorsCount ..
	AllValidatorsCount = ValidatorsPerShard * 4
)

// StakeKeeper ..
type StakeKeeper interface {
	// Activity
	Inventory() struct {
		BLSPublicKeys         [][48]byte    `json:"bls_pubkey"`
		WithDelegationApplied []numeric.Dec `json:"with-delegation-applied,omitempty"`
		// CurrentlyActive       []bool
	}
}
