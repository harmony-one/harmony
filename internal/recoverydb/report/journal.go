package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Journal states (plan §4 "Journal state machine").
const (
	StateInProgress           = "IN_PROGRESS"
	StateCompleteVerified     = "COMPLETE_VERIFIED"
	StateCompleteUnreleasable = "COMPLETE_UNRELEASABLE"
)

// Package-db promote-and-seal substates, recorded within IN_PROGRESS (the one
// defined exception to no-resume, plan §4 / WS7).
const (
	SubstatePromoting = "PROMOTING"
	SubstatePromoted  = "PROMOTED"
	SubstateSealed    = "SEALED"
)

// JournalRecord is one fsynced line of the sidecar journal.
type JournalRecord struct {
	Seq       int    `json:"seq"`
	State     string `json:"state"`
	Substate  string `json:"substate,omitempty"`
	ReleaseID string `json:"release_id,omitempty"`
	Note      string `json:"note,omitempty"`
	At        string `json:"at"` // RFC3339; informational only
}

// Journal is an fsynced (O_SYNC) append-only sidecar journal next to a
// mutating command's destination. Crash before the completion record leaves
// IN_PROGRESS, which every reopen refuses (v1 no-resume).
type Journal struct {
	path    string
	f       *os.File
	records []JournalRecord
}

// JournalPath returns the sidecar journal path for a destination directory.
func JournalPath(destination string) string {
	return filepath.Clean(destination) + ".journal"
}

// CreateJournal starts a fresh journal, refusing if one already exists (the
// caller decides what an existing journal means; for every command but
// package-db reconciliation it means refuse-discard-rebuild). The first
// IN_PROGRESS record is written and fsynced before returning.
func CreateJournal(path string) (*Journal, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("report: journal %s already exists; refusing (v1 never resumes an unclean destination)", path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("report: stat journal %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("report: create journal %s: %w", path, err)
	}
	if err := fsyncDir(filepath.Dir(path)); err != nil {
		f.Close()
		return nil, err
	}
	j := &Journal{path: path, f: f}
	if err := j.append(JournalRecord{State: StateInProgress}); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

// LoadJournal reads an existing journal for inspection or reconciliation.
// The returned journal can append further records.
func LoadJournal(path string) (*Journal, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report: read journal %s: %w", path, err)
	}
	var records []JournalRecord
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	ln := 0
	for sc.Scan() {
		ln++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec JournalRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("report: malformed journal record %s:%d: %w", path, ln, err)
		}
		records = append(records, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("report: scan journal %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("report: journal %s is empty", path)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("report: reopen journal %s: %w", path, err)
	}
	return &Journal{path: path, f: f, records: records}, nil
}

// JournalState returns the terminal state of a destination's journal, or
// StateInProgress if no completion record was written. Missing journal is an
// error (the destination was not produced by these tools).
func JournalState(path string) (string, *JournalRecord, error) {
	j, err := LoadJournal(path)
	if err != nil {
		return "", nil, err
	}
	defer j.Close()
	last := j.Last()
	return last.State, last, nil
}

func (j *Journal) append(rec JournalRecord) error {
	rec.Seq = len(j.records) + 1
	rec.At = time.Now().UTC().Format(time.RFC3339)
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("report: marshal journal record: %w", err)
	}
	line = append(line, '\n')
	if _, err := j.f.Write(line); err != nil {
		return fmt.Errorf("report: append journal %s: %w", j.path, err)
	}
	// O_SYNC makes the write durable; an explicit Sync is a cheap belt.
	if err := j.f.Sync(); err != nil {
		return fmt.Errorf("report: fsync journal %s: %w", j.path, err)
	}
	j.records = append(j.records, rec)
	return nil
}

// Substate records a package-db promote/seal substate within IN_PROGRESS.
func (j *Journal) Substate(substate, releaseID string) error {
	return j.append(JournalRecord{State: StateInProgress, Substate: substate, ReleaseID: releaseID})
}

// Complete writes the terminal record (COMPLETE_VERIFIED or
// COMPLETE_UNRELEASABLE) with an optional note.
func (j *Journal) Complete(state, note string) error {
	if state != StateCompleteVerified && state != StateCompleteUnreleasable {
		return fmt.Errorf("report: invalid terminal journal state %q", state)
	}
	return j.append(JournalRecord{State: state, Note: note})
}

// Last returns the most recent record.
func (j *Journal) Last() *JournalRecord {
	if len(j.records) == 0 {
		return nil
	}
	return &j.records[len(j.records)-1]
}

// Records returns a copy of all records.
func (j *Journal) Records() []JournalRecord {
	return append([]JournalRecord(nil), j.records...)
}

// Close closes the journal file handle.
func (j *Journal) Close() error { return j.f.Close() }

// Path returns the journal file path.
func (j *Journal) Path() string { return j.path }
