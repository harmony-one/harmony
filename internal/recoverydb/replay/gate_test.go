package replay

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
)

// faultyProber injects a read error on selected keys; everything else
// delegates to a real store, so absence still reports (false, nil).
type faultyProber struct {
	inner  interface{ Has([]byte) (bool, error) }
	failOn func([]byte) bool
}

func (f faultyProber) Has(key []byte) (bool, error) {
	if f.failOn(key) {
		return false, errors.New("injected read fault")
	}
	return f.inner.Has(key)
}

// TestPostTargetSweepFailClosed pins round 14 finding 1: the post-target
// gate must distinguish not-found from read errors — an I/O error on any
// probe is a gate failure, never treated as absence.
func TestPostTargetSweepFailClosed(t *testing.T) {
	const target = uint64(22)
	child := common.HexToHash("0xabad1dea")

	t.Run("cleanPasses", func(t *testing.T) {
		if err := postTargetSweep(rawdb.NewMemoryDatabase(), target, child); err != nil {
			t.Fatalf("clean sweep must pass, got %v", err)
		}
	})

	t.Run("presentRefuses", func(t *testing.T) {
		db := rawdb.NewMemoryDatabase()
		if err := db.Put(keys.CanonicalHashKey(target+3), common.Hash{1}.Bytes()); err != nil {
			t.Fatal(err)
		}
		err := postTargetSweep(db, target, child)
		if err == nil || !strings.Contains(err.Error(), "present") {
			t.Fatalf("post-target canonical mapping must refuse, got %v", err)
		}
	})

	faultCases := []struct {
		name string
		key  []byte
		want string
	}{
		{"canonicalReadFault", keys.CanonicalHashKey(target + 1), "probe canonical mapping"},
		{"blockSigReadFault", keys.BlockSigKey(target + 5), "probe block-sig"},
		{"abandonedChildReadFault", keys.HeaderNumberKey(child), "probe abandoned-child"},
	}
	for _, tc := range faultCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := faultyProber{
				inner:  rawdb.NewMemoryDatabase(),
				failOn: func(k []byte) bool { return bytes.Equal(k, tc.key) },
			}
			err := postTargetSweep(db, target, child)
			if err == nil {
				t.Fatalf("read fault on %s must fail the sweep (fail-open!)", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "injected read fault") {
				t.Fatalf("error must surface the probe and the fault, got %v", err)
			}
		})
	}
}
