package report

import (
	"encoding/json"
	"testing"
)

func TestResolveExitPrecedence(t *testing.T) {
	// Highest-precedence wins: 130 > 16 > 15 > 13 > 14 > 22 > 21 > 20 > 24 > 23 > 0.
	cases := []struct {
		in   []int
		want int
	}{
		{[]int{ExitOK}, ExitOK},
		{[]int{ExitMissingRequired, ExitInvalidRetained}, ExitInvalidRetained}, // 21 outranks 20
		{[]int{ExitAuditAnomaly, ExitDeterminismMismatch}, ExitAuditAnomaly},
		{[]int{ExitOK, ExitMissingRequired, ExitAuditAnomaly}, ExitMissingRequired},
		{[]int{ExitIO, ExitTargetStateUnavailable}, ExitIO},
		{[]int{ExitSIGINT, ExitInvalidRetained, ExitUnsafeOpen}, ExitSIGINT},
		{[]int{ExitBadInvocation, ExitUnsafeOpen}, ExitBadInvocation},
	}
	for _, c := range cases {
		if got := ResolveExit(c.in...); got != c.want {
			t.Errorf("ResolveExit(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestExitForFindings(t *testing.T) {
	// A mixed MissingRequired + InvalidRetained set exits 21.
	fs := []Finding{
		{Severity: SeverityFatal, Class: ClassMissingRequired},
		{Severity: SeverityFatal, Class: ClassInvalidRetained},
		{Severity: SeverityReviewItem, Class: ClassDiagnostic},
	}
	if got := ExitForFindings(fs); got != ExitInvalidRetained {
		t.Fatalf("mixed fatal set exits %d, want %d", got, ExitInvalidRetained)
	}
	// NoncanonicalKey maps to 21.
	if got := ExitForFindings([]Finding{{Severity: SeverityFatal, Class: ClassNoncanonicalKey}}); got != ExitInvalidRetained {
		t.Fatalf("noncanonical exits %d, want %d", got, ExitInvalidRetained)
	}
	// No fatals -> 0 (review items never gate).
	if got := ExitForFindings([]Finding{{Severity: SeverityReviewItem, Class: ClassDiagnostic}}); got != ExitOK {
		t.Fatalf("review-item-only exits %d, want 0", got)
	}
}

func TestCanonicalJSONStableAndSorted(t *testing.T) {
	a := map[string]interface{}{"b": 1, "a": 2, "nested": map[string]interface{}{"z": 1, "y": 2}}
	b := map[string]interface{}{"nested": map[string]interface{}{"y": 2, "z": 1}, "a": 2, "b": 1}
	ca, err := CanonicalJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := CanonicalJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("canonical JSON not order-independent:\n%s\n%s", ca, cb)
	}
	want := `{"a":2,"b":1,"nested":{"y":2,"z":1}}`
	if string(ca) != want {
		t.Fatalf("canonical JSON = %s, want %s", ca, want)
	}
}

func TestCanonicalJSONNumbersVerbatim(t *testing.T) {
	// Large uint64 must survive without float rounding.
	type doc struct {
		N uint64 `json:"n"`
	}
	raw, err := CanonicalJSON(doc{N: 92730034})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"n":92730034}` {
		t.Fatalf("got %s", raw)
	}
	var back doc
	if err := json.Unmarshal(raw, &back); err != nil || back.N != 92730034 {
		t.Fatalf("round-trip lost precision: %v %d", err, back.N)
	}
}

func TestSortFindingsDeterministic(t *testing.T) {
	fs := []Finding{
		{Code: "b", Key: "02"},
		{Code: "a", Key: "02"},
		{Code: "a", Key: "01"},
		{Code: "a", Key: "01", Detail: "x"},
	}
	SortFindings(fs)
	if fs[0].Code != "a" || fs[0].Key != "01" || fs[0].Detail != "" {
		t.Fatalf("first = %+v", fs[0])
	}
	if fs[3].Code != "b" {
		t.Fatalf("last = %+v", fs[3])
	}
}
