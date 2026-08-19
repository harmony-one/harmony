package rodb

import (
	"errors"
	"fmt"
	"io"
)

// verificationFailure is the marker interface implemented by check-failure
// errors (see report.Failure). A verification failure computed while the
// latch is clean is a genuine FAIL; one computed while reads were erroring
// is untrustworthy and follows the read-error path instead.
type verificationFailure interface {
	VerificationFailure() bool
}

// IsVerificationFailure reports whether err is a check FAIL rather than an
// I/O problem.
func IsVerificationFailure(err error) bool {
	var v verificationFailure
	return errors.As(err, &v) && v.VerificationFailure()
}

// Runner opens the database and runs pipeline stages with the bounded
// reopen-and-retry policy for genuine live-file races.
type Runner struct {
	Dir  string
	Opts Options
	// MaxAttempts bounds open+run attempts per stage (default 3).
	MaxAttempts int
	// Progress, if set, receives human-readable retry notes (stderr).
	Progress io.Writer

	// open hooks the database open (tests inject fault wrappers).
	open func() (*DB, error)

	db          *DB
	latch       *Latch
	reopenCount int
}

// NewRunner constructs a stage runner for the database directory.
func NewRunner(dir string, opts Options) *Runner {
	r := &Runner{Dir: dir, Opts: opts, MaxAttempts: 3}
	r.open = func() (*DB, error) { return Open(dir, opts) }
	r.latch = &Latch{}
	return r
}

// SetOpenFunc overrides the database open function (test fault injection).
func (r *Runner) SetOpenFunc(open func() (*DB, error)) { r.open = open }

// ReopenCount returns the total number of reopen attempts performed.
func (r *Runner) ReopenCount() int { return r.reopenCount }

// Latch returns the shared read-error latch.
func (r *Runner) Latch() *Latch { return r.latch }

// Close closes the underlying database if open.
func (r *Runner) Close() {
	if r.db != nil {
		_ = r.db.Close()
		r.db = nil
	}
}

func (r *Runner) progressf(format string, args ...interface{}) {
	if r.Progress != nil {
		fmt.Fprintf(r.Progress, format+"\n", args...)
	}
}

// Stage runs fn against the adapter, classifying failures:
//
//   - fn nil + clean latch: stage succeeded
//   - verification failure + clean latch: genuine FAIL, returned as-is
//   - retryable race (referenced-file ENOENT, journal/manifest turnover):
//     close, reopen against the fresh manifest generation, retry the stage;
//     bounded by MaxAttempts, exhaustion returns *ReadError
//   - immutable-SST corruption: *ReadError immediately, zero retries,
//     naming the corrupt table
//   - anything else: *ReadError immediately
func (r *Runner) Stage(name string, fn func(kv *KV) error) error {
	for attempt := 1; ; attempt++ {
		if r.db == nil {
			db, err := r.open()
			if err != nil {
				class, detail := Classify(err)
				if class == ClassRetryableRace && attempt < r.MaxAttempts {
					r.reopenCount++
					r.progressf("[%s] open hit a live-file race (%v); reopening (attempt %d/%d)", name, err, attempt+1, r.MaxAttempts)
					continue
				}
				return &ReadError{Err: fmt.Errorf("open database: %w", err), Detail: detail, Retries: attempt - 1}
			}
			r.db = db
		}
		r.latch.Reset()
		err := fn(r.db.KV(r.latch))

		latched := r.latch.First()
		if err == nil && latched == nil {
			return nil
		}
		if err != nil && IsVerificationFailure(err) && latched == nil {
			return err
		}
		// Read-error path. Prefer the latched root cause when present.
		cause := err
		if latched != nil {
			cause = latched
		}
		class, detail := Classify(cause)
		switch class {
		case ClassRetryableRace:
			if attempt < r.MaxAttempts {
				r.reopenCount++
				r.progressf("[%s] read hit a live-file race (%v); reopening and retrying stage (attempt %d/%d)", name, cause, attempt+1, r.MaxAttempts)
				r.Close()
				continue
			}
			return &ReadError{Err: cause, Detail: detail, Retries: attempt - 1}
		case ClassCorruptTable:
			// Immutable-SST corruption: no live-writer race explains it.
			return &ReadError{Err: cause, Detail: detail, Retries: 0}
		default:
			return &ReadError{Err: cause, Detail: detail, Retries: 0}
		}
	}
}
