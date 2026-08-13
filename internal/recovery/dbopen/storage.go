// Package dbopen is the metadata commands' strict, fail-closed read-only
// LevelDB opener (plan §4.2). Both stock paths are unsafe: geth's wrapper
// calls leveldb.RecoverFile on ErrCorrupted even in read-only mode, and
// direct goleveldb storage.OpenFile(path, true) O_CREATEs a missing LOCK.
//
// Intentional contrast with preflight's landed opener
// (internal/recovery/inplace/rodb): preflight targets a LIVE database, so
// its storage never touches LOCK and it retries live-file races. The
// metadata commands target a STOPPED node: this opener takes a SHARED
// flock on the existing LOCK file as a writer guard (a running node holds
// LOCK_EX and fails the open) and performs zero retries. Separate
// implementations by design (plan §4.2).
package dbopen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/storage"
)

// ErrWriteRefused is returned by every mutating storage or KV method.
var ErrWriteRefused = errors.New("dbopen: write refused (strict read-only handle)")

// ErrConcurrentWriter reports a live process holding the database LOCK.
var ErrConcurrentWriter = errors.New("dbopen: another process holds the database lock (node still running?)")

// ErrMissingLock reports a database directory without a LOCK file — either
// not a LevelDB or a damaged one; the strict opener never creates it.
var ErrMissingLock = errors.New("dbopen: LOCK file missing (never created by this tool)")

// strictStorage implements goleveldb's storage.Storage over an existing DB
// directory:
//
//   - Lock() opens the EXISTING <dir>/LOCK with O_RDONLY|O_NOFOLLOW — never
//     O_CREATE; a missing or concurrently-deleted LOCK errors at the single
//     atomic open(2) (no check-then-open TOCTOU window) — then takes a
//     shared flock (a concurrent writer holding LOCK_EX fails the open);
//   - Open/List/GetMeta are plain file reads;
//   - Create/Remove/Rename/SetMeta refuse; Log is a no-op.
type strictStorage struct {
	dir string

	mu     sync.Mutex
	locked bool
	closed bool
	lockF  *os.File
}

var _ storage.Storage = (*strictStorage)(nil)

func newStrictStorage(dir string) *strictStorage {
	return &strictStorage{dir: dir}
}

type strictLock struct{ s *strictStorage }

func (l *strictLock) Unlock() {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	l.s.releaseLockLocked()
}

func (s *strictStorage) releaseLockLocked() {
	if s.lockF != nil {
		_ = funlock(s.lockF)
		_ = s.lockF.Close()
		s.lockF = nil
	}
	s.locked = false
}

// Lock opens the existing LOCK file read-only (single atomic open, no
// create) and acquires a shared flock. Unit-testable directly: it only
// errors, it never creates a file.
func (s *strictStorage) Lock() (storage.Locker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, storage.ErrClosed
	}
	if s.locked {
		return nil, storage.ErrLocked
	}
	f, err := openNoFollowReadOnly(filepath.Join(s.dir, "LOCK"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrMissingLock, filepath.Join(s.dir, "LOCK"))
		}
		return nil, fmt.Errorf("dbopen: open LOCK: %w", err)
	}
	if err := flockShared(f); err != nil {
		_ = f.Close()
		if errors.Is(err, errWouldBlock) {
			return nil, ErrConcurrentWriter
		}
		return nil, fmt.Errorf("dbopen: flock LOCK: %w", err)
	}
	s.lockF = f
	s.locked = true
	return &strictLock{s: s}, nil
}

// Log is a no-op (the stock storage appends to the LOG file).
func (s *strictStorage) Log(str string) {}

// SetMeta would rewrite CURRENT; refused.
func (s *strictStorage) SetMeta(fd storage.FileDesc) error { return ErrWriteRefused }

