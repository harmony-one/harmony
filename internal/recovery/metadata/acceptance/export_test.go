package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/integrity"
	"github.com/harmony-one/harmony/internal/recovery/metadata/hmr"
	"github.com/harmony-one/harmony/internal/recovery/metadata/refexport"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

func runExport(t *testing.T, dir, anchorPath, outDir string, opts refexport.Options) int {
	t.Helper()
	opts.DBPath, opts.AnchorPath, opts.OutDir = dir, anchorPath, outDir
	return refexport.Run(context.Background(), opts, os.Stderr)
}

// TestExportByteReproducible: two invocations over one fixture produce
// byte-identical .hmr and reference manifests; the self-check passes; the
// package digest equals an independent sha256 of the .hmr; the run
// checksum file verifies.
func TestExportByteReproducible(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)

	outA := filepath.Join(t.TempDir(), "a")
	outB := filepath.Join(t.TempDir(), "b")
	if code := runExport(t, dir, anchorPath, outA, refexport.Options{}); code != 0 {
		t.Fatalf("export A exit %d", code)
	}
	if code := runExport(t, dir, anchorPath, outB, refexport.Options{}); code != 0 {
		t.Fatalf("export B exit %d", code)
	}
	hmrName := filepath.Join("release", "metadata-30.hmr")
	refName := filepath.Join("release", "metadata-30.reference.json")
	hmrA := readFile(t, filepath.Join(outA, hmrName))
	hmrB := readFile(t, filepath.Join(outB, hmrName))
	if !bytes.Equal(hmrA, hmrB) {
		t.Fatal("two export invocations produced different .hmr bytes")
	}
	refA := readFile(t, filepath.Join(outA, refName))
	refB := readFile(t, filepath.Join(outB, refName))
	if !bytes.Equal(refA, refB) {
		t.Fatal("two export invocations produced different reference manifests")
	}

	// The manifest's package digest equals an independent sha256 of the
	// .hmr; the reference is decodable and its container round-trips.
	m, err := hmr.DecodeManifest(refA)
	if err != nil {
		t.Fatal(err)
	}
	if m.PackageSHA256 != report.SHA256Hex(hmrA) {
		t.Fatalf("manifest package digest %s != sha256(.hmr) %s", m.PackageSHA256, report.SHA256Hex(hmrA))
	}
	if _, err := hmr.Decode(hmrA); err != nil {
		t.Fatalf("emitted .hmr does not decode: %v", err)
	}
	// The internal run-checksum file verifies.
	if err := integrity.Verify(filepath.Join(outA, "release", "run-checksums.sha256")); err != nil {
		t.Fatalf("run checksums do not verify: %v", err)
	}
	// The success reports are published atomically INSIDE release/ with
	// the artifacts; they differ only in run evidence (they share the same
	// artifact digests).
	var repA, repB refexport.Report
	_ = json.Unmarshal(readFile(t, filepath.Join(outA, "release", "export-report.json")), &repA)
	_ = json.Unmarshal(readFile(t, filepath.Join(outB, "release", "export-report.json")), &repB)
	if repA.Artifacts.ManifestSHA != repB.Artifacts.ManifestSHA {
		t.Fatal("reference digests differ across runs")
	}
	if !repA.Determinism.Ran || !repA.Determinism.Passed {
		t.Fatal("determinism self-check must run and pass by default")
	}
	if repA.ExitCode != 0 || repA.Artifacts.ReleaseDir != "release" {
		t.Fatalf("published success report must record exit 0 and the release dir; got exit %d dir %q",
			repA.ExitCode, repA.Artifacts.ReleaseDir)
	}
	// No stray failure report at the out-dir root on a clean run.
	if _, err := os.Stat(filepath.Join(outA, "export-report.json")); !os.IsNotExist(err) {
		t.Fatalf("clean run must not leave a root-level report (stat err %v)", err)
	}
}

func TestExportRefusesOnFatal(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	// Corrupt a retained metadata record: overwrite ss<target-epoch> with
	// junk so the boundary byte-equality check fails (INVALID_RETAINED).
	corruptSS(t, dir)
	out := filepath.Join(t.TempDir(), "out")
	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitInvalidRetained {
		t.Fatalf("export over corrupt ss exit %d, want %d", code, report.ExitInvalidRetained)
	}
	// No release directory emitted on refusal.
	if _, err := os.Stat(filepath.Join(out, "release")); !os.IsNotExist(err) {
		t.Fatal("refused export must not create the release directory")
	}
	// The export report records the refusal.
	var rep refexport.Report
	_ = json.Unmarshal(readFile(t, filepath.Join(out, "export-report.json")), &rep)
	if !rep.Refused {
		t.Fatal("export report must record the refusal")
	}
}

