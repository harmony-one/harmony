package acceptance

import (
	"context"
	"math/big"
	"os"
	"testing"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	"github.com/harmony-one/harmony/internal/recovery/metadata/refexport"
)

// corruptSS overwrites ss<target-epoch> with junk (cold-DB surgery). Epoch
// 2 canonical suffix is 0x02.
func corruptSS(t *testing.T, dir string) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	key := append([]byte("ss"), big.NewInt(2).Bytes()...)
	if err := db.Put(key, []byte("corrupted-shard-state")); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

// runExportForAudit runs export-reference into outDir (for tests that
// need the reference manifest artifact).
func runExportForAudit(t *testing.T, dir, anchorPath, outDir string) int {
	t.Helper()
	return refexport.Run(context.Background(), refexport.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir,
	}, os.Stderr)
}

// runAuditForSeal runs a CI-scoped audit (reserve gate skipped).
func runAuditForSeal(t *testing.T, dir, anchorPath, outDir, scratch string) int {
	t.Helper()
	return audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir, Scratch: scratch,
		SkipReserveCheckForTest: true,
	}, os.Stderr)
}
