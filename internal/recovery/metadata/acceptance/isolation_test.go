package acceptance

import (
	"os"
	"os/exec"
	"regexp"
	"testing"
)

// internal/chain keeps process-wide reward-payout caches (votingPowerCache,
// delegateShareCache) keyed only by (epoch, shard) / (epoch, validator) —
// NOT by chain identity. The production harmony-recovery binary opens exactly
// one chain per process, so those caches are always coherent there. A test
// binary, however, generates MANY chains in one process; two fixtures whose
// epoch-3 validator snapshots differ (e.g. an extra pre-snapshot delegation)
// then poison each other's block-47 aggregated payout through the shared
// cache: the stale share map is missing the extra delegator, AddReward's
// nil-error wrap (errors.Wrapf with a nil err) silently aborts the payout
// loop midway, and the delegator accrues nothing — no error surfaces.
//
// Fixing the caches or AddReward lives in consensus code and is explicitly
// out of scope for this recovery branch, so the recovery-local mechanism is
// PROCESS isolation: any test whose fixture's payout-epoch snapshot diverges
// from the package's shared shape re-executes itself in a fresh test process
// where the caches start cold. Everything else in the package shares one
// identical snapshot shape, so its cache collisions are value-identical and
// harmless.

// isolatedEnv marks the re-executed child process.
const isolatedEnv = "HMY_RECOVERY_TEST_ISOLATED"

// runIsolatedSubtest re-executes exactly the calling test in a fresh process
// (cold internal/chain caches) and reports its result. Returns true when the
// caller IS the isolated child and should run the real test body; returns
// false in the parent after the child completed (the parent must return
// immediately).
func runIsolatedSubtest(t *testing.T) bool {
	t.Helper()
	if os.Getenv(isolatedEnv) == "1" {
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("isolation: locate test binary: %v", err)
	}
	cmd := exec.Command(exe,
		"-test.run=^"+regexp.QuoteMeta(t.Name())+"$",
		"-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), isolatedEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated re-exec of %s failed: %v\n%s", t.Name(), err, out)
	}
	return false
}
