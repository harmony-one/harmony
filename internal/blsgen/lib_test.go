package blsgen

import "testing"

func TestUpdateStakingList(t *testing.T) {
	privateKey, fileName := GenBlsKeyWithPassPhrase("abcd")
	anotherPriKey := LoadBlsKeyWithPassPhrase(fileName, "abcd")

	if !privateKey.IsEqual(anotherPriKey) {
		t.Error("Error when generating bls key.")
	}
}