// TestReleaseSealing stages exactly the four release files and seals.
func TestReleaseSealing(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")
	if code := runExport(t, dir, anchorPath, out, refexport.Options{}); code != 0 {
		t.Fatalf("export exit %d", code)
	}
	// Run the audit to produce the fourth release file.
	auditOut := filepath.Join(t.TempDir(), "audit")
	scratch := filepath.Join(t.TempDir(), "scratch")
	if code := runAuditForSeal(t, dir, anchorPath, auditOut, scratch); code != 0 {
		t.Fatalf("audit exit %d", code)
	}

	// Stage exactly the four release files (§3): anchor config, .hmr,
	// reference JSON, audit report.
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"recovery-anchor.json":        anchorPath,
		"metadata-30.hmr":             filepath.Join(out, "release", "metadata-30.hmr"),
		"metadata-30.reference.json":  filepath.Join(out, "release", "metadata-30.reference.json"),
		"abandoned-branch-audit.json": filepath.Join(auditOut, "abandoned-branch-audit.json"),
	}
	var want []string
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(staging, name), readFile(t, src), 0o644); err != nil {
			t.Fatal(err)
		}
		want = append(want, name)
	}
	if err := integrity.Seal(staging, want, "SHA256SUMS"); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	if err := integrity.Verify(filepath.Join(staging, "SHA256SUMS")); err != nil {
		t.Fatalf("verify sealed: %v", err)
	}
	entries, _ := integrity.Parse(filepath.Join(staging, "SHA256SUMS"))
	if len(entries) != 4 {
		t.Fatalf("SHA256SUMS must list exactly 4 files, got %d", len(entries))
	}

	// The reference manifest's anchor_config_sha256 matches the staged
	// recovery-anchor.json.
	m, err := hmr.DecodeManifest(readFile(t, filepath.Join(staging, "metadata-30.reference.json")))
	if err != nil {
		t.Fatal(err)
	}
	stagedAnchorSHA := report.SHA256Hex(readFile(t, filepath.Join(staging, "recovery-anchor.json")))
	if m.AnchorConfigSHA != stagedAnchorSHA {
		t.Fatalf("manifest anchor_config_sha256 %s != staged anchor sha %s", m.AnchorConfigSHA, stagedAnchorSHA)
	}

	// A planted stray (internal report) fails the seal.
	if err := os.WriteFile(filepath.Join(staging, "export-report.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := integrity.Seal(staging, want, "SHA256SUMS"); err == nil {
		t.Fatal("a stray file must fail the seal")
	}
}

// TestDeterminismSelfCheckCatchesFault injects nondeterminism into the
// second derivation's serialization: the self-check must exit 23, emit no
// release artifacts, and write the determinism-diff dumps.
func TestDeterminismSelfCheckCatchesFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")

	refexport.TestMutatePassB = func(hmrBytes, manifest []byte) ([]byte, []byte) {
		// Flip the last .hmr byte on the second pass only.
		mutated := append([]byte(nil), hmrBytes...)
		mutated[len(mutated)-1] ^= 0xff
		return mutated, manifest
	}
	defer func() { refexport.TestMutatePassB = nil }()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitDeterminismMismatch {
		t.Fatalf("self-check fault exit %d, want %d", code, report.ExitDeterminismMismatch)
	}
	if _, err := os.Stat(filepath.Join(out, "release")); !os.IsNotExist(err) {
		t.Fatal("determinism mismatch must not create the release directory")
	}
	for _, dump := range []string{"pass-a.hmr", "pass-b.hmr", "pass-a.reference.json", "pass-b.reference.json"} {
		if _, err := os.Stat(filepath.Join(out, "determinism-diff", dump)); err != nil {
			t.Fatalf("missing determinism diff dump %s: %v", dump, err)
		}
	}
	// The two dumped .hmr files must differ at the reported offset.
	a := readFile(t, filepath.Join(out, "determinism-diff", "pass-a.hmr"))
	b := readFile(t, filepath.Join(out, "determinism-diff", "pass-b.hmr"))
	if bytes.Equal(a, b) {
		t.Fatal("pass-a and pass-b dumps must differ")
	}
}

