package staketest

import (
	"fmt"

	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// CheckValidatorEqual checks the equality of validator. If not equal, an
// error is returned
func CheckValidatorEqual(v1, v2 staking.Validator) error {
	if v1.Address != v2.Address {
		return fmt.Errorf(".Address not equal: %x / %x", v1.Address, v2.Address)
	}
	if err := checkPubKeysEqual(v1.SlotPubKeys, v2.SlotPubKeys); err != nil {
		return fmt.Errorf(".SlotPubKeys%v", err)
	}
	if v1.LastEpochInCommittee.Cmp(v2.LastEpochInCommittee) != 0 {
		return fmt.Errorf(".LastEpochInCommittee not equal: %v / %v",
			v1.LastEpochInCommittee, v2.LastEpochInCommittee)
	}
	if v1.MinSelfDelegation.Cmp(v2.MinSelfDelegation) != 0 {
		return fmt.Errorf(".MinSelfDelegation not equal: %v / %v",
			v1.MinSelfDelegation, v2.MinSelfDelegation)
	}
	if v1.MaxTotalDelegation.Cmp(v2.MaxTotalDelegation) != 0 {
		return fmt.Errorf(".MaxTotalDelegation not equal: %v / %v",
			v1.MaxTotalDelegation, v2.MaxTotalDelegation)
	}
	if v1.Status != v2.Status {
		return fmt.Errorf(".Status not equal: %v / %v", v1.Status, v2.Status)
	}
	if err := checkCommissionEqual(v1.Commission, v2.Commission); err != nil {
		return fmt.Errorf(".Commission%v", err)
	}
	if err := checkDescriptionEqual(v1.Description, v2.Description); err != nil {
		return fmt.Errorf(".Description%v", err)
	}
	if v1.CreationHeight.Cmp(v2.CreationHeight) != 0 {
		return fmt.Errorf(".CreationHeight not equal: %v / %v",
			v1.CreationHeight, v2.CreationHeight)
	}
	return nil
}

func checkPubKeysEqual(pubs1, pubs2 []shard.BLSPublicKey) error {
	if len(pubs1) != len(pubs2) {
		return fmt.Errorf(".len not equal: %v / %v", len(pubs1), len(pubs2))
	}
	for i := range pubs1 {
		if pubs1[i] != pubs2[i] {
			return fmt.Errorf("[%v] not equal: %x / %x", i, pubs1[i], pubs2[i])
		}
	}
	return nil
}

func checkDescriptionEqual(d1, d2 staking.Description) error {
	if d1.Name != d2.Name {
		return fmt.Errorf(".Name not equal: %v / %v", d1.Name, d2.Name)
	}
	if d1.Identity != d2.Identity {
		return fmt.Errorf(".Identity not equal: %v / %v", d1.Identity, d2.Identity)
	}
	if d1.Website != d2.Website {
		return fmt.Errorf(".Website not equal: %v / %v", d1.Website, d2.Website)
	}
	if d1.Details != d2.Details {
		return fmt.Errorf(".Details not equal: %v / %v", d1.Details, d2.Details)
	}
	if d1.SecurityContact != d2.SecurityContact {
		return fmt.Errorf(".SecurityContact not equal: %v / %v", d1.SecurityContact, d2.SecurityContact)
	}
	return nil
}

func checkCommissionEqual(c1, c2 staking.Commission) error {
	if err := checkCommissionRateEqual(c1.CommissionRates, c2.CommissionRates); err != nil {
		return fmt.Errorf(".CommissionRate.%v", err)
	}
	if c1.UpdateHeight.Cmp(c2.UpdateHeight) != 0 {
		return fmt.Errorf(".UpdateHeight not equal: %v / %v",
			c1.UpdateHeight, c2.UpdateHeight)
	}
	return nil
}

func checkCommissionRateEqual(cr1, cr2 staking.CommissionRates) error {
	if !cr1.Rate.Equal(cr2.Rate) {
		return fmt.Errorf("Rate not equal: %v / %v", cr1.Rate, cr2.Rate)
	}
	if !cr1.MaxChangeRate.Equal(cr2.MaxChangeRate) {
		return fmt.Errorf("MaxChangeRate not equal: %v / %v",
			cr1.MaxChangeRate, cr2.MaxChangeRate)
	}
	if !cr1.MaxRate.Equal(cr2.MaxRate) {
		return fmt.Errorf("MaxRate not equal: %v / %v", cr1.MaxRate, cr2.MaxRate)
	}
	return nil
}
