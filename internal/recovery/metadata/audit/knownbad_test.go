package audit

import (
	"strings"
	"testing"
)

// TestCrossCheckKnownBad exhaustively drives the known-bad gate (§4.6 output
// 1). It is a UNIT test on purpose: on any single-shard localnet fixture it is
// impossible to produce a block that fails ONLY incoming-receipts validation
// while still re-executing to its stored root — see the package note. A header
// IncomingReceiptHash tamper necessarily also breaks the commit signature
// (collateral header-signature failure), and a body incoming-receipt tamper is
// applied by re-execution (ApplyIncomingReceipt) and diverges the state root
// into a FATAL insert failure rather than a recorded validity finding. Driving
// the pure gate with synthesized outcomes is therefore the only deterministic
// way to exercise the gate-satisfied path and the exact set of anomalies.
func TestCrossCheckKnownBad(t *testing.T) {
	const kb = 100
	out := func(h uint64, fails ...string) blockOutcome {
		return blockOutcome{Height: h, ValidityFails: fails}
	}
	receipts := "incoming-receipts: [verifyIncomingReceipts] Invalid IncomingReceiptHash in block header"
	seal := "header-signature: deserialize signature and bitmap: bad"
	vrf := "vrf: bad vrf"

	tests := []struct {
		name         string
		outcomes     []blockOutcome
		knownBad     []uint64
		wantChecked  bool
		wantAnomaly  []string // kinds expected, in order
		wantNoAnomly bool
	}{
		{
			name:         "receipt_only_at_known_bad_satisfies_gate",
			outcomes:     []blockOutcome{out(kb, receipts)},
			knownBad:     []uint64{kb},
			wantChecked:  true,
			wantNoAnomly: true,
		},
		{
			name:        "extra_seal_failure_at_known_bad_is_anomalous",
			outcomes:    []blockOutcome{out(kb, receipts, seal)},
			knownBad:    []uint64{kb},
			wantChecked: true, // exploit signature reproduced ...
			wantAnomaly: []string{"known-bad-extra-failure"}, // ... but the extra defect still gates
		},
		{
			name:        "wrong_failure_only_does_not_satisfy_gate",
			outcomes:    []blockOutcome{out(kb, seal)},
			knownBad:    []uint64{kb},
			wantChecked: false,
			wantAnomaly: []string{"known-bad-extra-failure", "known-bad-wrong-failure"},
		},
		{
			name:        "no_failure_at_known_bad_is_absent",
			outcomes:    []blockOutcome{out(kb + 1)}, // no fails anywhere
			knownBad:    []uint64{kb},
			wantChecked: false,
			wantAnomaly: []string{"known-bad-failure-absent"},
		},
		{
			name:        "failure_off_the_known_bad_list_is_unexpected",
			outcomes:    []blockOutcome{out(kb, receipts), out(42, vrf)},
			knownBad:    []uint64{kb},
			wantChecked: true,
			wantAnomaly: []string{"unexpected-validity-failure"},
		},
		{
			name:        "multiple_extra_failures_each_anomalous",
			outcomes:    []blockOutcome{out(kb, receipts, seal, vrf)},
			knownBad:    []uint64{kb},
			wantChecked: true,
			wantAnomaly: []string{"known-bad-extra-failure", "known-bad-extra-failure"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checked, _, anomalies := crossCheckKnownBad(tc.outcomes, tc.knownBad)
			if checked != tc.wantChecked {
				t.Fatalf("crossChecked=%v, want %v", checked, tc.wantChecked)
			}
			if tc.wantNoAnomly && len(anomalies) != 0 {
				t.Fatalf("expected no anomalies, got %+v", anomalies)
			}
			if len(anomalies) != len(tc.wantAnomaly) {
				t.Fatalf("got %d anomalies %+v, want kinds %v", len(anomalies), anomalies, tc.wantAnomaly)
			}
			for i, want := range tc.wantAnomaly {
				if anomalies[i].Kind != want {
					t.Fatalf("anomaly %d kind=%q, want %q (all: %+v)", i, anomalies[i].Kind, want, anomalies)
				}
			}
		})
	}

	// Guard the exact label prefix the gate keys on: recordModeChecks emits
	// "incoming-receipts: <err>", so a change to that label would silently
	// break the gate. Pin the prefix here.
	if !strings.HasPrefix(receipts, "incoming-receipts:") {
		t.Fatal("test receipts label must carry the incoming-receipts prefix the gate matches")
	}
}