// TestExportRunOnceRefusesExisting pins the transactional run-once
// contract: an out-dir already holding a release directory is refused
// (exit 15) and the existing artifacts stay byte-untouched.
func TestExportRunOnceRefusesExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")
	if code := runExport(t, dir, anchorPath, out, refexport.Options{}); code != 0 {
		t.Fatalf("first export exit %d", code)
	}
	before := readFile(t, filepath.Join(out, "release", "metadata-30.hmr"))
	if code := runExport(t, dir, anchorPath, out, refexport.Options{}); code != report.ExitBadInvocation {
		t.Fatalf("second export into the same out-dir exit %d, want %d", code, report.ExitBadInvocation)
	}
	after := readFile(t, filepath.Join(out, "release", "metadata-30.hmr"))
	if !bytes.Equal(before, after) {
		t.Fatal("a refused rerun must not touch existing artifacts")
	}
	// No staging leftovers.
	if _, err := os.Stat(filepath.Join(out, ".staging")); !os.IsNotExist(err) {
		t.Fatal("staging directory must not survive a run")
	}
}

// assertNoReleaseNames requires that NOTHING consumer-facing exists in the
// out-dir after a failed publication: no release/ directory and no
// release-named file at the top level. (The export report and diagnostic
// dumps are run evidence, not release names.)
func assertNoReleaseNames(t *testing.T, out string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(out, "release")); !os.IsNotExist(err) {
		t.Fatalf("release directory exists after a failed publication (stat err %v)", err)
	}
	for _, name := range []string{"metadata-30.hmr", "metadata-30.reference.json", "run-checksums.sha256"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("release-named file %s exists at the out-dir top level after a failed publication", name)
		}
	}
}

// TestExportPublishRenameFault pins the atomic-publication contract on the
// rename path: the single os.Rename publishing the release directory fails,
// so the release name must not appear AT ALL — there is no partially
// promoted state to roll back — and the run exits 14 with the report
// written.
func TestExportPublishRenameFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")

	refexport.TestPromoteFault = func(string) error { return errInjectedPromoteFault }
	defer func() { refexport.TestPromoteFault = nil }()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitIO {
		t.Fatalf("publish fault exit %d, want %d", code, report.ExitIO)
	}
	assertNoReleaseNames(t, out)
	// Staging is cleaned up too.
	if _, err := os.Stat(filepath.Join(out, ".staging")); !os.IsNotExist(err) {
		t.Fatal("staging directory must not survive a run")
	}
	// The success report only ever exists inside release/ (which never
	// appeared); the surviving root document is the FAILURE report,
	// carrying exit 14 and no release artifacts.
	var rep refexport.Report
	if err := json.Unmarshal(readFile(t, filepath.Join(out, "export-report.json")), &rep); err != nil {
		t.Fatalf("failure report must be written on a failed publish: %v", err)
	}
	if rep.ExitCode != report.ExitIO || rep.Artifacts.ReleaseDir != "" || rep.Artifacts.HMRFile != "" {
		t.Fatalf("report after failed publish must record exit %d with no release artifacts; got exit %d release %q hmr %q",
			report.ExitIO, rep.ExitCode, rep.Artifacts.ReleaseDir, rep.Artifacts.HMRFile)
	}
}

// TestExportStagedReportWriteFault pins the one-atomic-unit protocol on the
// report side: the success report is staged WITH the artifacts and
// published by the same rename, so a failure to write it blocks the whole
// publication — exit 14, no release names, and the root document is a
// failure report.
func TestExportStagedReportWriteFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")

	refexport.TestReportFault = func(path string) error {
		if filepath.Base(filepath.Dir(path)) == ".staging" {
			return errInjectedReportFault
		}
		return nil
	}
	defer func() { refexport.TestReportFault = nil }()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitIO {
		t.Fatalf("staged-report fault exit %d, want %d", code, report.ExitIO)
	}
	assertNoReleaseNames(t, out)
	if _, err := os.Stat(filepath.Join(out, ".staging")); !os.IsNotExist(err) {
		t.Fatal("staging directory must not survive a run")
	}
	var rep refexport.Report
	if err := json.Unmarshal(readFile(t, filepath.Join(out, "export-report.json")), &rep); err != nil {
		t.Fatalf("failure report must be written when the staged report cannot be: %v", err)
	}
	if rep.ExitCode != report.ExitIO || rep.Artifacts.ReleaseDir != "" {
		t.Fatalf("root document must be a failure report (exit %d, no release dir); got exit %d dir %q",
			report.ExitIO, rep.ExitCode, rep.Artifacts.ReleaseDir)
	}
}

