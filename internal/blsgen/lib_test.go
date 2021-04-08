package blsgen

import (
	"bytes"
	"encoding/hex"
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

	if !bytes.Equal(privateKey.ToBytes(), anotherPriKey.ToBytes()) {
		t.Errorf("Error when generating bls key \n%s\n%s\n%s",
			fileName,
			hex.EncodeToString(privateKey.ToBytes()),
			hex.EncodeToString(anotherPriKey.ToBytes()),
		)
	}

	// Clean up the testing file.
	os.Remove(fileName)
}
