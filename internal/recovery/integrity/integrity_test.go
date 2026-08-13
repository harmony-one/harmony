package integrity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", "alpha")
	write(t, dir, "b.txt", "beta")
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := WriteSums(sums, []string{"a.txt", "b.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := Verify(sums); err != nil {
		t.Fatalf("verify clean: %v", err)
	}
	// Tamper -> verify fails.
	write(t, dir, "a.txt", "ALPHA")
	if err := Verify(sums); err == nil {
		t.Fatal("verify should fail after tamper")
	}
}

func TestFormatIsSha256sumCompatible(t *testing.T) {
	out := Format([]Entry{{SHA256Hex: "ab", Name: "z.txt"}, {SHA256Hex: "cd", Name: "a.txt"}})
	// Sorted by name; two-space separator; trailing newline per line.
	want := "cd  a.txt\nab  z.txt\n"
	if string(out) != want {
		t.Fatalf("format = %q, want %q", out, want)
	}
}

func TestSealExactFilesOnly(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"one.json", "two.hmr", "three.json", "four.json"} {
		write(t, dir, n, n)
	}
	want := []string{"one.json", "two.hmr", "three.json", "four.json"}
	if err := Seal(dir, want, "SHA256SUMS"); err != nil {
		t.Fatalf("seal clean: %v", err)
	}
	// Verify the sealed sums.
	if err := Verify(filepath.Join(dir, "SHA256SUMS")); err != nil {
		t.Fatalf("verify sealed: %v", err)
	}
	entries, _ := Parse(filepath.Join(dir, "SHA256SUMS"))
	if len(entries) != 4 {
		t.Fatalf("sealed %d entries, want exactly 4", len(entries))
	}

	// A planted stray fails the seal.
	write(t, dir, "stray.txt", "stray")
	if err := Seal(dir, want, "SHA256SUMS"); err == nil || !strings.Contains(err.Error(), "stray") {
		t.Fatalf("stray must fail the seal, got %v", err)
	}
}

func TestSealRegeneratesNoStaleEntry(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "audit.json", "v1")
	write(t, dir, "ref.json", "ref")
	files := []string{"audit.json", "ref.json"}
	if err := Seal(dir, files, "SHA256SUMS"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	// The audit file changes; re-sealing must regenerate (no stale entry).
	write(t, dir, "audit.json", "v2-different-content")
	if err := Seal(dir, files, "SHA256SUMS"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if string(first) == string(second) {
		t.Fatal("re-seal did not regenerate the checksum after content change")
	}
	if err := Verify(filepath.Join(dir, "SHA256SUMS")); err != nil {
		t.Fatalf("verify after re-seal: %v", err)
	}
}

func TestSealMissingFileFails(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "present.json", "x")
	err := Seal(dir, []string{"present.json", "absent.json"}, "SHA256SUMS")
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing file must fail the seal, got %v", err)
	}
}