// TestExportFailureReportDoubleFault drives the reviewer's worst case: the
// publication rename fails AND the failure report cannot be written either
// (its root path is occupied by a non-empty directory). Even then no
// success-shaped report is visible anywhere — the success report only ever
// existed inside non-consumer .staging — no release names appear, and the
// process exit code carries the failure.
func TestExportFailureReportDoubleFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(filepath.Join(out, "export-report.json", "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}

	refexport.TestPromoteFault = func(string) error { return errInjectedPromoteFault }
	defer func() { refexport.TestPromoteFault = nil }()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitIO {
		t.Fatalf("double fault exit %d, want %d", code, report.ExitIO)
	}
	assertNoReleaseNames(t, out)
	// No success-shaped report can be visible: release/ (and its report)
	// never appeared, and the root path holds no report document.
	if _, err := os.Stat(filepath.Join(out, "release", "export-report.json")); !os.IsNotExist(err) {
		t.Fatalf("no published report may exist after a failed publish (stat err %v)", err)
	}
	if fi, err := os.Stat(filepath.Join(out, "export-report.json")); err != nil || !fi.IsDir() {
		t.Fatalf("root report path must still be the occupying directory (fi %v err %v)", fi, err)
	}
}

// TestExportVerificationFault pins the pre-publication verification: a
// staged artifact that cannot be verified (any error, not only IsNotExist)
// blocks publication BEFORE anything is visible — the release name never
// appears, exit 14.
func TestExportVerificationFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")

	refexport.TestVerifyFault = func(name string) error {
		if name == "metadata-30.reference.json" {
			return errInjectedVerifyFault
		}
		return nil
	}
	defer func() { refexport.TestVerifyFault = nil }()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitIO {
		t.Fatalf("verification fault exit %d, want %d", code, report.ExitIO)
	}
	assertNoReleaseNames(t, out)
}

// TestExportPublishFaultWithCleanupFault drives BOTH faults at once: the
// publish rename fails AND the staging cleanup afterwards fails too. With
// the atomic protocol a failed cleanup can strand only the non-consumer
// .staging directory — never anything under a release name — and the exit
// code stays the hard I/O failure of the publish.
func TestExportPublishFaultWithCleanupFault(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	out := filepath.Join(t.TempDir(), "out")

	refexport.TestPromoteFault = func(string) error { return errInjectedPromoteFault }
	refexport.TestCleanupFault = func(string) error { return errInjectedCleanupFault }
	defer func() {
		refexport.TestPromoteFault = nil
		refexport.TestCleanupFault = nil
	}()

	code := runExport(t, dir, anchorPath, out, refexport.Options{})
	if code != report.ExitIO {
		t.Fatalf("publish+cleanup fault exit %d, want %d", code, report.ExitIO)
	}
	assertNoReleaseNames(t, out)
	// The stranded staging directory is the WORST outcome this double
	// fault can produce — and it is not a consumer-facing name.
	if _, err := os.Stat(filepath.Join(out, ".staging")); err != nil {
		t.Fatalf("expected the stranded staging dir for this scenario, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "export-report.json")); err != nil {
		t.Fatalf("export report must be written even on failed publish+cleanup: %v", err)
	}
}

var errInjectedPromoteFault = errInjected("injected publish fault")
var errInjectedVerifyFault = errInjected("injected verification fault")
var errInjectedCleanupFault = errInjected("injected cleanup fault")
var errInjectedReportFault = errInjected("injected report-write fault")

type errInjected string

func (e errInjected) Error() string { return string(e) }

// TestGeneratorDeterministic proves two independent generations of the
// fixture chain export byte-identical .hmr and reference manifests (the
// fixture kit is reproducible from the generator — WS7 acceptance).
func TestGeneratorDeterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir1 := buildFixture(t)
	dir2 := buildFixture(t)
	a1 := writeAnchor(t, dir1, fxTarget)
	a2 := writeAnchor(t, dir2, fxTarget)
	out1 := filepath.Join(t.TempDir(), "1")
	out2 := filepath.Join(t.TempDir(), "2")
	if code := runExport(t, dir1, a1, out1, refexport.Options{}); code != 0 {
		t.Fatalf("export 1 exit %d", code)
	}
	if code := runExport(t, dir2, a2, out2, refexport.Options{}); code != 0 {
		t.Fatalf("export 2 exit %d", code)
	}
	h1 := readFile(t, filepath.Join(out1, "release", "metadata-30.hmr"))
	h2 := readFile(t, filepath.Join(out2, "release", "metadata-30.hmr"))
	if !bytes.Equal(h1, h2) {
		t.Fatal("two independent generations produced different .hmr (generator is not deterministic)")
	}
	r1 := readFile(t, filepath.Join(out1, "release", "metadata-30.reference.json"))
	r2 := readFile(t, filepath.Join(out2, "release", "metadata-30.reference.json"))
	if !bytes.Equal(r1, r2) {
		t.Fatal("two independent generations produced different reference manifests")
	}
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}
