package acceptance

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// TestScanFallbackMissingSS: deleting ss<target-epoch> is a clean-DB
// fallback signal -> exit 20 (MISSING_REQUIRED_METADATA).
func TestScanFallbackMissingSS(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	copyDir := t.TempDir() + "/harmony_db_0"
	if err := fixture.CopyDir(dir, copyDir); err != nil {
		t.Fatal(err)
	}
	deleteKey(t, copyDir, append([]byte("ss"), big.NewInt(2).Bytes()...))
	code, rep := runScan(t, copyDir, anchorPath)
	if code != report.ExitMissingRequired {
		t.Fatalf("deleted ss exit %d, want 20 (findings %+v)", code, rep.Findings.Items)
	}
	if rep.Verdict != "MISSING_REQUIRED_METADATA" {
		t.Fatalf("verdict %s", rep.Verdict)
	}
}

// TestScanFallbackMissingBlkRwd: deleting blk-rwd-<target> -> exit 20.
func TestScanFallbackMissingBlkRwd(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	copyDir := t.TempDir() + "/harmony_db_0"
	if err := fixture.CopyDir(dir, copyDir); err != nil {
		t.Fatal(err)
	}
	deleteKey(t, copyDir, append([]byte("blk-rwd-"), be8(fxTarget)...))
	code, _ := runScan(t, copyDir, anchorPath)
	if code != report.ExitMissingRequired {
		t.Fatalf("deleted blk-rwd exit %d, want 20", code)
	}
}

// TestScanCorruptShardStateInvalid: a well-formed-key but wrong-value
// ss<target-epoch> is fatal corruption -> exit 21.
func TestScanCorruptShardStateInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	copyDir := t.TempDir() + "/harmony_db_0"
	if err := fixture.CopyDir(dir, copyDir); err != nil {
		t.Fatal(err)
	}
	corruptSS(t, copyDir)
	code, _ := runScan(t, copyDir, anchorPath)
	if code != report.ExitInvalidRetained {
		t.Fatalf("corrupt ss exit %d, want 21", code)
	}
}

func be8(n uint64) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(n)
		n >>= 8
	}
	return b
}

func deleteKey(t *testing.T, dir string, key []byte) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(key); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
