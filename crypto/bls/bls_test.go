package bls

import (
	"testing"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

func TestNewMask(test *testing.T) {
	_, pubKey1 := utils.GenKeyBLS("127.0.0.1", "5555")
	_, pubKey2 := utils.GenKeyBLS("127.0.0.1", "6666")
	_, pubKey3 := utils.GenKeyBLS("127.0.0.1", "7777")

	mask, err := NewMask([]*bls.PublicKey{pubKey1, pubKey2, pubKey3}, pubKey1)

	if err != nil {
		test.Errorf("Failed to create a new Mask: %s", err)
	}

	if mask.Len() != 1 {
		test.Errorf("Mask created with wrong size: %d", mask.Len())
	}

	enabled, err := mask.KeyEnabled(pubKey1)
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
