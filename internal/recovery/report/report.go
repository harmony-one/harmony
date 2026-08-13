// Package report holds the metadata command family's shared output
// contract: the orthogonal finding severity/class model, the exit-code
// table with its deterministic precedence, canonical-JSON encoding (the
// byte-stable form every digested document uses), digest primitives, and
// atomic report writing.
//
// The preflight subcommand has its own, separate report package
// (internal/recovery/inplace/report) with exits 0/1/2/3; the two tables
// overlap only at 0 = success (plan §4.5).
package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Exit codes for the metadata command family (plan §4.5). Metadata
// subcommands never emit 1/2/3 themselves; cobra flag-parse and unknown
// command errors exit 2 at the shared root.
const (
	ExitOK                     = 0
	ExitUnsafeOpen             = 13 // unsafe DB open / concurrent writer / missing LOCK
	ExitIO                     = 14 // I/O or corruption
	ExitBadInvocation          = 15 // invalid config / path overlap (inside RunE)
	ExitInterrupted            = 16 // interrupted (non-signal)
	ExitMissingRequired        = 20 // MISSING_REQUIRED_METADATA (clean-DB fallback signal)
	ExitInvalidRetained        = 21 // INVALID_RETAINED_METADATA (incl. NoncanonicalKey)
	ExitTargetStateUnavailable = 22 // TARGET_STATE_UNAVAILABLE
	ExitDeterminismMismatch    = 23 // DETERMINISM_MISMATCH (export self-check)
	ExitAuditAnomaly           = 24 // AUDIT_ANOMALY (incl. POINTER_AMBIGUOUS)
	ExitSIGINT                 = 130
)

// precedence orders mixed outcomes: highest applicable wins (plan §4.5).
// Corruption (21) deliberately outranks the fallback signal (20): a corrupt
// DB must be investigated, not routed to the clean-DB path.
var precedence = []int{
	ExitSIGINT,
	ExitInterrupted,
	ExitBadInvocation,
	ExitUnsafeOpen,
	ExitIO,
	ExitTargetStateUnavailable,
	ExitInvalidRetained,
	ExitMissingRequired,
	ExitAuditAnomaly,
	ExitDeterminismMismatch,
	ExitOK,
}

// precedenceRank maps exit code -> position (lower = wins).
var precedenceRank = func() map[int]int {
	m := make(map[int]int, len(precedence))
	for i, c := range precedence {
		m[c] = i
	}
	return m
}()

// ErrSIGINT is the cancellation cause the CLI's signal handler records
// when SIGINT is delivered, so exits distinguish 130 (SIGINT) from 16
// (ordinary interruption).
var ErrSIGINT = errors.New("interrupted by SIGINT")

// InterruptExit maps a canceled context to the §4.5 interruption codes:
// 130 when the cancellation cause is SIGINT delivery, 16 otherwise.
func InterruptExit(ctx context.Context) int {
	if errors.Is(context.Cause(ctx), ErrSIGINT) {
		return ExitSIGINT
	}
	return ExitInterrupted
}

// ResolveExit returns the highest-precedence code among candidates.
// Unknown codes are rejected loudly (programming error).
func ResolveExit(codes ...int) int {
	best := ExitOK
	bestRank := precedenceRank[ExitOK]
	for _, c := range codes {
		r, ok := precedenceRank[c]
		if !ok {
			panic(fmt.Sprintf("report: unknown exit code %d", c))
		}
		if r < bestRank {
			best, bestRank = c, r
		}
	}
	return best
}

// Severity of a finding. Severity and Class are orthogonal (plan §4.3).
type Severity string

const (
	SeverityInfo       Severity = "info"
	SeverityReviewItem Severity = "review-item"
	SeverityFatal      Severity = "fatal"
)

// Class of a finding; every Fatal carries a Class mapping to exactly one
// exit code.
type Class string

