package fixture_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
)

// TestBuildByteReproducible: two independent generations of the same
// variant must produce byte-identical database trees (the canonical
// rewrite makes LevelDB sequence numbers a pure function of the content
// and drops the timestamped LOG).
func TestBuildByteReproducible(t *testing.T) {
	dirA := filepath.Join(t.TempDir(), "harmony_db_0")
	dirB := filepath.Join(t.TempDir(), "harmony_db_0")
	ma, err := fixture.Build(dirA, fixture.VariantBase)
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	mb, err := fixture.Build(dirB, fixture.VariantBase)
	if err != nil {
		t.Fatalf("build B: %v", err)
	}
	if ma.TargetHash != mb.TargetHash || ma.StateRoot != mb.StateRoot {
		t.Fatalf("logical content differs: %s/%s vs %s/%s",
			ma.TargetHash.Hex(), ma.StateRoot.Hex(), mb.TargetHash.Hex(), mb.StateRoot.Hex())
	}
	CompareTrees(t, dirA, dirB)
}

// CompareTrees fails the test unless the two flat directories hold the same
// file names with byte-identical contents.
func CompareTrees(t *testing.T, a, b string) {
	t.Helper()
	filesA := readTree(t, a)
	filesB := readTree(t, b)
	for name := range filesB {
		if _, ok := filesA[name]; !ok {
			t.Errorf("file %s only in %s", name, b)
		}
	}
	for name, data := range filesA {
		got, ok := filesB[name]
		if !ok {
			t.Errorf("file %s only in %s", name, a)
			continue
		}
		if !bytes.Equal(data, got) {
			t.Errorf("file %s differs (%d vs %d bytes)", name, len(data), len(got))
		}
	}
}

func readTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, ent := range entries {
		if ent.IsDir() {
			t.Fatalf("unexpected subdirectory %s in %s", ent.Name(), dir)
		}
		data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[ent.Name()] = data
	}
	return files
}
