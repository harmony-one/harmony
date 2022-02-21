package staketest

import (
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
)

// CheckValidatorWrapperEqual checks the equality of staking.ValidatorWrapper. If not equal, an
// error is returned. Note nil pointer is treated as zero in this compare function.
func CheckValidatorWrapperEqual(w1, w2 staking.ValidatorWrapper) error {
	if err := checkValidatorWrapperEqual(w1, w2); err != nil {
		return fmt.Errorf("wrapper%v", err)
	}
	return nil
}

// CheckValidatorEqual checks the equality of validator. If not equal, an
// error is returned. Note nil pointer is treated as zero in this compare function.
func CheckValidatorEqual(v1, v2 staking.Validator) error {
	if err := checkValidatorEqual(v1, v2); err != nil {
		return fmt.Errorf("validator%v", err)
	}
	return nil
}

func checkValidatorWrapperEqual(w1, w2 staking.ValidatorWrapper) error {
	if err := checkValidatorEqual(w1.Validator, w2.Validator); err != nil {
		return fmt.Errorf(".Validator%v", err)
	}
	if err := CheckDelegationsEqual(w1.Delegations, w2.Delegations); err != nil {
		return fmt.Errorf(".Delegations%v", err)
	}
	if err := checkBigIntEqual(w1.Counters.NumBlocksToSign, w2.Counters.NumBlocksToSign); err != nil {
		return fmt.Errorf("..Counters.NumBlocksToSign %v", err)
	}
	if err := checkBigIntEqual(w1.Counters.NumBlocksSigned, w2.Counters.NumBlocksSigned); err != nil {
		return fmt.Errorf("..Counters.NumBlocksSigned %v", err)
	}
	if err := checkBigIntEqual(w1.BlockReward, w2.BlockReward); err != nil {
		return fmt.Errorf(".BlockReward %v", err)
	}
	return nil
}

func checkValidatorEqual(v1, v2 staking.Validator) error {
	if v1.Address != v2.Address {
		return fmt.Errorf(".Address not equal: %x / %x", v1.Address, v2.Address)
	}
	if err := checkPubKeysEqual(v1.SlotPubKeys, v2.SlotPubKeys); err != nil {
		return fmt.Errorf(".SlotPubKeys%v", err)
	}
	if err := checkBigIntEqual(v1.LastEpochInCommittee, v2.LastEpochInCommittee); err != nil {
		return fmt.Errorf(".LastEpochInCommittee %v", err)
	}
	if err := checkBigIntEqual(v1.MinSelfDelegation, v2.MinSelfDelegation); err != nil {
		return fmt.Errorf(".MinSelfDelegation %v", err)
	}
	if err := checkBigIntEqual(v1.MaxTotalDelegation, v2.MaxTotalDelegation); err != nil {
		return fmt.Errorf(".MaxTotalDelegation %v", err)
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
	if err := checkBigIntEqual(v1.CreationHeight, v2.CreationHeight); err != nil {
		return fmt.Errorf(".CreationHeight %v", err)
	}
	return nil
}

func CheckDelegationsEqual(ds1, ds2 staking.Delegations) error {
	if len(ds1) != len(ds2) {
		return fmt.Errorf(".len not equal: %v / %v", len(ds1), len(ds2))
	}
	for i := range ds1 {
		if err := checkDelegationEqual(ds1[i], ds2[i]); err != nil {
			return fmt.Errorf("[%v]%v", i, err)
		}
	}
	return nil
}

func checkDelegationEqual(d1, d2 staking.Delegation) error {
	if d1.DelegatorAddress != d2.DelegatorAddress {
		return fmt.Errorf(".DelegatorAddress not equal: %x / %x",
			d1.DelegatorAddress, d2.DelegatorAddress)
	}
	if err := checkBigIntEqual(d1.Amount, d2.Amount); err != nil {
		return fmt.Errorf(".Amount %v", err)
	}
	if err := checkBigIntEqual(d1.Reward, d2.Reward); err != nil {
		return fmt.Errorf(".Reward %v", err)
	}
	if err := checkUndelegationsEqual(d1.Undelegations, d2.Undelegations); err != nil {
		return fmt.Errorf(".Undelegations%v", err)
	}
	return nil
}

func checkUndelegationsEqual(uds1, uds2 staking.Undelegations) error {
	if len(uds1) != len(uds2) {
		return fmt.Errorf(".len not equal: %v / %v", len(uds1), len(uds2))
	}
	for i := range uds1 {
		if err := checkUndelegationEqual(uds1[i], uds2[i]); err != nil {
			return fmt.Errorf("[%v]%v", i, err)
		}
	}
	return nil
}

func checkUndelegationEqual(ud1, ud2 staking.Undelegation) error {
	if err := checkBigIntEqual(ud1.Amount, ud2.Amount); err != nil {
		return fmt.Errorf(".Amount %v", err)
	}
	if err := checkBigIntEqual(ud1.Epoch, ud2.Epoch); err != nil {
		return fmt.Errorf(".Epoch %v", err)
	}
	return nil
}

func checkPubKeysEqual(pubs1, pubs2 []bls.SerializedPublicKey) error {
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
		return fmt.Errorf(".CommissionRate%v", err)
	}
	if err := checkBigIntEqual(c1.UpdateHeight, c2.UpdateHeight); err != nil {
		return fmt.Errorf(".UpdateHeight %v", err)
	}
	return nil
}

func checkCommissionRateEqual(cr1, cr2 staking.CommissionRates) error {
	if err := checkDecEqual(cr1.Rate, cr2.Rate); err != nil {
		return fmt.Errorf(".Rate %v", err)
	}
	if err := checkDecEqual(cr1.MaxChangeRate, cr2.MaxChangeRate); err != nil {
		return fmt.Errorf(".MaxChangeRate %v", err)
	}
	if err := checkDecEqual(cr1.MaxRate, cr2.MaxRate); err != nil {
		return fmt.Errorf(".MaxRate %v", err)
	}
	return nil
}

func checkDecEqual(d1, d2 numeric.Dec) error {
	if d1.IsNil() {
		d1 = numeric.ZeroDec()
	}
	if d2.IsNil() {
		d2 = numeric.ZeroDec()
	}
	if !d1.Equal(d2) {
		return fmt.Errorf("not equal: %v / %v", d1, d2)
	}
	return nil
}

func checkBigIntEqual(i1, i2 *big.Int) error {
	if i1 == nil {
		i1 = big.NewInt(0)
	}
	if i2 == nil {
		i2 = big.NewInt(0)
	}
	if i1.Cmp(i2) != 0 {
		return fmt.Errorf("not equal: %v / %v", i1, i2)
	}
	return nil
}
