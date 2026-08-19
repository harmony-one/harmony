package inspect

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// faultyDB injects a Has error on selected keys; all other operations
// delegate, so genuine absence still reports (false, nil).
type faultyDB struct {
	ethdb.Database
	failOn func([]byte) bool
}

func (f faultyDB) Has(key []byte) (bool, error) {
	if f.failOn(key) {
		return false, errors.New("injected Has fault")
	}
	return f.Database.Has(key)
}

// TestPreflightHasFaultsFailClosed pins round 14 finding 2: a Has() error on
// the LastFast or epoch-VRF probes must surface as its own refusal, never be
// collapsed into "absent".
func TestPreflightHasFaultsFailClosed(t *testing.T) {
	sched, err := harness.Schedule("localnet")
	if err != nil {
		t.Fatal(err)
	}
	run := func(failOn func([]byte) bool) []string {
		rep := &report.InspectReport{HeadsAgree: true, MarkerPresence: map[string]bool{}}
		db := faultyDB{Database: rawdb.NewMemoryDatabase(), failOn: failOn}
		runPreflight(Params{FullState: true}, rep, db, report.HeadTuple{},
			sched, 0,
			func(string, string, ...interface{}) {}, func(string) {})
		return rep.ReplayPreflight.Failures
	}
	contains := func(failures []string, want string) bool {
		for _, f := range failures {
			if strings.Contains(f, want) {
				return true
			}
		}
		return false
	}

	t.Run("lastFastFault", func(t *testing.T) {
		failures := run(func(k []byte) bool { return bytes.Equal(k, keys.HeadFastBlockKey) })
		if !contains(failures, "LastFast probe failed") {
			t.Fatalf("LastFast Has fault must refuse explicitly, got %v", failures)
		}
	})
	t.Run("lastFastAbsent", func(t *testing.T) {
		failures := run(func([]byte) bool { return false })
		if !contains(failures, "LastFast missing") {
			t.Fatalf("genuine absence must still refuse as missing, got %v", failures)
		}
	})
	t.Run("epochVrfFault", func(t *testing.T) {
		vrfKey := keys.EpochVrfKey(big.NewInt(0))
		failures := run(func(k []byte) bool { return bytes.Equal(k, vrfKey) })
		if !contains(failures, "epoch-0 VRF probe failed") {
			t.Fatalf("epoch-VRF Has fault must refuse explicitly, got %v", failures)
		}
	})
}
