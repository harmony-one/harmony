package blsgen

import (
	"os"
	"testing"
)

// TestUpdateStakingList creates a bls key with passphrase and compare it with the one loaded from the generated file.
func TestUpdateStakingList(t *testing.T) {
	var err error
	privateKey, fileName, err := GenBlsKeyWithPassPhrase("abcd")
	if err != nil {
		t.Error(err)
	}
	anotherPriKey, err := LoadBlsKeyWithPassPhrase(fileName, "abcd")
	if err != nil {
		t.Error(err)
	}

	if !privateKey.IsEqual(anotherPriKey) {
		t.Error("Error when generating bls key.")
	}

	// Clean up the testing file.
	os.Remove(fileName)
}
