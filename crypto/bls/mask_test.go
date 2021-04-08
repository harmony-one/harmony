package bls

import (
	"strings"
	"testing"
)

// Test the basic functionality of a BLS multi-sig mask.
func TestNewMask(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()

	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	enabled, err := mask.KeyEnabled(pubKey1.Serialized())
	if !enabled || err != nil {
		test.Errorf("My key pubKey1 should have been enabled: %s", err)
	}

	if mask.CountEnabled() != 1 {
		test.Error("Only one key should have been enabled")
	}

	if mask.CountTotal() != 3 {
		test.Error("Should have a total of 3 keys")
	}
}

func TestNewMaskWithAbsentPublicKey(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()
	pubKey4 := RandSecretKey().PublicKey()

	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey4)

	if err == nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask != nil {
		test.Errorf("Expected failure to create a new mask")
	}

}

func TestThreshHoldPolicy(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()

	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	threshHoldPolicy := *NewThresholdPolicy(1)

	mask.SetKey(pubKey1.Serialized(), true)
	mask.SetKey(pubKey2.Serialized(), true)

	if mask.CountEnabled() != 2 {
		test.Errorf("Number of enabled nodes: %d , expected count = 2 ", mask.CountEnabled())
	}

	if !threshHoldPolicy.Check(mask) {
		test.Error("Number of enabled nodes less than threshold")
	}

	mask.SetKey(pubKey1.Serialized(), false)
	mask.SetKey(pubKey2.Serialized(), false)

	if threshHoldPolicy.Check(mask) {
		test.Error("Number of enabled nodes more than equal to threshold")
	}
}

func TestCompletePolicy(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()

	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	completePolicy := CompletePolicy{}

	mask.SetKey(pubKey1.Serialized(), true)
	mask.SetKey(pubKey2.Serialized(), true)
	mask.SetKey(pubKey3.Serialized(), true)

	if mask.CountEnabled() != 3 {
		test.Errorf("Number of enabled nodes: %d , expected count = 3 ", mask.CountEnabled())
	}

	if !completePolicy.Check(mask) {
		test.Error("Number of enabled nodes not equal to total count")
	}

	mask.SetKey(pubKey1.Serialized(), false)

	if completePolicy.Check(mask) {
		test.Error("Number of enabled nodes equal to total count")
	}
}

func TestAggregatedSignature(test *testing.T) {
	// var sec bls.SecretKey
	// sec.SetByCSPRNG()

	sec := RandSecretKey()

	signatures := []Signature{sec.Sign([]byte("message1")), sec.Sign([]byte("message2"))}

	aggregatedSignature := AggreagateSignatures(signatures)

	str := aggregatedSignature.ToHex()

	if strings.Compare(str, "0") == 0 {
		test.Error("Error creating multisignature", str)
	}
}

func TestAggregateMasks(test *testing.T) {
	message := []byte("message")
	newMessage := []byte("message")
	emptyMessage := []byte("")

	aggMask, err := AggregateMasks(message, emptyMessage)
	if aggMask != nil {
		test.Error("Expected mismatching bitmap lengths")
	}
	if err == nil {
		test.Error("Expected error thrown because of bitmap length mismatch")
	}

	if _, err := AggregateMasks(message, newMessage); err != nil {
		test.Error("Error thrown in aggregating masks")
	}
}

func TestEnableKeyFunctions(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()
	pubKey4 := RandSecretKey().PublicKey()
	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}
	length := mask.Len()
	_ = length

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	mask.SetBit(0, true)
	mask.SetBit(1, false)
	mask.SetBit(0, true)

	if err := mask.SetBit(5, true); err == nil {
		test.Error("Expected index out of range error")
	}

	enabledKeys := mask.GetPubKeyFromMask(true)
	disabledKeys := mask.GetPubKeyFromMask(false)

	if len(enabledKeys) != 1 {
		test.Error("Count of enabled keys doesn't match")
	}

	if len(disabledKeys) != 2 {
		test.Error("Count of disabled keys don't match")
	}

	if _, error := mask.KeyEnabled(pubKey4.Serialized()); error == nil {
		test.Error("Expected key not found error")
	}

	if _, error := mask.IndexEnabled(5); error == nil {
		test.Error("Expected index out of range error")
	}

	if err := mask.SetKey(pubKey4.Serialized(), true); err == nil {
		test.Error("Expected key not found error")
	}
}

func TestGetSignedPubKeysFromBitmap(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()
	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}
	length := mask.Len()
	_ = length

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	mask.SetBit(0, true)
	mask.SetBit(1, false)
	mask.SetBit(0, true)
	mask.SetBit(2, true)

	enabledKeysFromBitmap, _ := mask.GetSignedPubKeysFromBitmap(mask.Bitmap)

	if len(enabledKeysFromBitmap) != 2 ||
		!enabledKeysFromBitmap[0].Equal(pubKey1) || !enabledKeysFromBitmap[1].Equal(pubKey3) {
		test.Error("Enabled keys from bitmap are incorrect")
	}
}

func TestSetKeyAtomic(test *testing.T) {
	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()
	pubKey3 := RandSecretKey().PublicKey()
	pubKey4 := RandSecretKey().PublicKey()
	mask, err := NewMask([]PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}
	length := mask.Len()
	_ = length

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	mask.SetKey(pubKey1.Serialized(), true)

	enabledKeysFromBitmap, _ := mask.GetSignedPubKeysFromBitmap(mask.Bitmap)

	if len(enabledKeysFromBitmap) != 1 ||
		!enabledKeysFromBitmap[0].Equal(pubKey1) {
		test.Error("Enabled keys from bitmap are incorrect")
	}

	mask.SetKeysAtomic([]PublicKey{pubKey1, pubKey2}, true)

	enabledKeysFromBitmap, _ = mask.GetSignedPubKeysFromBitmap(mask.Bitmap)
	if len(enabledKeysFromBitmap) != 2 ||
		!enabledKeysFromBitmap[0].Equal(pubKey1) || !enabledKeysFromBitmap[1].Equal(pubKey2) {
		test.Error("Enabled keys from bitmap are incorrect")
	}

	err = mask.SetKeysAtomic([]PublicKey{pubKey1, pubKey4}, true)

	if !strings.Contains(err.Error(), "key not found") {
		test.Error(err, "expect error due to key not found")
	}
}

func TestCopyParticipatingMask(test *testing.T) {

	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()

	mask, _ := NewMask([]PublicKey{pubKey1, pubKey2}, pubKey1)

	clonedMask := mask.Mask()

	if len(clonedMask) != 1 {
		test.Error("Length of clonedMask doesn't match")
	}

}

func TestSetMask(test *testing.T) {

	pubKey1 := RandSecretKey().PublicKey()
	pubKey2 := RandSecretKey().PublicKey()

	mask, _ := NewMask([]PublicKey{pubKey1, pubKey2}, pubKey1)

	_ = mask
	maskBytes := []byte{3}
	mask.SetMask(maskBytes)

	if mask.CountEnabled() != 2 {
		test.Error("Count of Enabled nodes doesn't match")
	}

	newMaskBytes := []byte{3, 2}

	if err := mask.SetMask(newMaskBytes); err == nil {
		test.Error("Expected mismatching Bitmap lengths")
	}
}
