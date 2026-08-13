// Package rodb opens a (possibly live) harmony_db_0 LevelDB strictly
// read-only, without taking the OS flock and without ever writing to the
// database directory.
//
// Never-write property, three layers:
//  1. goleveldb opened with opt.Options{ReadOnly: true, ErrorIfMissing: true}
//  2. the custom storage.Storage refuses Create/Remove/Rename/SetMeta and
//     never opens, creates or probes the LOCK and LOG files
//  3. the ethdb adapter refuses Put/Delete/Compact and returns
//     write-refusing batches
//
// leveldb.RecoverFile is never called (a RecoverFile against a live
// validator DB rewrites the MANIFEST on disk and corrupts the node).
package rodb

import (
	"errors"
	"fmt"
	"os"

	lverrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

// ErrWriteRefused is returned by every write path of the read-only storage
// and the ethdb adapter.
var ErrWriteRefused = errors.New("rodb: write refused (recovery preflight is strictly read-only)")

// LayoutError marks an unusable/unsupported database layout (exit code 2).
type LayoutError struct {
	Reason string
}

func (e *LayoutError) Error() string { return "unsupported database layout: " + e.Reason }

// ReadError marks a persistent read error (exit code 3). Remedy: re-run; if
// it keeps failing, stop the node briefly and re-run.
type ReadError struct {
	Err     error
	Detail  string // e.g. the corrupt table file name
	Retries int    // reopen attempts consumed before giving up
}

func (e *ReadError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("persistent read error (%s): %v", e.Detail, e.Err)
	}
	return fmt.Sprintf("persistent read error: %v", e.Err)
}

func (e *ReadError) Unwrap() error { return e.Err }

// Class is the retry classification of a read error observed on a live DB.
type Class int

const (
	// ClassNone: no error.
	ClassNone Class = iota
	// ClassRetryableRace: exactly the error classes a concurrent live
	// writer can cause: (a) ENOENT on a referenced file (compaction deleted
	// a table under our pinned manifest), and (b) ErrCorrupted attributed
	// to journal or manifest files (torn .log tail during in-memory journal
	// recovery, CURRENT/MANIFEST rotation mid-open).
	ClassRetryableRace
	// ClassCorruptTable: a checksum/content error inside an existing
	// immutable SST. No live-writer race explains it; never retried.
	ClassCorruptTable
	// ClassPersistent: everything else (EACCES, EIO, ...); never retried.
	ClassPersistent
)

// Classify sorts an error into retry classes. The string is a short detail
// (the corrupt table name for ClassCorruptTable).
func Classify(err error) (Class, string) {
	if err == nil {
		return ClassNone, ""
	}
	if os.IsNotExist(err) {
		return ClassRetryableRace, ""
	}
	var fd storage.FileDesc
	var isCorrupted bool
	var lvErr *lverrors.ErrCorrupted
	var stErr *storage.ErrCorrupted
	if errors.As(err, &lvErr) {
		fd, isCorrupted = lvErr.Fd, true
	} else if errors.As(err, &stErr) {
		fd, isCorrupted = stErr.Fd, true
	}
	if isCorrupted {
		switch fd.Type {
		case storage.TypeTable:
			return ClassCorruptTable, fd.String()
		case storage.TypeJournal, storage.TypeManifest:
			return ClassRetryableRace, fd.String()
		default:
			// Corruption not attributable to a specific live-file class:
			// fail closed, do not retry.
			return ClassPersistent, ""
		}
	}
	return ClassPersistent, ""
}

// IsNotFound reports whether err is a key-absence error from the underlying
// key-value store (as opposed to an I/O or corruption error).
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, lverrors.ErrNotFound) {
		return true
	}
	// geth's memorydb returns a private errors.New("not found"); tolerate it
	// so strict readers behave identically under test databases.
	return err.Error() == "not found"
}
