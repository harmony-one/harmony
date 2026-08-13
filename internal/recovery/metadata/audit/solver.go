package audit

import (
	"fmt"
	"sort"
)

// SolvePointer implements the §4.4 invariant solver for the target-time
// last-crosslink pointer. The pointer advances on every beacon block from
// the stored value through consecutive numbers until the first gap, so:
//
//	(i)  closure_pre(p*)  = p*  — saturation: the target-time pointer was
//	     maximal over pre-target records;
//	(ii) closure_full(p*) = Q   — replaying branch writes from p*
//	     reproduces the stored pointer.
//
// "Highest retained key" is NOT a valid reconstruction, and replay
// observation cannot prove the branch left the pointer alone (a replay
// seeded with the possibly-branch-advanced stored value writes nothing,
// vacuously).
//
// sPre is the set of pre-target crosslink block numbers (current records
// minus the audit's branch-written subset); branch is the branch-written
// subset; q is the stored pointer's block number. Returns the unique
// solution, or ok=false with every candidate for the POINTER_AMBIGUOUS
// report.
func SolvePointer(sPre, branch map[uint64]bool, q uint64) (p uint64, ok bool, candidates []uint64) {
	full := make(map[uint64]bool, len(sPre)+len(branch))
	for n := range sPre {
		full[n] = true
	}
	for n := range branch {
		full[n] = true
	}
	closure := func(set map[uint64]bool, from uint64) uint64 {
		for set[from+1] {
			from++
		}
		return from
	}
	var sols []uint64
	for n := range sPre {
		if sPre[n+1] {
			continue // not saturated over pre
		}
		if closure(full, n) == q {
			sols = append(sols, n)
		}
	}
	sort.Slice(sols, func(i, j int) bool { return sols[i] < sols[j] })
	if len(sols) == 1 {
		return sols[0], true, sols
	}
	return 0, false, sols
}

// ValidateTrustedPointer checks a --trusted-shard1-pointer value against
// both invariants (a trusted value violating either fails the audit).
func ValidateTrustedPointer(trusted uint64, sPre, branch map[uint64]bool, q uint64) error {
	if !sPre[trusted] {
		return fmt.Errorf("trusted pointer %d is not a retained pre-target crosslink record", trusted)
	}
	if sPre[trusted+1] {
		return fmt.Errorf("trusted pointer %d violates saturation: pre-target record %d exists", trusted, trusted+1)
	}
	full := make(map[uint64]bool, len(sPre)+len(branch))
	for n := range sPre {
		full[n] = true
	}
	for n := range branch {
		full[n] = true
	}
	c := trusted
	for full[c+1] {
		c++
	}
	if c != q {
		return fmt.Errorf("trusted pointer %d violates branch replay: closure over full set reaches %d, stored pointer is %d", trusted, c, q)
	}
	return nil
}
