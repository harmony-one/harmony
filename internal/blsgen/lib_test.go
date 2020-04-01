package blsgen

import (
	"os"
	"testing"
)

// TestUpdateStakingList creates a bls key with passphrase and compare it with the one loaded from the generated file.
func TestUpdateStakingList(t *testing.T) {
	var err error
	privateKey, fileName, err := GenBLSKeyWithPassPhrase("abcd")
	if err != nil {
		t.Error(err)
	}
	anotherPriKey, err := LoadBLSKeyWithPassPhrase(fileName, "abcd")
	if err != nil {
		t.Error(err)
	}

	if !privateKey.IsEqual(anotherPriKey) {
		t.Errorf("Error when generating bls key \n%s\n%s\n%s",
			fileName,
			privateKey.SerializeToHexStr(),
			anotherPriKey.SerializeToHexStr(),
		)
	}

	// Clean up the testing file.
	os.Remove(fileName)
}