// GetMeta returns the manifest descriptor named by CURRENT.
func (s *strictStorage) GetMeta() (storage.FileDesc, error) {
	if err := s.ok(); err != nil {
		return storage.FileDesc{}, err
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, "CURRENT"))
	if err != nil {
		return storage.FileDesc{}, err
	}
	content := strings.TrimRight(string(raw), "\r\n ")
	var num int64
	if _, err := fmt.Sscanf(content, "MANIFEST-%d", &num); err != nil || num < 0 {
		return storage.FileDesc{}, fmt.Errorf("dbopen: malformed CURRENT content %q", content)
	}
	fd := storage.FileDesc{Type: storage.TypeManifest, Num: num}
	if _, err := os.Stat(filepath.Join(s.dir, fsGenName(fd))); err != nil {
		return storage.FileDesc{}, err
	}
	return fd, nil
}

// List returns descriptors matching the type mask per the stock file
// naming rules (LOCK, LOG, CURRENT* do not parse and are skipped).
func (s *strictStorage) List(ft storage.FileType) ([]storage.FileDesc, error) {
	if err := s.ok(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	seen := make(map[storage.FileDesc]bool)
	var fds []storage.FileDesc
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if fd, ok := fsParseName(ent.Name()); ok && fd.Type&ft != 0 && !seen[fd] {
			seen[fd] = true
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

// Open opens the named file read-only.
func (s *strictStorage) Open(fd storage.FileDesc) (storage.Reader, error) {
	if err := s.ok(); err != nil {
		return nil, err
	}
	if !storage.FileDescOk(fd) {
		return nil, storage.ErrInvalidFile
	}
	f, err := os.Open(filepath.Join(s.dir, fsGenName(fd)))
	if os.IsNotExist(err) && fd.Type == storage.TypeTable {
		// Tables written by older goleveldb use the .sst suffix.
		f, err = os.Open(filepath.Join(s.dir, fsGenOldName(fd)))
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *strictStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	return nil, ErrWriteRefused
}

func (s *strictStorage) Remove(fd storage.FileDesc) error { return ErrWriteRefused }

func (s *strictStorage) Rename(oldfd, newfd storage.FileDesc) error { return ErrWriteRefused }

func (s *strictStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseLockLocked()
	s.closed = true
	return nil
}

func (s *strictStorage) ok() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.ErrClosed
	}
	return nil
}

// fsGenName/fsGenOldName/fsParseName mirror goleveldb's file storage naming
// rules (leveldb/storage/file_storage.go).
func fsGenName(fd storage.FileDesc) string {
	switch fd.Type {
	case storage.TypeManifest:
		return fmt.Sprintf("MANIFEST-%06d", fd.Num)
	case storage.TypeJournal:
		return fmt.Sprintf("%06d.log", fd.Num)
	case storage.TypeTable:
		return fmt.Sprintf("%06d.ldb", fd.Num)
	case storage.TypeTemp:
		return fmt.Sprintf("%06d.tmp", fd.Num)
	default:
		panic("dbopen: invalid file type")
	}
}

func fsGenOldName(fd storage.FileDesc) string {
	if fd.Type == storage.TypeTable {
		return fmt.Sprintf("%06d.sst", fd.Num)
	}
	return fsGenName(fd)
}

func fsParseName(name string) (fd storage.FileDesc, ok bool) {
	var tail string
	_, err := fmt.Sscanf(name, "%d.%s", &fd.Num, &tail)
	if err == nil {
		switch tail {
		case "log":
			fd.Type = storage.TypeJournal
		case "ldb", "sst":
			fd.Type = storage.TypeTable
		case "tmp":
			fd.Type = storage.TypeTemp
		default:
			return storage.FileDesc{}, false
		}
		return fd, true
	}
	n, _ := fmt.Sscanf(name, "MANIFEST-%d%s", &fd.Num, &tail)
	if n == 1 {
		fd.Type = storage.TypeManifest
		return fd, true
	}
	return storage.FileDesc{}, false
}
