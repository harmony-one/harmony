package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

const goldenDir = "../../testdata/recovery/preflight/golden"

// normalizeReceipt zeroes the volatile fields (host identity, timing, build
// stamp, machine paths); everything else in the fixture receipts is
// deterministic, including the state digest.
func normalizeReceipt(rec report.Receipt) report.Receipt {
	rec.Hostname = "<host>"
	rec.DBPath = "<db>"
	rec.StartedAt = "<time>"
	rec.DurationS = 0
	rec.Build = report.Build{}
	if rec.ExitCode == report.ExitReadError {
		// Table numbering/offsets in goleveldb error text can vary with
		// compaction scheduling.
		rec.FailReason = "<read-error>"
	}
	return rec
}

func checkGolden(t *testing.T, name string, rec *report.Receipt) {
	t.Helper()
	if rec == nil {
		t.Fatal("no receipt")
	}
	got, err := json.MarshalIndent(normalizeReceipt(*rec), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(goldenDir, name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden %s updated", name)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden %s missing (run with UPDATE_GOLDEN=1): %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("receipt drifted from golden %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// TestGoldenReceipts pins the full receipt shape for PASS, a FAIL class and
// a read error against committed goldens.
func TestGoldenReceipts(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		res := runCLI(t, m, db, "--name", "golden-validator")
		wantExit(t, res, report.ExitPass)
		checkGolden(t, "receipt-pass.json", res.receipt)
	})
	t.Run("fail-state-walk", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, m.StateRoot.Bytes()))
		res := runCLI(t, m, db, "--name", "golden-validator")
		wantExit(t, res, report.ExitFail)
		checkGolden(t, "receipt-fail-state-walk.json", res.receipt)
	})
	t.Run("read-error", func(t *testing.T) {
		// Corrupt every table so the very first read fails: the receipt is
		// then independent of the (scheduling-dependent) physical table
		// layout - all checks stay "skipped".
		m, db := cloneFixture(t, fixture.VariantBase)
		corruptAllSSTs(t, db)
		res := runCLI(t, m, db, "--name", "golden-validator")
		wantExit(t, res, report.ExitReadError)
		if res.receipt.Retries.ReopenCount != 0 {
			t.Fatalf("corrupt table must not retry: %+v", res.receipt.Retries)
		}
		checkGolden(t, "receipt-read-error.json", res.receipt)
	})
}

// TestCommittedFixtureAgreement runs the CLI against the committed
// materialized base fixture (testdata/recovery/preflight/base) and pins the
// result to the golden PASS receipt digest - tying the committed fixtures,
// the in-test generator and the goldens together.
func TestCommittedFixtureAgreement(t *testing.T) {
	committed := "../../testdata/recovery/preflight/base/harmony_db_0"
	if _, err := os.Stat(committed); err != nil {
		t.Fatalf("materialized fixtures missing (%v); regenerate with scripts/recovery/gen-preflight-fixtures.sh", err)
	}
	var goldenRec report.Receipt
	goldenRaw, err := os.ReadFile(filepath.Join(goldenDir, "receipt-pass.json"))
	if err != nil {
		t.Fatalf("golden receipt missing: %v", err)
	}
	if err := json.Unmarshal(goldenRaw, &goldenRec); err != nil {
		t.Fatal(err)
	}
	// Complete-tree reproducibility: the committed fixture must be
	// byte-identical to a fresh hermetic generation (the canonical rewrite
	// in fixture.Build guarantees this; a mismatch means the committed
	// copies are stale - regenerate them).
	m := getFixture(t, fixture.VariantBase)
	fresh, comm := snapshotDir(t, m.Dir), snapshotDir(t, committed)
	for name := range comm {
		if _, ok := fresh[name]; !ok {
			t.Fatalf("committed fixture has extra file %s; regenerate with scripts/recovery/gen-preflight-fixtures.sh", name)
		}
	}
	for name, data := range fresh {
		got, ok := comm[name]
		if !ok {
			t.Fatalf("committed fixture missing %s; regenerate with scripts/recovery/gen-preflight-fixtures.sh", name)
		}
		if !bytes.Equal(data, got) {
			t.Fatalf("committed fixture file %s differs from a fresh generation (%d vs %d bytes); regenerate with scripts/recovery/gen-preflight-fixtures.sh",
				name, len(got), len(data))
		}
	}

	// Copy so the run cannot disturb the committed fixture.
	dst := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDB(committed, dst); err != nil {
		t.Fatal(err)
	}
	res := runCLI(t, m, dst, "--name", "golden-validator")
	wantExit(t, res, report.ExitPass)
	if res.receipt.State.Digest != goldenRec.State.Digest {
		t.Fatalf("committed fixture digest %s != golden %s", res.receipt.State.Digest, goldenRec.State.Digest)
	}
	if res.receipt.Target.Hash != goldenRec.Target.Hash {
		t.Fatalf("committed fixture target %s != golden %s", res.receipt.Target.Hash, goldenRec.Target.Hash)
	}
}

// TestDocMatchesFlags asserts the one-page doc lists every visible
// preflight flag and mentions no flag that does not exist (hidden test-only
// flags may appear in the test-only note).
func TestDocMatchesFlags(t *testing.T) {
	docPath := "../../docs/recovery/preflight.md"
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("doc missing: %v", err)
	}
	doc := string(raw)

	known := map[string]bool{}
	cmd := newPreflightCommand()
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		known[f.Name] = true
		if !f.Hidden && !strings.Contains(doc, "--"+f.Name) {
			t.Errorf("visible flag --%s is not documented in %s", f.Name, docPath)
		}
	})

	for _, match := range flagTokenRe.FindAllStringSubmatch(doc, -1) {
		name := match[1]
		if !known[name] {
			t.Errorf("doc mentions unknown flag --%s", name)
		}
	}
}

var flagTokenRe = regexp.MustCompile(`--([a-z][a-z0-9-]+)`)
