package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

// Shared pristine fixtures (built once per variant, copied per test).
var (
	fixturesMu   sync.Mutex
	fixturesRoot string
	fixtures     = map[fixture.Variant]*fixture.Manifest{}
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "preflight-fixtures-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture tmp:", err)
		os.Exit(1)
	}
	fixturesRoot = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// getFixture builds (once) and returns the pristine manifest for a variant.
func getFixture(t *testing.T, v fixture.Variant) *fixture.Manifest {
	t.Helper()
	fixturesMu.Lock()
	defer fixturesMu.Unlock()
	if m, ok := fixtures[v]; ok {
		return m
	}
	dir := filepath.Join(fixturesRoot, fmt.Sprintf("variant-%d", v))
	m, err := fixture.Build(dir, v)
	if err != nil {
		t.Fatalf("build fixture variant %d: %v", v, err)
	}
	fixtures[v] = m
	return m
}

// cloneFixture copies the pristine variant DB into a fresh directory the
// test may mutate.
func cloneFixture(t *testing.T, v fixture.Variant) (*fixture.Manifest, string) {
	t.Helper()
	m := getFixture(t, v)
	dst := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDB(m.Dir, dst); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return m, dst
}

type cliResult struct {
	code    int
	stdout  string
	stderr  string
	receipt *report.Receipt
	report  string
}

