package staketest

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/shard"
)

func TestGetDefaultValidator(t *testing.T) {
	v := GetDefaultValidator()
	if err := assertValidatorDeepCopy(v, validatorPrototype); err != nil {
		t.Error(err)
	}
}

func TestGetDefaultValidatorWrapper(t *testing.T) {
	w := GetDefaultValidatorWrapper()
	if err := assertValidatorWrapperDeepCopy(w, vWrapperPrototype); err != nil {
		t.Error(err)
	}
}

func TestGetDefaultValidatorWithAddr(t *testing.T) {
	tests := []struct {
		addr common.Address
		keys []shard.BLSPublicKey
	}{
		{
			addr: common.BigToAddress(common.Big1),
			keys: []shard.BLSPublicKey{{1}, {}},
		},
		{
			addr: common.Address{},
			keys: make([]shard.BLSPublicKey, 0),
		},
		{},
	}
	for i, test := range tests {
		v := GetDefaultValidatorWithAddr(test.addr, test.keys)

		exp := CopyValidator(validatorPrototype)
		exp.Address = test.addr
		exp.SlotPubKeys = test.keys

		if err := assertValidatorDeepCopy(v, exp); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func TestGetDefaultValidatorWrapperWithAddr(t *testing.T) {
	tests := []struct {
		addr common.Address
		keys []shard.BLSPublicKey
	}{
		{
			addr: common.BigToAddress(common.Big1),
			keys: []shard.BLSPublicKey{{1}, {}},
		},
		{
			addr: common.Address{},
			keys: make([]shard.BLSPublicKey, 0),
		},
		{},
	}
	for i, test := range tests {
		v := GetDefaultValidatorWrapperWithAddr(test.addr, test.keys)

		exp := CopyValidatorWrapper(vWrapperPrototype)
		exp.Address = test.addr
		exp.SlotPubKeys = test.keys
		exp.Delegations[0].DelegatorAddress = test.addr

		if err := assertValidatorWrapperDeepCopy(v, exp); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}
