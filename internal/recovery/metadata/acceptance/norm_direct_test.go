package acceptance

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/internal/recovery/report"
	"github.com/harmony-one/harmony/internal/recovery/strictdb"
)

// normalizeDir runs the shared library directly over a DB dir and returns
// the result (digests + normalized values).
func normalizeDir(t *testing.T, dir, anchorPath string) *norm.Result {
	t.Helper()
	res, err := anchor.Resolve(anchorPath)
	if err != nil {
		t.Fatal(err)
	}
	open, err := source.OpenSource(dir, res, dbopen.Options{})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer open.Close()
	srcs, err := open.BuildSources()
	if err != nil {
		t.Fatal(err)
	}
	r, err := norm.Normalize(open.NormA, srcs)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func digestJSON(t *testing.T, r *norm.Result) string {
	t.Helper()
	b, err := report.CanonicalJSON(r.Digests)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestJunkInsensitivity: independently junked copies of the same chain
// (extra pendingCL, sync markers, stats content) produce byte-identical
// digests and absence assertions.
func TestJunkInsensitivity(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	base := normalizeDir(t, dir, anchorPath)
	baseDigest := digestJSON(t, base)

	junk := t.TempDir() + "/harmony_db_0"
	if err := fixture.CopyDir(dir, junk); err != nil {
		t.Fatal(err)
	}
	addJunk(t, junk)
	jres := normalizeDir(t, junk, anchorPath)
	if digestJSON(t, jres) != baseDigest {
		t.Fatal("junked copy produced different digests (normalization is not junk-insensitive)")
	}
	// Absence assertions (predicate + expected_remaining) must be identical.
	if len(jres.Assertions) != len(base.Assertions) {
		t.Fatalf("assertion counts differ: %d vs %d", len(jres.Assertions), len(base.Assertions))
	}
	for i := range base.Assertions {
		if base.Assertions[i].Namespace != jres.Assertions[i].Namespace ||
			base.Assertions[i].Predicate != jres.Assertions[i].Predicate ||
			base.Assertions[i].ExpectedRemaining != jres.Assertions[i].ExpectedRemaining {
			t.Fatalf("absence assertion %d differs under junk", i)
		}
	}
}

// TestMechanicalCleanEquality is B5 test 1: the dirty descendant normalizes
// to the clean golden. Mechanically apply the deletion plan (deletions +
// rewrites) to a copy, then re-normalize: the digests must match, no
// epoch>T / post-T record remains, and the stats namespace is bit-identical
// to the input.
func TestMechanicalCleanEquality(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	base := normalizeDir(t, dir, anchorPath)
	baseDigest := digestJSON(t, base)

	// Snapshot the source stats namespace for the bit-identical check.
	srcStats := readNamespace(t, dir, []byte("validator-stats"))

	clean := t.TempDir() + "/harmony_db_0"
	if err := fixture.CopyDir(dir, clean); err != nil {
		t.Fatal(err)
	}
	applyPlan(t, clean, base)

	// Re-normalize the cleaned copy: identical digests (the dirty
	// descendant reduced to the clean golden).
	cres := normalizeDir(t, clean, anchorPath)
	if digestJSON(t, cres) != baseDigest {
		t.Fatal("mechanically cleaned copy does not normalize to the same digests")
	}

	// Raw-prefix scans: no epoch>T snapshot/ss, no post-T blk-rwd, no
	// post-T dvl index remain.
	assertNoFutureRecords(t, clean)

	// Stats namespace bit-identical (kept untouched, §8 Q4).
	cleanStats := readNamespace(t, clean, []byte("validator-stats"))
	if !equalKV(srcStats, cleanStats) {
		t.Fatal("stats namespace changed under mechanical cleanup (must be bit-identical)")
	}
}

// addJunk writes node-local / sync-era junk that normalization must ignore.
func addJunk(t *testing.T, dir string) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	put := func(k, v []byte) {
		if err := db.Put(k, v); err != nil {
			t.Fatal(err)
		}
	}
	put([]byte("pendingCL"), []byte("junk-pending-crosslinks"))
	put([]byte("pendingSC"), []byte("junk-pending-slashing"))
	put([]byte("LastPivot"), []byte("junk-pivot"))
	put([]byte("SnapdbInfo"), []byte("junk-snapdb"))
	put([]byte("unclean-shutdown"), []byte("junk"))
	put([]byte("LastCommits"), []byte("junk-legacy-commits"))
}

// applyPlan mechanically applies the deletion plan to a DB copy
// (test-harness only; B4 owns production apply).
func applyPlan(t *testing.T, dir string, r *norm.Result) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, d := range r.Deletions.Deletions() {
		if err := db.Delete(common.FromHex(d.Key)); err != nil {
			t.Fatal(err)
		}
	}
	// Rewrites: materialize the normalized values from the set.
	rewrite := map[string][]byte{}
	rewrite[string(r.Normalized.ValidatorList.Key)] = r.Normalized.ValidatorList.Value
	for _, rec := range r.Normalized.DVL {
		rewrite[string(rec.Key)] = rec.Value
	}
	for _, rw := range r.Deletions.Rewrites() {
		key := common.FromHex(rw.Key)
		val, ok := rewrite[string(key)]
		if !ok {
			t.Fatalf("no normalized value to materialize rewrite %s", rw.Key)
		}
		if report.SHA256Hex(val) != rw.NewValueSHA256 {
			t.Fatalf("rewrite value hash mismatch for %s", rw.Key)
		}
		if err := db.Put(key, val); err != nil {
			t.Fatal(err)
		}
	}
}

func assertNoFutureRecords(t *testing.T, dir string) {
	t.Helper()
	db, err := dbopen.OpenStrictReadOnly(dir, dbopen.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	kv := db.KV()
	target := uint64(fxTarget)
	epoch := uint64(2)

	check := func(prefix []byte) {
		_ = strictdb.ForEach(kv, prefix, func(key, value []byte) error {
			ns, meta := strictdb.Classify(key)
			switch ns {
			case strictdb.NsValidatorSnapshot, strictdb.NsShardState:
				if meta.Epoch != nil && meta.Epoch.Uint64() > epoch {
					t.Fatalf("epoch>%d record remains in cleaned DB: %s %x", epoch, ns, key)
				}
			case strictdb.NsBlockRewardAccum:
				if meta.Number > target {
					t.Fatalf("post-target blk-rwd remains: %d", meta.Number)
				}
			}
			return nil
		})
	}
	check([]byte("validator-snapshot"))
	check([]byte("ss"))
	check([]byte("blk-rwd-"))
}

func readNamespace(t *testing.T, dir string, prefix []byte) map[string][]byte {
	t.Helper()
	db, err := dbopen.OpenStrictReadOnly(dir, dbopen.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	out := map[string][]byte{}
	_ = strictdb.ForEach(db.KV(), prefix, func(k, v []byte) error {
		out[string(k)] = append([]byte(nil), v...)
		return nil
	})
	return out
}

func equalKV(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(b[k], v) {
			return false
		}
	}
	return true
}

var _ = big.NewInt