// runCLI executes the preflight through the full CLI (cobra flags included).
func runCLI(t *testing.T, m *fixture.Manifest, db string, extra ...string) cliResult {
	t.Helper()
	reportPath := filepath.Join(t.TempDir(), "preflight-result.json")
	args := []string{
		"preflight",
		"--db", db,
		"--network", "localnet",
		"--target-height", fmt.Sprint(fixture.TargetHeight),
		"--target-hash", m.TargetHash.Hex(),
		"--report", reportPath,
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	res := cliResult{code: code, stdout: stdout.String(), stderr: stderr.String(), report: reportPath}
	if data, err := os.ReadFile(reportPath); err == nil {
		var rec report.Receipt
		if err := json.Unmarshal(data, &rec); err != nil {
			t.Fatalf("receipt does not parse: %v\n%s", err, data)
		}
		res.receipt = &rec
	}
	return res
}

func wantExit(t *testing.T, res cliResult, code int) {
	t.Helper()
	if res.code != code {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", res.code, code, res.stdout, res.stderr)
	}
}

func wantFailLine(t *testing.T, res cliResult, substr string) {
	t.Helper()
	wantExit(t, res, report.ExitFail)
	lines := strings.Split(strings.TrimRight(res.stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout must be exactly one line, got %d:\n%s", len(lines), res.stdout)
	}
	if !strings.HasPrefix(lines[0], "FAIL: ") {
		t.Fatalf("stdout line %q does not start with FAIL:", lines[0])
	}
	if !strings.Contains(lines[0], substr) {
		t.Fatalf("FAIL line %q does not mention %q", lines[0], substr)
	}
	if res.receipt == nil {
		t.Fatalf("no receipt written on FAIL")
	}
	if res.receipt.Result != "FAIL" || res.receipt.ExitCode != report.ExitFail {
		t.Fatalf("receipt result/exit = %s/%d, want FAIL/1", res.receipt.Result, res.receipt.ExitCode)
	}
}

// TestPreflightPass is the end-to-end PASS row: full CLI, pristine fixture.
func TestPreflightPass(t *testing.T) {
	m, db := cloneFixture(t, fixture.VariantBase)
	res := runCLI(t, m, db, "--name", "test-validator")
	wantExit(t, res, report.ExitPass)

	if res.stdout != "PASS\n" {
		t.Fatalf("stdout = %q, want exactly \"PASS\\n\"", res.stdout)
	}
	rec := res.receipt
	if rec == nil {
		t.Fatal("no receipt written")
	}
	if rec.Result != "PASS" || rec.ExitCode != 0 {
		t.Fatalf("receipt result=%s exit=%d", rec.Result, rec.ExitCode)
	}
	if rec.Tool != report.Tool || rec.Schema != report.Schema {
		t.Fatalf("receipt tool/schema = %q/%q", rec.Tool, rec.Schema)
	}
	if rec.Name != "test-validator" || rec.Hostname == "" {
		t.Fatalf("receipt name/hostname = %q/%q", rec.Name, rec.Hostname)
	}
	for _, id := range report.CheckIDs {
		if rec.Checks[id] != "ok" {
			t.Fatalf("check %s = %q, want ok (all checks: %v)", id, rec.Checks[id], rec.Checks)
		}
	}
	if rec.Target.Hash != m.TargetHash.Hex() || rec.Target.Height != fixture.TargetHeight {
		t.Fatalf("receipt target %+v", rec.Target)
	}
	if rec.Target.StateRoot != m.StateRoot.Hex() {
		t.Fatalf("receipt state root %s, want %s", rec.Target.StateRoot, m.StateRoot.Hex())
	}
	if rec.Target.Epoch != fixture.Epoch || rec.Target.ViewID != fixture.TargetHeight {
		t.Fatalf("receipt epoch/viewid %d/%d", rec.Target.Epoch, rec.Target.ViewID)
	}
	if rec.CertificateSources.SatisfiedBy != "exact-key+child-header" ||
		!rec.CertificateSources.ExactKeyPresent || !rec.CertificateSources.ChildHeaderPresent {
		t.Fatalf("certificate sources %+v", rec.CertificateSources)
	}
	if rec.HeadSample.WalkToTarget != "reached-target" {
		t.Fatalf("head sample %+v", rec.HeadSample)
	}
	if !strings.Contains(rec.HeadSample.ChildAtTargetPlus, m.ChildHash.Hex()) {
		t.Fatalf("child sample %q", rec.HeadSample.ChildAtTargetPlus)
	}
	c := rec.State.Counts
	if c.Accounts < fixture.NumEOA || c.StorageTries == 0 || c.StorageLeaves == 0 ||
		c.UniqueCodeContract == 0 || c.UniqueCodeValidator == 0 {
		t.Fatalf("state counts %+v", c)
	}
	if rec.State.Digest == "" || rec.State.DigestAlgorithm == "" {
		t.Fatalf("digest missing: %+v", rec.State)
	}
	// The base fixture carries the informational flag-edge and dual-class
	// anomalies; they never gate.
	wantKinds := []string{"flag-decoded-zero", "flag-noncanonical-value", "wrapper-shaped-contract-code", "code-dual-class"}
	for _, k := range wantKinds {
		if rec.State.Anomalies.ByKind[k] == 0 {
			t.Fatalf("expected anomaly kind %s in passing receipt, got %+v", k, rec.State.Anomalies)
		}
	}
	if rec.State.Anomalies.Omitted != 0 {
		t.Fatalf("unexpected omitted anomalies: %+v", rec.State.Anomalies)
	}
	if rec.SampleNote == "" {
		t.Fatal("sample note missing")
	}
	if rec.Retries.ReopenCount != 0 {
		t.Fatalf("unexpected reopens on a stopped fixture: %d", rec.Retries.ReopenCount)
	}
}

// TestPreflightDigestDeterminism: worker-count variation must be
// byte-identical (scheduling-independent fold).
func TestPreflightDigestDeterminism(t *testing.T) {
	m, db := cloneFixture(t, fixture.VariantBase)
	res1 := runCLI(t, m, db, "--storage-workers", "1")
	wantExit(t, res1, report.ExitPass)
	res8 := runCLI(t, m, db, "--storage-workers", "8")
	wantExit(t, res8, report.ExitPass)
	if res1.receipt.State.Digest != res8.receipt.State.Digest {
		t.Fatalf("digest differs across worker counts: %s vs %s",
			res1.receipt.State.Digest, res8.receipt.State.Digest)
	}
	if res1.receipt.State.Counts != res8.receipt.State.Counts {
		t.Fatalf("counts differ across worker counts: %+v vs %+v",
			res1.receipt.State.Counts, res8.receipt.State.Counts)
	}
}
