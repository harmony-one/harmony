// Package integrity implements the plain-SHA-256 integrity layer that
// replaces signing (operator decision, plan §4 "Integrity and
// hash-chaining"): sibling <file>.sha256 checksum files, directory-level
// SHA256SUMS, and report hash-chaining. It protects against corruption,
// mix-ups, and stale inputs — not against a motivated tamperer with
// filesystem access (residual risk operator-accepted, recorded in the
// runbook).
package integrity

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileSHA256 streams a file and returns its lowercase hex SHA-256.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("integrity: open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("integrity: hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BytesSHA256 returns the lowercase hex SHA-256 of b.
func BytesSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ChecksumPath returns the sibling checksum file path for a file.
func ChecksumPath(path string) string { return path + ".sha256" }

// WriteChecksumFile writes the sibling <file>.sha256 in sha256sum format
// ("<hex>  <basename>\n") and fsyncs it.
func WriteChecksumFile(path string) (string, error) {
	sum, err := FileSHA256(path)
	if err != nil {
		return "", err
	}
	line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(path))
	if err := writeFileSync(ChecksumPath(path), []byte(line)); err != nil {
		return "", err
	}
	return sum, nil
}

// VerifyChecksumFile recomputes the file hash and compares it against the
// sibling checksum file. Any mismatch or missing sidecar is an error.
func VerifyChecksumFile(path string) (string, error) {
	want, err := readChecksumSidecar(ChecksumPath(path), filepath.Base(path))
	if err != nil {
		return "", err
	}
	got, err := FileSHA256(path)
	if err != nil {
		return "", err
	}
	if got != want {
		return "", fmt.Errorf("integrity: checksum mismatch for %s: recorded %s, recomputed %s", path, want, got)
	}
	return got, nil
}

func readChecksumSidecar(sidecar, wantName string) (string, error) {
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		return "", fmt.Errorf("integrity: read checksum sidecar %s: %w", sidecar, err)
	}
	line := strings.TrimSpace(string(raw))
	sum, name, ok := parseSumLine(line)
	if !ok {
		return "", fmt.Errorf("integrity: malformed checksum sidecar %s", sidecar)
	}
	if name != wantName {
		return "", fmt.Errorf("integrity: checksum sidecar %s names %q, want %q", sidecar, name, wantName)
	}
	return sum, nil
}

func parseSumLine(line string) (sum, name string, ok bool) {
	// sha256sum format: "<64 hex>  <name>" (two spaces; or " *" for binary).
	if len(line) < 64+2+1 {
		return "", "", false
	}
	sum = strings.ToLower(line[:64])
	if _, err := hex.DecodeString(sum); err != nil {
		return "", "", false
	}
	rest := line[64:]
	if !strings.HasPrefix(rest, "  ") && !strings.HasPrefix(rest, " *") {
		return "", "", false
	}
	name = rest[2:]
	if name == "" {
		return "", "", false
	}
	return sum, name, true
}

// VerifyRecorded recomputes a file's SHA-256 and refuses if it disagrees
// with the recorded chain value (the checksum-gate primitive).
func VerifyRecorded(path, recordedHex string) error {
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, recordedHex) {
		return fmt.Errorf("integrity: checksum gate failed for %s: recorded %s, recomputed %s", path, recordedHex, got)
	}
	return nil
}

// SumsEntry is one line of a SHA256SUMS file.
type SumsEntry struct {
	SHA256 string
	Name   string // relative path, forward slashes
}

// WriteSums writes a SHA256SUMS file (sha256sum flat format, sorted by
// name) and fsyncs it.
func WriteSums(path string, entries []SumsEntry) error {
	sorted := append([]SumsEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var b strings.Builder
	for _, e := range sorted {
		if strings.Contains(e.Name, "\n") {
			return fmt.Errorf("integrity: refusing newline in SHA256SUMS name %q", e.Name)
		}
		fmt.Fprintf(&b, "%s  %s\n", strings.ToLower(e.SHA256), e.Name)
	}
	return writeFileSync(path, []byte(b.String()))
}

// ReadSums parses a SHA256SUMS file.
func ReadSums(path string) ([]SumsEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("integrity: open %s: %w", path, err)
	}
	defer f.Close()
	var out []SumsEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	ln := 0
	for sc.Scan() {
		ln++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		sum, name, ok := parseSumLine(line)
		if !ok {
			return nil, fmt.Errorf("integrity: malformed line %d in %s", ln, path)
		}
		out = append(out, SumsEntry{SHA256: sum, Name: name})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("integrity: scan %s: %w", path, err)
	}
	return out, nil
}

// InputRef is one hash-chain link: a consumed report/manifest bound by name,
// path (informational) and SHA-256. Every phase report embeds the refs of
// every input it consumed, making the anchor → … → release.json chain
// verifiable end to end.
type InputRef struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// NewInputRef hashes path and builds the chain link.
func NewInputRef(name, path string) (InputRef, error) {
	sum, err := FileSHA256(path)
	if err != nil {
		return InputRef{}, err
	}
	return InputRef{Name: name, Path: path, SHA256: sum}, nil
}

// VerifyInputRef recomputes the file hash behind a chain link and refuses on
// any disagreement (checksum gate).
func VerifyInputRef(ref InputRef) error {
	if ref.SHA256 == "" {
		return fmt.Errorf("integrity: input ref %q has no recorded sha256 (missing chain link)", ref.Name)
	}
	return VerifyRecorded(ref.Path, ref.SHA256)
}

// SelfSHA256 hashes the running executable — the producing-tool binary
// identity recorded in reports and the recovery marker.
func SelfSHA256() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("integrity: locate executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("integrity: resolve executable: %w", err)
	}
	return FileSHA256(resolved)
}

func writeFileSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("integrity: create %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("integrity: write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("integrity: fsync %s: %w", path, err)
	}
	return f.Close()
}
