package staketest

import (
	"testing"

	staking "github.com/harmony-one/harmony/staking/types"
)

func TestCheckValidatorWrapperEqual(t *testing.T) {
	tests := []struct {
		w1, w2 staking.ValidatorWrapper
	}{
		{vWrapperPrototype, vWrapperPrototype},
		{makeZeroValidatorWrapper(), makeZeroValidatorWrapper()},
		{staking.ValidatorWrapper{}, staking.ValidatorWrapper{}},
	}
	for i, test := range tests {
		if err := CheckValidatorWrapperEqual(test.w1, test.w2); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func TestCheckValidatorEqual(t *testing.T) {
	tests := []struct {
		v1, v2 staking.Validator
	}{
		{validatorPrototype, validatorPrototype},
		{makeZeroValidator(), makeZeroValidator()},
		{staking.Validator{}, staking.Validator{}},
	}
	for i, test := range tests {
		if err := CheckValidatorEqual(test.v1, test.v2); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}
