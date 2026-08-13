package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// FS abstracts the two syscalls FsyncWalk needs so tests can inject
// failures (plan §4 "Durability": unit-tested with an injected-failure fs
// wrapper).
type FS interface {
	Open(path string) (*os.File, error)
	Sync(f *os.File) error
}

type osFS struct{}

func (osFS) Open(path string) (*os.File, error) { return os.Open(path) }
func (osFS) Sync(f *os.File) error              { return f.Sync() }

// OSFS is the real filesystem.
var OSFS FS = osFS{}

// FsyncWalk fsyncs every regular file and directory under root (files first,
// then directories bottom-up, then root itself). goleveldb writes are not
// per-write fsynced and nofreezedb.Sync() is unsupported (plan §2.2.8), so
// this explicit walk is the durability step of every mutating command.
func FsyncWalk(fs FS, root string) error {
	var files, dirs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		} else if info.Mode().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("report: fsync-walk %s: %w", root, err)
	}
	// Deepest directories first so parents are synced after children.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, group := range [][]string{files, dirs} {
		for _, p := range group {
			f, err := fs.Open(p)
			if err != nil {
				return fmt.Errorf("report: fsync-walk open %s: %w", p, err)
			}
			if err := fs.Sync(f); err != nil {
				f.Close()
				return fmt.Errorf("report: fsync-walk sync %s: %w", p, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("report: fsync-walk close %s: %w", p, err)
			}
		}
	}
	return nil
}

func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("report: open dir %s for fsync: %w", dir, err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("report: fsync dir %s: %w", dir, err)
	}
	return nil
}

func newBytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
