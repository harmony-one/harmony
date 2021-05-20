package staketest

import (
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	oneBig       = big.NewInt(1e18)
	tenKOnes     = new(big.Int).Mul(big.NewInt(10000), oneBig)
	twentyKOnes  = new(big.Int).Mul(big.NewInt(20000), oneBig)
	hundredKOnes = new(big.Int).Mul(big.NewInt(100000), oneBig)

	// DefaultDelAmount is the default delegation amount
	DefaultDelAmount = new(big.Int).Set(twentyKOnes)

	// DefaultMinSelfDel is the default value of MinSelfDelegation
	DefaultMinSelfDel = new(big.Int).Set(tenKOnes)

	// DefaultMaxTotalDel is the default value of MaxTotalDelegation
	DefaultMaxTotalDel = new(big.Int).Set(hundredKOnes)
)

var (
	vWrapperPrototype = func() staking.ValidatorWrapper {
		w := staking.ValidatorWrapper{
			Validator: validatorPrototype,
			Delegations: staking.Delegations{
				staking.Delegation{
					DelegatorAddress: validatorPrototype.Address,
					Amount:           DefaultDelAmount,
					Reward:           common.Big0,
					Undelegations:    staking.Undelegations{},
				},
			},
			BlockReward: common.Big0,
		}
		w.Counters.NumBlocksToSign = common.Big0
		w.Counters.NumBlocksSigned = common.Big0
		return w
	}()

	validatorPrototype = staking.Validator{
		Address:              common.Address{},
		SlotPubKeys:          []bls.SerializedPublicKey{bls.SerializedPublicKey{}},
		LastEpochInCommittee: common.Big0,
		MinSelfDelegation:    DefaultMinSelfDel,
		MaxTotalDelegation:   DefaultMaxTotalDel,
		Status:               effective.Active,
		Commission:           commission,
		Description:          description,
		CreationHeight:       common.Big0,
	}

	commissionRates = staking.CommissionRates{
		Rate:          numeric.NewDecWithPrec(4, 1),
		MaxRate:       numeric.NewDecWithPrec(9, 1),
		MaxChangeRate: numeric.NewDecWithPrec(4, 1),
	}

	commission = staking.Commission{
		CommissionRates: commissionRates,
		UpdateHeight:    common.Big0,
	}

	description = staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
)

// GetDefaultValidator return the default staking.Validator for testing
func GetDefaultValidator() staking.Validator {
	return CopyValidator(validatorPrototype)
}

// GetDefaultValidatorWithAddr return the default staking.Validator with the
// given validator address and bls keys
func GetDefaultValidatorWithAddr(addr common.Address, pubs []bls.SerializedPublicKey) staking.Validator {
	v := CopyValidator(validatorPrototype)
	v.Address = addr
	if pubs != nil {
		v.SlotPubKeys = make([]bls.SerializedPublicKey, len(pubs))
		copy(v.SlotPubKeys, pubs)
	} else {
		v.SlotPubKeys = nil
	}
	return v
}

// GetDefaultValidatorWrapper return the default staking.ValidatorWrapper for testing
func GetDefaultValidatorWrapper() staking.ValidatorWrapper {
	return CopyValidatorWrapper(vWrapperPrototype)
}

// GetDefaultValidatorWrapperWithAddr return the default staking.ValidatorWrapper
// with the given validator address and bls keys.
func GetDefaultValidatorWrapperWithAddr(addr common.Address, pubs []bls.SerializedPublicKey) staking.ValidatorWrapper {
	w := CopyValidatorWrapper(vWrapperPrototype)
	w.Address = addr
	if pubs != nil {
		w.SlotPubKeys = make([]bls.SerializedPublicKey, len(pubs))
		copy(w.SlotPubKeys, pubs)
	} else {
		w.SlotPubKeys = nil
	}
	w.Delegations[0].DelegatorAddress = addr

	return w
}
