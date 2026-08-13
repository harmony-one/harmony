// Package report holds the validator-facing output contract: the PASS/FAIL
// console line, the small flat JSON receipt, and the exit codes.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Exit codes (fixed contract).
const (
	ExitPass      = 0 // all checks passed (point-in-time sample if the node was running)
	ExitFail      = 1 // a verification check failed (one-line reason on the console)
	ExitUnusable  = 2 // bad flags, missing DB, unsupported layout, not a LevelDB
	ExitReadError = 3 // persistent read errors after retries
)

// RemedyLine is printed to stderr on exit 3.
const RemedyLine = "remedy: re-run; if it keeps failing, stop the node briefly and re-run"

// SampleNote is the point-in-time live-sample disclaimer embedded in every
// receipt.
const SampleNote = "result reflects the database at scan time; if the node was running, this is a point-in-time sample - authoritative verification happens at apply time on a stopped node"

// Schema is the receipt schema identifier.
const Schema = "preflight-result-v2"

// Tool is the receipt tool identifier.
const Tool = "harmony-recovery preflight"

// Failure is a verification-check FAIL (as opposed to an I/O read error).
// The first failing check wins the console line; the receipt carries the
// rest.
type Failure struct {
	Check  string // stable check id, e.g. "target_header"
	Reason string // one line
}

// Failf builds a Failure.
func Failf(check, format string, args ...interface{}) *Failure {
	return &Failure{Check: check, Reason: fmt.Sprintf(format, args...)}
}

func (f *Failure) Error() string { return f.Check + ": " + f.Reason }

// VerificationFailure marks this error as a check FAIL for the retry runner
// (rodb.IsVerificationFailure).
func (f *Failure) VerificationFailure() bool { return true }

// Build carries informational build-stamp fields (no gating or refusal
// paths).
type Build struct {
	GitDescribe string `json:"git_describe,omitempty"`
	VCSRevision string `json:"vcs_revision,omitempty"`
	VCSModified bool   `json:"vcs_modified,omitempty"`
	GoVersion   string `json:"go_version,omitempty"`
}

// Target is the verified target tuple.
type Target struct {
	Height    uint64 `json:"height"`
	Hash      string `json:"hash"`
	StateRoot string `json:"state_root,omitempty"`
	Epoch     uint64 `json:"epoch,omitempty"`
	ViewID    uint64 `json:"view_id,omitempty"`
}

// HeadSample is the informational upward sample (never gates).
type HeadSample struct {
	LastHeader        string `json:"last_header,omitempty"`
	LastBlock         string `json:"last_block,omitempty"`
	WalkToTarget      string `json:"walk_to_target,omitempty"`
	ChildAtTargetPlus string `json:"child_at_target_plus_1,omitempty"`
}

// CertificateSources records which certificate sources were present and
// which satisfied the check.
type CertificateSources struct {
	ExactKeyPresent    bool   `json:"exact_key_present"`
	ChildHeaderPresent bool   `json:"child_header_present"`
	SatisfiedBy        string `json:"satisfied_by,omitempty"`
}

// Anomaly is one bounded example entry.
type Anomaly struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Anomalies is the bounded anomaly report: full counters, first-seen
// examples, and the omitted count. Anomalies never gate.
type Anomalies struct {
	Total   int            `json:"total"`
	ByKind  map[string]int `json:"by_kind,omitempty"`
	Example []Anomaly      `json:"examples,omitempty"`
	Omitted int            `json:"omitted"`
}

// StateCounts are the state-walk counters.
type StateCounts struct {
	Accounts            uint64 `json:"accounts"`
	AccountTrieNodes    uint64 `json:"account_trie_nodes"`
	StorageTries        uint64 `json:"storage_tries"`
	StorageTrieNodes    uint64 `json:"storage_trie_nodes"`
	StorageLeaves       uint64 `json:"storage_leaves"`
	CodeRefsContract    uint64 `json:"code_refs_contract"`
	CodeRefsValidator   uint64 `json:"code_refs_validator"`
	UniqueCodeContract  uint64 `json:"unique_code_contract"`
	UniqueCodeValidator uint64 `json:"unique_code_validator"`
	UniqueCodeBytes     uint64 `json:"unique_code_bytes"`
}

