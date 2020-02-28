package effective

import (
	staking "github.com/harmony-one/harmony/staking/types"
)

// IsEligibleForEPOSAuction ..
func IsEligibleForEPOSAuction(v *staking.ValidatorWrapper) bool {
	return v.Active && !v.Banned
}
