package report

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJournalStateMachine(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	path := JournalPath(dest)

	j, err := CreateJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if j.Last().State != StateInProgress {
		t.Fatalf("fresh journal state %s", j.Last().State)
	}
	// Reopen-refusal of IN_PROGRESS: creating again refuses (v1 no-resume).
	if _, err := CreateJournal(path); err == nil {
		t.Fatal("second CreateJournal must refuse")
	}
	if st, _, err := JournalState(path); err != nil || st != StateInProgress {
		t.Fatalf("state %s err %v", st, err)
	}

	// Substates within IN_PROGRESS (the package-db exception).
	if err := j.Substate(SubstatePromoting, "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := j.Substate(SubstatePromoted, "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := j.Complete(StateCompleteVerified, "done"); err != nil {
		t.Fatal(err)
	}
	if err := j.Complete("BOGUS", ""); err == nil {
		t.Fatal("invalid terminal state must refuse")
	}
	j.Close()

	st, last, err := JournalState(path)
	if err != nil || st != StateCompleteVerified {
		t.Fatalf("terminal state %s err %v", st, err)
	}
	if last.Note != "done" {
		t.Fatalf("note lost")
	}

	loaded, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	recs := loaded.Records()
	loaded.Close()
	if len(recs) != 4 || recs[1].Substate != SubstatePromoting || recs[1].ReleaseID != "abc123" {
		t.Fatalf("journal records wrong: %+v", recs)
	}

	// COMPLETE_UNRELEASABLE is a distinct terminal state.
	dest2 := filepath.Join(t.TempDir(), "d2")
	os.MkdirAll(dest2, 0o755)
	j2, err := CreateJournal(JournalPath(dest2))
	if err != nil {
		t.Fatal(err)
	}
	if err := j2.Complete(StateCompleteUnreleasable, "size gate"); err != nil {
		t.Fatal(err)
	}
	j2.Close()
	if st, _, _ := JournalState(JournalPath(dest2)); st != StateCompleteUnreleasable {
		t.Fatalf("state %s", st)
	}
}

func TestDigestDeterminismAndDomains(t *testing.T) {
	h1 := NewHasher("domain.a")
	h2 := NewHasher("domain.a")
	h3 := NewHasher("domain.b")
	for _, h := range []*Hasher{h1, h2, h3} {
		h.Add([]byte("key1"), []byte("val1"))
		h.Add([]byte("key2"), []byte("val2"))
	}
	if h1.Digest() != h2.Digest() {
		t.Fatal("same input, same domain must produce identical digests")
	}
	if h1.Digest().SHA256 == h3.Digest().SHA256 {
		t.Fatal("domains must separate digests")
	}
	// Length-prefixing: (ab, c) != (a, bc).
	ha := NewHasher("d")
	ha.Add([]byte("ab"), []byte("c"))
	hb := NewHasher("d")
	hb.Add([]byte("a"), []byte("bc"))
	if ha.Digest().SHA256 == hb.Digest().SHA256 {
		t.Fatal("length prefixing broken: chunk boundaries must matter")
	}
	// A single flipped value byte changes the digest.
	hc := NewHasher("d")
	hc.Add([]byte("ab"), []byte("d"))
	if ha.Digest().SHA256 == hc.Digest().SHA256 {
		t.Fatal("flipped byte must change digest")
	}
}

func TestDigestSetValidateAndDiff(t *testing.T) {
	mk := func() *DigestSet {
		s := &DigestSet{SchemaVersion: DigestSetSchemaV1, Network: "localnet"}
		h := NewHasher("x")
		h.Add([]byte("k"))
		d := h.Digest()
		s.Accounts, s.StorageSlots, s.Codes = d, d, d
		s.CXSpent, s.CXOutgoingWindow, s.CrosslinkIndex, s.CrosslinkShardLast = d, d, d, d
		s.ValidatorList, s.Delegations, s.ValidatorSnapshots, s.ShardStates = d, d, d, d
		s.EpochBlockNumbers, s.EpochVrf, s.EpochVdf, s.RewardAccumulators = d, d, d, d
		return s
	}
	good := mk()
	if err := good.Validate(); err != nil {
		t.Fatalf("valid set rejected: %v", err)
	}
	// Missing field is a hard failure (plan §11.3).
	bad := mk()
	bad.EpochVrf = Digest{}
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "epoch_vrf") {
		t.Fatalf("missing field must fail naming it, got %v", err)
	}
	// Per-field diff.
	other := mk()
	other.CXSpent = Digest{Count: 9, SHA256: strings.Repeat("ab", 32)}
	diffs := good.Diff(other)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "cx_spent") {
		t.Fatalf("diff wrong: %v", diffs)
	}
}

type failFS struct {
	FS
	failOn string
}

func (f failFS) Sync(fd *os.File) error {
	if strings.Contains(fd.Name(), f.failOn) {
		return errors.New("injected fsync failure")
	}
	return f.FS.Sync(fd)
}

func TestFsyncWalkInjectedFailure(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "a.ldb"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "CURRENT"), []byte("y"), 0o644)

	if err := FsyncWalk(OSFS, root); err != nil {
		t.Fatalf("clean walk: %v", err)
	}
	if err := FsyncWalk(failFS{FS: OSFS, failOn: "a.ldb"}, root); err == nil {
		t.Fatal("injected file fsync failure must surface")
	}
	if err := FsyncWalk(failFS{FS: OSFS, failOn: "sub"}, root); err == nil {
		t.Fatal("injected dir fsync failure must surface")
	}
}