// State is the state-walk section of the receipt.
type State struct {
	Digest          string      `json:"digest,omitempty"`
	DigestAlgorithm string      `json:"digest_algorithm,omitempty"`
	Counts          StateCounts `json:"counts"`
	Anomalies       Anomalies   `json:"anomalies"`
}

// Retries reports live-race reopen activity.
type Retries struct {
	ReopenCount int `json:"reopen_count"`
}

// Receipt is the one small flat JSON file a validator attaches in Telegram.
type Receipt struct {
	Tool   string `json:"tool"`
	Schema string `json:"schema"`
	Build  Build  `json:"build"`

	Name     string `json:"name,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Network  string `json:"network"`
	Shard    uint32 `json:"shard"`
	DBPath   string `json:"db_path"`

	NodeProbablyRunning *bool  `json:"node_probably_running,omitempty"`
	SampleNote          string `json:"sample_note"`

	StartedAt string  `json:"started_at"`
	DurationS float64 `json:"duration_s"`
	Retries   Retries `json:"retries"`

	Target Target `json:"target"`

	// Checks maps stable check ids to "ok" | "fail: <reason>" | "skipped".
	Checks map[string]string `json:"checks"`

	HeadSample         HeadSample         `json:"head_sample"`
	CertificateSources CertificateSources `json:"certificate_sources"`

	State State `json:"state"`

	Result     string `json:"result"` // "PASS" | "FAIL" (exit_code 3 marks a read-error FAIL)
	FailReason string `json:"fail_reason,omitempty"`
	ExitCode   int    `json:"exit_code"`
}

// CheckIDs is the stable, ordered check id list.
var CheckIDs = []string{
	"target_header",
	"body",
	"ancestry_to_boundary",
	"shard_state",
	"certificate",
	"state_walk",
}

// NewChecks returns a check map with every check "skipped".
func NewChecks() map[string]string {
	m := make(map[string]string, len(CheckIDs))
	for _, id := range CheckIDs {
		m[id] = "skipped"
	}
	return m
}

// ValidateReportPath refuses a report path that resolves inside the DB
// directory (writing into a live LevelDB directory could confuse the node).
// Both the raw and the symlink-resolved forms are compared: the report's
// parent may not exist yet, and partially-resolvable paths must not slip
// through (e.g. /var vs /private/var on macOS).
func ValidateReportPath(reportPath, dbPath string) error {
	absReport, err := filepath.Abs(reportPath)
	if err != nil {
		return fmt.Errorf("resolve report path: %w", err)
	}
	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}
	within := func(base, p string) bool {
		rel, err := filepath.Rel(base, p)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	resolvedDB := absDB
	if r, err := filepath.EvalSymlinks(absDB); err == nil {
		resolvedDB = r
	}
	resolvedReport := absReport
	// Resolve the deepest existing ancestor of the report path and rejoin
	// the non-existing remainder.
	dir, rest := filepath.Dir(absReport), filepath.Base(absReport)
	for {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			resolvedReport = filepath.Join(r, rest)
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
	if within(absDB, absReport) || within(resolvedDB, resolvedReport) {
		return fmt.Errorf("report path %s resolves inside the DB directory %s; choose a path outside the database", reportPath, dbPath)
	}
	return nil
}

// Write atomically writes the receipt: temp file + rename in the target
// directory.
func (r *Receipt) Write(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".preflight-result-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// FinalLine prints the exactly-one-line stdout contract for completed
// verification runs: "PASS" or "FAIL: <one-line reason>".
func FinalLine(stdout io.Writer, pass bool, failReason string) {
	if pass {
		fmt.Fprintln(stdout, "PASS")
		return
	}
	fmt.Fprintf(stdout, "FAIL: %s\n", failReason)
}
