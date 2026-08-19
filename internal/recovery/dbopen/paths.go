package dbopen

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ValidateOutputPath refuses an output/report/scratch path that resolves
// inside the source DB directory (plan §4.2). Both the raw and the
// symlink-resolved forms are compared; the output's parent may not exist
// yet, so the deepest existing ancestor is resolved and rejoined.
func ValidateOutputPath(outPath, dbPath string) error {
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("dbopen: resolve output path: %w", err)
	}
	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("dbopen: resolve db path: %w", err)
	}
	within := func(base, p string) bool {
		rel, err := filepath.Rel(base, p)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "."
	}
	resolvedDB := absDB
	if r, err := filepath.EvalSymlinks(absDB); err == nil {
		resolvedDB = r
	}
	resolvedOut := absOut
	dir, rest := filepath.Dir(absOut), filepath.Base(absOut)
	for {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolvedOut = filepath.Join(r, rest)
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
	if absOut == absDB || resolvedOut == resolvedDB ||
		within(absDB, absOut) || within(resolvedDB, resolvedOut) {
		return fmt.Errorf("dbopen: output path %s resolves inside the source DB directory %s; choose a path outside the database", outPath, dbPath)
	}
	return nil
}

// Fingerprint is the DB identity snapshot recorded in every report and
// compared before/after for zero-write proofs.
type Fingerprint struct {
	Path        string `json:"path"`
	Device      uint64 `json:"device"`
	Inode       uint64 `json:"inode"`
	Current     string `json:"current"`      // CURRENT file content (trimmed)
	ManifestSHA string `json:"manifest_sha"` // SHA-256 of the manifest named by CURRENT
	FileListSHA string `json:"file_list_sha"`
	FileCount   int    `json:"file_count"`
}

// FingerprintDir captures the identity of a database directory. The file
// list hash covers name+size+mtime of every regular file, so any mutation
// (including a created LOCK or rewritten MANIFEST) changes it.
func FingerprintDir(path string) (*Fingerprint, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	fp := &Fingerprint{Path: resolved}
	fp.Device, fp.Inode, err = statDevIno(resolved)
	if err != nil {
		return nil, err
	}
	current, err := os.ReadFile(filepath.Join(resolved, "CURRENT"))
	if err != nil {
		return nil, err
	}
	fp.Current = strings.TrimSpace(string(current))
	manifest, err := os.ReadFile(filepath.Join(resolved, fp.Current))
	if err != nil {
		return nil, err
	}
	msum := sha256.Sum256(manifest)
	fp.ManifestSHA = fmt.Sprintf("%x", msum[:])

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		names = append(names, fmt.Sprintf("%s|%d|%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(names)
	lsum := sha256.Sum256([]byte(strings.Join(names, "\n")))
	fp.FileListSHA = fmt.Sprintf("%x", lsum[:])
	fp.FileCount = len(names)
	return fp, nil
}

// Equal compares the mutation-relevant fields of two fingerprints,
// including device/inode identity (a swapped-out directory with identical
// content is still a different database).
func (f *Fingerprint) Equal(o *Fingerprint) bool {
	return f.Device == o.Device && f.Inode == o.Inode &&
		f.Current == o.Current && f.ManifestSHA == o.ManifestSHA &&
		f.FileListSHA == o.FileListSHA && f.FileCount == o.FileCount
}
