package blsgen

import (
	"os"
	"testing"
)

// TestUpdateStakingList creates a bls key with passphrase and compare it with the one loaded from the generated file.
func TestUpdateStakingList(t *testing.T) {
	privateKey, fileName := GenBlsKeyWithPassPhrase("abcd")
	anotherPriKey := LoadBlsKeyWithPassPhrase(fileName, "abcd")

	if !privateKey.IsEqual(anotherPriKey) {
		t.Error("Error when generating bls key.")
	}

	// Clean up the testing file.
	os.Remove(fileName)
}
