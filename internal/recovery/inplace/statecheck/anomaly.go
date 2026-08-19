package statecheck

import "github.com/harmony-one/harmony/internal/recovery/inplace/report"

// Anomaly kinds (informational; anomalies never gate).
const (
	// AnomalyFlagDecodedZero: an IsValidatorKey storage leaf exists but its
	// decoded value is the zero hash - the account is unflagged (matching
	// Object.IsValidator's decode-and-test), the stray leaf is noted.
	AnomalyFlagDecodedZero = "flag-decoded-zero"
	// AnomalyFlagNonCanonical: the IsValidatorKey leaf decodes non-zero but
	// differs from the canonical staking.IsValidator value - flagged, noted.
	AnomalyFlagNonCanonical = "flag-noncanonical-value"
	// AnomalyCodeMultiLocation: identical code bytes found at more than one
	// physical location (c/vc/legacy) - resolved by precedence c > vc >
	// legacy, noted.
	AnomalyCodeMultiLocation = "code-multiple-locations"
	// AnomalyWrapperShapedContract: unflagged account whose code bytes
	// RLP-decode as a validator wrapper - stays contract code, noted.
	AnomalyWrapperShapedContract = "wrapper-shaped-contract-code"
	// AnomalyCodeDualClass: the same code hash referenced as contract code
	// by one account and validator code by another - counted per class,
	// noted.
	AnomalyCodeDualClass = "code-dual-class"
)

// maxAnomalyExamples bounds the receipt's example list.
const maxAnomalyExamples = 20

// AnomalySet keeps full per-kind counters plus the first-seen bounded
// examples, deterministic for a given database.
type AnomalySet struct {
	total    int
	byKind   map[string]int
	examples []report.Anomaly
}

// NewAnomalySet builds an empty set.
func NewAnomalySet() *AnomalySet {
	return &AnomalySet{byKind: make(map[string]int)}
}

// Add records one anomaly.
func (s *AnomalySet) Add(kind, detail string) {
	s.total++
	s.byKind[kind]++
	if len(s.examples) < maxAnomalyExamples {
		s.examples = append(s.examples, report.Anomaly{Kind: kind, Detail: detail})
	}
}

// AddAll folds src into s preserving src's internal order (used for the
// ordered account fold, keeping examples first-seen deterministic under
// worker parallelism).
func (s *AnomalySet) AddAll(src *AnomalySet) {
	if src == nil {
		return
	}
	for _, ex := range src.examples {
		s.Add(ex.Kind, ex.Detail)
	}
	// Examples beyond src's own bound still count.
	for kind, n := range src.byKind {
		seen := 0
		for _, ex := range src.examples {
			if ex.Kind == kind {
				seen++
			}
		}
		for i := seen; i < n; i++ {
			s.total++
			s.byKind[kind]++
		}
	}
}

// Report converts to the bounded receipt form.
func (s *AnomalySet) Report() report.Anomalies {
	out := report.Anomalies{
		Total:   s.total,
		Omitted: s.total - len(s.examples),
	}
	if len(s.byKind) > 0 {
		out.ByKind = make(map[string]int, len(s.byKind))
		for k, v := range s.byKind {
			out.ByKind[k] = v
		}
	}
	out.Example = append(out.Example, s.examples...)
	return out
}
