package integrity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChecksumFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := WriteChecksumFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyChecksumFile(path)
	if err != nil || got != sum {
		t.Fatalf("verify: %v (%s vs %s)", err, got, sum)
	}
	// Single-byte corruption detected.
	if err := os.WriteFile(path, []byte(`{"a":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyChecksumFile(path); err == nil {
		t.Fatal("corrupted file must fail its checksum gate")
	}
	// Missing sidecar is an error.
	os.Remove(ChecksumPath(path))
	if _, err := VerifyChecksumFile(path); err == nil {
		t.Fatal("missing sidecar must fail")
	}
}

func TestInputRefChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	os.WriteFile(path, []byte("payload"), 0o644)
	ref, err := NewInputRef("input", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInputRef(ref); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("tampered"), 0o644)
	if err := VerifyInputRef(ref); err == nil {
		t.Fatal("chain link over a changed file must fail")
	}
	if err := VerifyInputRef(InputRef{Name: "x", Path: path}); err == nil {
		t.Fatal("missing recorded sha must fail (missing chain link)")
	}
}

func TestSums(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SHA256SUMS")
	entries := []SumsEntry{
		{SHA256: strings.Repeat("ab", 32), Name: "payload/b.ldb"},
		{SHA256: strings.Repeat("cd", 32), Name: "a.json"},
	}
	if err := WriteSums(path, entries); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSums(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "a.json" {
		t.Fatalf("sums not sorted/parsed: %+v", got)
	}
	// Newline injection refused.
	if err := WriteSums(path, []SumsEntry{{SHA256: strings.Repeat("ab", 32), Name: "evil\nname"}}); err == nil {
		t.Fatal("newline in name must refuse")
	}
	// Malformed line rejected.
	os.WriteFile(path, []byte("nothex  file\n"), 0o644)
	if _, err := ReadSums(path); err == nil {
		t.Fatal("malformed line must fail")
	}
}