const (
	ClassMissingRequired        Class = "missing-required"
	ClassInvalidRetained        Class = "invalid-retained"
	ClassTargetStateUnavailable Class = "target-state-unavailable"
	ClassNoncanonicalKey        Class = "noncanonical-key"
	ClassPollutionSuspect       Class = "pollution-suspect"
	ClassAuditAnomaly           Class = "audit-anomaly"
	ClassDeterminismMismatch    Class = "determinism-mismatch"
	ClassDiagnostic             Class = "diagnostic"
)

// ExitForClass maps a Fatal finding class to its exit code.
func ExitForClass(c Class) int {
	switch c {
	case ClassMissingRequired:
		return ExitMissingRequired
	case ClassInvalidRetained, ClassNoncanonicalKey:
		return ExitInvalidRetained
	case ClassTargetStateUnavailable:
		return ExitTargetStateUnavailable
	case ClassAuditAnomaly, ClassPollutionSuspect:
		return ExitAuditAnomaly
	case ClassDeterminismMismatch:
		return ExitDeterminismMismatch
	default:
		return ExitOK
	}
}

// Finding is one classified observation. Key is the hex of the raw LevelDB
// key it concerns (empty when not key-scoped).
type Finding struct {
	Severity Severity `json:"severity"`
	Class    Class    `json:"class"`
	Code     string   `json:"code"`
	Key      string   `json:"key,omitempty"`
	Detail   string   `json:"detail,omitempty"`
	// ChainDeterministic marks findings that are a pure function of the
	// retained chain-invariant content (and therefore belong in the
	// diagnostics digest); junk-, machine- or branch-dependent findings
	// are run evidence only (plan §4.5).
	ChainDeterministic bool `json:"chain_deterministic,omitempty"`
}

// SortFindings orders findings by (code, key, detail) — the canonical order
// used for the diagnostics digest and report sections.
func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Code != fs[j].Code {
			return fs[i].Code < fs[j].Code
		}
		if fs[i].Key != fs[j].Key {
			return fs[i].Key < fs[j].Key
		}
		return fs[i].Detail < fs[j].Detail
	})
}

// ExitForFindings resolves the exit code implied by a finding set: the
// highest-precedence exit among all Fatal findings' classes (0 if none).
func ExitForFindings(fs []Finding) int {
	codes := []int{ExitOK}
	for _, f := range fs {
		if f.Severity == SeverityFatal {
			codes = append(codes, ExitForClass(f.Class))
		}
	}
	return ResolveExit(codes...)
}

// CanonicalJSON encodes v as strictly canonical JSON: object keys sorted,
// no insignificant whitespace, numbers passed through verbatim
// (json.Number), and a guaranteed byte-stable output for equal inputs. It
// round-trips v through encoding/json, so struct field names follow their
// json tags.
func CanonicalJSON(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree interface{}
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, tree); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v interface{}) error {
	switch t := v.(type) {
	case map[string]interface{}:
		buf.WriteByte('{')
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := marshalScalar(buf, k); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case json.Number:
		buf.WriteString(t.String())
		return nil
	default:
		return marshalScalar(buf, t)
	}
}

// marshalScalar encodes a scalar (or map key) without HTML escaping, so
// predicate strings like "epoch>3002" stay literal. json.Encoder appends a
// trailing newline that we trim; the output stays deterministic.
func marshalScalar(buf *bytes.Buffer, v interface{}) error {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	b := tmp.Bytes()
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
	}
	buf.Write(b)
	return nil
}

// SHA256Hex returns the lowercase hex SHA-256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

// DigestCanonicalJSON is SHA256Hex(CanonicalJSON(v)).
func DigestCanonicalJSON(v interface{}) (string, error) {
	b, err := CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	return SHA256Hex(b), nil
}

// WriteFileAtomic writes data to path via temp file + rename in the target
// directory.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
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

// WriteJSONAtomic writes indented (human-readable) JSON atomically. Reports
// use this form; digested documents use CanonicalJSON.
func WriteJSONAtomic(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'))
}
