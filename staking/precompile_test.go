package staking

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestParseStakingMethod(t *testing.T) {
	input := []byte{109, 107, 47, 119}
	if method, err := ParseStakingMethod(input); err != nil {
		t.Errorf("Expected no error but got %v", err)
	} else {
		if method.Name != "CollectRewards" {
			t.Errorf("Expected method CollectRewards but got %v", method.Name)
		}
	}
}

func TestParseAddressFromKey(t *testing.T) {
	// provide bytes to ParseAddressFromKey
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55}
	expected := common.BytesToAddress(input)
	args := map[string]interface{}{}
	args["Key"] = input
	if address, err := ParseAddressFromKey(args, "Key"); err != nil {
		t.Errorf("Got error %v while parsing bytes address", err)
	} else if address.Hex() != expected.Hex() {
		t.Errorf("Expected %v, got %v", expected.Hex(), address.Hex())
	}
}

func TestValidateContractAddress(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19, 55}
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse address from <nil>")
	if _, err := ValidateContractAddress(common.BytesToAddress(input), args, "ValidatorAddress"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseDescription(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse Description from <nil>")
	if _, err := ParseDescription(args, "Description"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseCommissionRates(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse CommissionRates from <nil>")
	if _, err := ParseCommissionRates(args, "CommissionRates"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseBigIntFromKey(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse BigInt from <nil>")
	if _, err := ParseBigIntFromKey(args, "PotentialBigInt"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseSlotPubKeys(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse SlotPubKeys from <nil>")
	if _, err := ParseSlotPubKeys(args, "SlotPubKeys"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseSlotKeySigs(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse SlotKeySigs from <nil>")
	if _, err := ParseSlotKeySigs(args, "SlotKeySigs"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseSlotPubKeyFromKey(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse SlotPubKey from <nil>")
	if _, err := ParseSlotPubKeyFromKey(args, "SlotPubKey"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseSlotKeySigFromKey(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse SlotKeySig from <nil>")
	if _, err := ParseSlotKeySigFromKey(args, "SlotKeySig"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}

func TestParseCommissionRate(t *testing.T) {
	args := map[string]interface{}{}
	expectedError := errors.New("Cannot parse CommissionRate from <nil>")
	if _, err := ParseCommissionRate(args, "CommissionRate"); err != nil {
		if expectedError.Error() != err.Error() {
			t.Errorf("Expected error %v, got %v", expectedError, err)
		}
	} else {
		t.Errorf("Expected error %v, got result", expectedError)
	}
}
