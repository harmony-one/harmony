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
