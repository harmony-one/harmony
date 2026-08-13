package audit

import "testing"

func set(nums ...uint64) map[uint64]bool {
	m := map[uint64]bool{}
	for _, n := range nums {
		m[n] = true
	}
	return m
}

// TestSolverBranchAdvance is the regression for the circular "replay showed
// no writes so Q stands" rule: pre-target pointer K, pre-target set gapped
// at K+1, branch fills K+1..Q; the solver must recover exactly K.
func TestSolverBranchAdvance(t *testing.T) {
	// Pre-target records: ...8,9,10 (K=10), gap at 11.
	sPre := set(8, 9, 10)
	// Branch fills 11,12,13 (Q=13).
	branch := set(11, 12, 13)
	q := uint64(13)
	p, ok, cands := SolvePointer(sPre, branch, q)
	if !ok || p != 10 {
		t.Fatalf("branch-advance: got p=%d ok=%v candidates=%v, want 10", p, ok, cands)
	}
}

// TestSolverAmbiguous: two gap-maximal candidates whose branch-filled
// closures both reach Q -> POINTER_AMBIGUOUS.
func TestSolverAmbiguous(t *testing.T) {
	// Pre: 3 (gap at 4) and 7 (gap at 8). Branch fills 4,5,6,7? No — 7 is
	// pre. Make two disjoint saturated pre candidates both closing to Q
	// via branch fills.
	sPre := set(3, 6) // 3 saturated (no 4), 6 saturated (no 7)
	branch := set(4, 5, 7, 8, 9, 10)
	q := uint64(10)
	// From 3: full has 3,4,5,6,7,8,9,10 -> closure 10 == Q.
	// From 6: full has 6,7,8,9,10 -> closure 10 == Q.
	_, ok, cands := SolvePointer(sPre, branch, q)
	if ok {
		t.Fatalf("expected ambiguity, got unique %v", cands)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %v", cands)
	}
	// The trusted escape hatch picks one and validates.
	if err := ValidateTrustedPointer(3, sPre, branch, q); err != nil {
		t.Fatalf("trusted pointer 3 should validate: %v", err)
	}
	// A trusted value violating saturation fails.
	if err := ValidateTrustedPointer(6, set(3, 6, 7), branch, q); err == nil {
		t.Fatal("trusted pointer violating saturation must fail")
	}
}

// TestSolverUniqueSolution: contiguous run T..K, K+1 absent everywhere, K+2
// retained -> solver finds K with no trusted input.
func TestSolverUniqueSolution(t *testing.T) {
	// Pre-target contiguous 1..5 (K=5), 6 absent everywhere, 7 retained.
	sPre := set(1, 2, 3, 4, 5, 7)
	branch := map[uint64]bool{}
	q := uint64(5)
	// Candidate 5: saturated (no 6), closure over full (no 6) stays 5 == Q. Good.
	// Candidate 7: saturated (no 8), closure 7 != Q. Rejected.
	p, ok, cands := SolvePointer(sPre, branch, q)
	if !ok || p != 5 {
		t.Fatalf("unique: got p=%d ok=%v cands=%v, want 5", p, ok, cands)
	}
}

func TestSolverTrustedValidatesBranchReplay(t *testing.T) {
	sPre := set(10)
	branch := set(11, 12)
	q := uint64(12)
	if err := ValidateTrustedPointer(10, sPre, branch, q); err != nil {
		t.Fatalf("valid trusted pointer rejected: %v", err)
	}
	// Trusted value whose branch replay does not reach Q fails.
	if err := ValidateTrustedPointer(10, sPre, set(11), q); err == nil {
		t.Fatal("trusted pointer failing branch replay must be rejected")
	}
}
