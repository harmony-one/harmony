package bls

import (
	"strings"
	"testing"

	"github.com/harmony-one/bls/ffi/go/bls"
)

// Test the basic functionality of a BLS multi-sig mask.
func TestNewMask(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	enabled, err := mask.KeyEnabled(pubKey1.Bytes)
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
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey4 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	pubKey4.Bytes.FromLibBLSPublicKey(pubKey4.Object)

	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey4.Bytes, true)

	if err == nil {
		test.Errorf("Failed to set a key: %s", err)
	}
}

func TestThreshHoldPolicy(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	threshHoldPolicy := *NewThresholdPolicy(1)

	mask.SetKey(pubKey1.Bytes, true)
	mask.SetKey(pubKey2.Bytes, true)

	if mask.CountEnabled() != 2 {
		test.Errorf("Number of enabled nodes: %d , expected count = 2 ", mask.CountEnabled())
	}

	if !threshHoldPolicy.Check(mask) {
		test.Error("Number of enabled nodes less than threshold")
	}

	mask.SetKey(pubKey1.Bytes, false)
	mask.SetKey(pubKey2.Bytes, false)

	if threshHoldPolicy.Check(mask) {
		test.Error("Number of enabled nodes more than equal to threshold")
	}
}

func TestCompletePolicy(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	completePolicy := CompletePolicy{}

	mask.SetKey(pubKey1.Bytes, true)
	mask.SetKey(pubKey2.Bytes, true)
	mask.SetKey(pubKey3.Bytes, true)

	if mask.CountEnabled() != 3 {
		test.Errorf("Number of enabled nodes: %d , expected count = 3 ", mask.CountEnabled())
	}

	if !completePolicy.Check(mask) {
		test.Error("Number of enabled nodes not equal to total count")
	}

	mask.SetKey(pubKey1.Bytes, false)

	if completePolicy.Check(mask) {
		test.Error("Number of enabled nodes equal to total count")
	}
}

func TestAggregatedSignature(test *testing.T) {
	var sec bls.SecretKey
	sec.SetByCSPRNG()

	signs := []*bls.Sign{sec.Sign("message1"), sec.Sign("message2")}

	multiSignature := AggregateSig(signs)

	str := multiSignature.SerializeToHexStr()

	if strings.Compare(multiSignature.SerializeToHexStr(), "0") == 0 {
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
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey4 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	pubKey4.Bytes.FromLibBLSPublicKey(pubKey4.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

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

	if _, error := mask.KeyEnabled(pubKey4.Bytes); error == nil {
		test.Error("Expected key not found error")
	}

	if _, error := mask.IndexEnabled(5); error == nil {
		test.Error("Expected index out of range error")
	}

	if err := mask.SetKey(pubKey4.Bytes, true); err == nil {
		test.Error("Expected key not found error")
	}
}

func TestGetSignedPubKeysFromBitmap(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey4 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	pubKey4.Bytes.FromLibBLSPublicKey(pubKey4.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

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
		!enabledKeysFromBitmap[0].Object.IsEqual(pubKey1.Object) || !enabledKeysFromBitmap[1].Object.IsEqual(pubKey3.Object) {
		test.Error("Enabled keys from bitmap are incorrect")
	}
}

func TestSetKeyAtomic(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey3 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey4 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	pubKey3.Bytes.FromLibBLSPublicKey(pubKey3.Object)
	pubKey4.Bytes.FromLibBLSPublicKey(pubKey4.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2, pubKey3})
	err := mask.SetKey(pubKey1.Bytes, true)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}
	length := mask.Len()
	_ = length

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	mask.SetKey(pubKey1.Bytes, true)

	enabledKeysFromBitmap, _ := mask.GetSignedPubKeysFromBitmap(mask.Bitmap)

	if len(enabledKeysFromBitmap) != 1 ||
		!enabledKeysFromBitmap[0].Object.IsEqual(pubKey1.Object) {
		test.Error("Enabled keys from bitmap are incorrect")
	}

	mask.SetKeysAtomic([]*PublicKeyWrapper{&pubKey1, &pubKey2}, true)

	enabledKeysFromBitmap, _ = mask.GetSignedPubKeysFromBitmap(mask.Bitmap)
	if len(enabledKeysFromBitmap) != 2 ||
		!enabledKeysFromBitmap[0].Object.IsEqual(pubKey1.Object) || !enabledKeysFromBitmap[1].Object.IsEqual(pubKey2.Object) {
		test.Error("Enabled keys from bitmap are incorrect")
	}

	err = mask.SetKeysAtomic([]*PublicKeyWrapper{&pubKey1, &pubKey4}, true)

	if !strings.Contains(err.Error(), "key not found") {
		test.Error(err, "expect error due to key not found")
	}
}

func TestCopyParticipatingMask(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2})
	_ = mask.SetKey(pubKey1.Bytes, true)

	clonedMask := mask.Mask()

	if len(clonedMask) != 1 {
		test.Error("Length of clonedMask doesn't match")
	}

}

func TestSetMask(test *testing.T) {
	pubKey1 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}
	pubKey2 := PublicKeyWrapper{Object: RandPrivateKey().GetPublicKey()}

	pubKey1.Bytes.FromLibBLSPublicKey(pubKey1.Object)
	pubKey2.Bytes.FromLibBLSPublicKey(pubKey2.Object)
	mask := NewMask([]PublicKeyWrapper{pubKey1, pubKey2})
	_ = mask.SetKey(pubKey1.Bytes, true)

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
