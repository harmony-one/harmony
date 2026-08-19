package acceptance

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/integrity"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// committedKitDir locates the committed fixture kit from the repo root
// (the .hmy directory anchors the root, same trick as RepoKeysDir).
func committedKitDir(t *testing.T) string {
	t.Helper()
	root := filepath.Dir(metafixture.RepoKeysDir())
	kit := filepath.Join(root, "testdata", "recovery", "metadata", "kit")
	if _, err := os.Stat(kit); err != nil {
		t.Fatalf("committed fixture kit missing (regenerate with scripts/recovery/gen-metadata-fixtures.sh): %v", err)
	}
	return kit
}

// TestCommittedKitGolden pins the committed fixture kit (WS7): a fresh
// in-process export over the committed dirty DB reproduces the committed
// golden .hmr and reference manifest byte-for-byte, the ground-truth
// digests match, and the run checksum file verifies. Any drift between the
// generator, the normalization ruleset and the committed tree fails here.
func TestCommittedKitGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("export over the kit is not short")
	}
	kit := committedKitDir(t)
	dirty := filepath.Join(kit, "harmony_db_0")
	anchorPath := filepath.Join(kit, "recovery-anchor.localnet.json")

	goldenHMR := readFile(t, filepath.Join(kit, "reference", "metadata-30.hmr"))
	goldenRef := readFile(t, filepath.Join(kit, "reference", "metadata-30.reference.json"))

	var truth struct {
		Blocks       uint64 `json:"blocks"`
		TargetHeight uint64 `json:"target_height"`
		HMRSHA256    string `json:"hmr_sha256"`
		RefSHA256    string `json:"reference_sha256"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(kit, "ground-truth.json")), &truth); err != nil {
		t.Fatal(err)
	}
	if truth.Blocks != fxBlocks || truth.TargetHeight != fxTarget {
		t.Fatalf("committed kit shape (blocks %d target %d) does not match the suite constants (%d/%d) — regenerate the kit",
			truth.Blocks, truth.TargetHeight, fxBlocks, fxTarget)
	}
	if got := report.SHA256Hex(goldenHMR); got != truth.HMRSHA256 {
		t.Fatalf("committed .hmr digest %s != ground truth %s", got, truth.HMRSHA256)
	}
	if got := report.SHA256Hex(goldenRef); got != truth.RefSHA256 {
		t.Fatalf("committed reference digest %s != ground truth %s", got, truth.RefSHA256)
	}
	if err := integrity.Verify(filepath.Join(kit, "reference", "run-checksums.sha256")); err != nil {
		t.Fatalf("committed run checksums do not verify: %v", err)
	}

	// Fresh export over the committed DB reproduces the goldens exactly.
	out := filepath.Join(t.TempDir(), "out")
	if code := runExportForAudit(t, dirty, anchorPath, out); code != 0 {
		t.Fatalf("export over the committed kit exit %d", code)
	}
	if !bytes.Equal(readFile(t, filepath.Join(out, "release", "metadata-30.hmr")), goldenHMR) {
		t.Fatal("fresh export .hmr differs from the committed golden (ruleset/generator drift — regenerate the kit and review)")
	}
	if !bytes.Equal(readFile(t, filepath.Join(out, "release", "metadata-30.reference.json")), goldenRef) {
		t.Fatal("fresh export reference manifest differs from the committed golden")
	}
}

// TestKitRegeneratesByteIdentical is the complete-tree reproducibility
// pin (WS7 / F7): regenerating the whole kit into a fresh directory with
// metafixture.GenerateKit (the same code path the `gen` command uses)
// reproduces the committed tree file-for-file and byte-for-byte. This
// covers every artifact — both LevelDB directories (including the *.log
// write-ahead files), the fixture-key *.hex files, the anchor config, the
// golden reference artifacts and ground-truth.json — not just the .hmr and
// manifest. Any generator/ruleset/canonicalization drift fails here and
// tells the operator to regenerate and review the committed kit.
func TestKitRegeneratesByteIdentical(t *testing.T) {
	if testing.Short() {
		t.Skip("kit regeneration is not short")
	}
	kit := committedKitDir(t)
	fresh := filepath.Join(t.TempDir(), "kit")
	if err := metafixture.GenerateKit(fresh, metafixture.RepoKeysDir(), io.Discard); err != nil {
		t.Fatalf("regenerate kit: %v", err)
	}
	assertTreesEqual(t, kit, fresh)
}

// assertTreesEqual fails if the two directory trees differ in the set of
// relative file paths or in any file's bytes.
func assertTreesEqual(t *testing.T, a, b string) {
	t.Helper()
	fa := treeFiles(t, a)
	fb := treeFiles(t, b)
	for rel := range fa {
		if _, ok := fb[rel]; !ok {
			t.Errorf("committed kit has %s, regenerated tree does not", rel)
		}
	}
	for rel := range fb {
		if _, ok := fa[rel]; !ok {
			t.Errorf("regenerated tree has %s, committed kit does not", rel)
		}
		if !bytes.Equal(fa[rel], fb[rel]) {
			t.Errorf("file %s differs between committed and regenerated kit (%d vs %d bytes)", rel, len(fa[rel]), len(fb[rel]))
		}
	}
}

func treeFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[rel] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}
