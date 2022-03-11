package staketest

import (
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	staking "github.com/harmony-one/harmony/staking/types"
)

// CopyValidatorWrapper deep copies staking.ValidatorWrapper
func CopyValidatorWrapper(w staking.ValidatorWrapper) staking.ValidatorWrapper {
	return copyValidatorWrapper(w, true)
}

// CopyValidatorWrapperNoDelegations deep copies staking.ValidatorWrapper, excluding Delegations
// With the heap pprof turned on, we see that copying validator wrapper is an expensive operation
// of which the most expensive part is copyng the delegations
// this function can be used when delegations are not expected to be changed by the caller
func CopyValidatorWrapperNoDelegations(w staking.ValidatorWrapper) staking.ValidatorWrapper {
	return copyValidatorWrapper(w, false)
}

func copyValidatorWrapper(w staking.ValidatorWrapper, copyDelegations bool) staking.ValidatorWrapper {
	cp := staking.ValidatorWrapper{
		Validator: CopyValidator(w.Validator),
	}
	if copyDelegations {
		cp.Delegations = CopyDelegations(w.Delegations)
	} else {
		cp.Delegations = w.Delegations
	}
	if w.Counters.NumBlocksSigned != nil {
		cp.Counters.NumBlocksSigned = new(big.Int).Set(w.Counters.NumBlocksSigned)
	}
	if w.Counters.NumBlocksToSign != nil {
		cp.Counters.NumBlocksToSign = new(big.Int).Set(w.Counters.NumBlocksToSign)
	}
	if w.BlockReward != nil {
		cp.BlockReward = new(big.Int).Set(w.BlockReward)
	}
	return cp
}

// CopyValidator deep copies staking.Validator
func CopyValidator(v staking.Validator) staking.Validator {
	cp := staking.Validator{
		Address:     v.Address,
		Status:      v.Status,
		Commission:  CopyCommission(v.Commission),
		Description: v.Description,
	}
	if v.SlotPubKeys != nil {
		cp.SlotPubKeys = make([]bls.SerializedPublicKey, len(v.SlotPubKeys))
		copy(cp.SlotPubKeys, v.SlotPubKeys)
	}
	if v.LastEpochInCommittee != nil {
		cp.LastEpochInCommittee = new(big.Int).Set(v.LastEpochInCommittee)
	}
	if v.MinSelfDelegation != nil {
		cp.MinSelfDelegation = new(big.Int).Set(v.MinSelfDelegation)
	}
	if v.MaxTotalDelegation != nil {
		cp.MaxTotalDelegation = new(big.Int).Set(v.MaxTotalDelegation)
	}
	if v.CreationHeight != nil {
		cp.CreationHeight = new(big.Int).Set(v.CreationHeight)
	}
	return cp
}

// CopyCommission deep copy the Commission
func CopyCommission(c staking.Commission) staking.Commission {
	cp := staking.Commission{
		CommissionRates: c.CommissionRates.Copy(),
	}
	if c.UpdateHeight != nil {
		cp.UpdateHeight = new(big.Int).Set(c.UpdateHeight)
	}
	return cp
}

// CopyDelegations deeps copy staking.Delegations
func CopyDelegations(ds staking.Delegations) staking.Delegations {
	if ds == nil {
		return nil
	}
	cp := make(staking.Delegations, 0, len(ds))
	for _, d := range ds {
		cp = append(cp, CopyDelegation(d))
	}
	return cp
}

// CopyDelegation copies staking.Delegation
func CopyDelegation(d staking.Delegation) staking.Delegation {
	cp := staking.Delegation{
		DelegatorAddress: d.DelegatorAddress,
		Undelegations:    CopyUndelegations(d.Undelegations),
	}
	if d.Amount != nil {
		cp.Amount = new(big.Int).Set(d.Amount)
	}
	if d.Reward != nil {
		cp.Reward = new(big.Int).Set(d.Reward)
	}
	return cp
}

// CopyUndelegations deep copies staking.Undelegations
func CopyUndelegations(uds staking.Undelegations) staking.Undelegations {
	if uds == nil {
		return nil
	}
	cp := make(staking.Undelegations, 0, len(uds))
	for _, ud := range uds {
		cp = append(cp, CopyUndelegation(ud))
	}
	return cp
}

// CopyUndelegation deep copies staking.Undelegation
func CopyUndelegation(ud staking.Undelegation) staking.Undelegation {
	cp := staking.Undelegation{}
	if ud.Amount != nil {
		cp.Amount = new(big.Int).Set(ud.Amount)
	}
	if ud.Epoch != nil {
		cp.Epoch = new(big.Int).Set(ud.Epoch)
	}
	return cp
}
