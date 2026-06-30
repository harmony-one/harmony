package consensus

import (
	"testing"
	"time"
)

// allowSlack absorbs sub-second drift between the test's "now" and the
// computed time. The proposer logic operates on whole-second Unix timestamps,
// so off-by-one-second comparisons are expected at boundaries; we tolerate up
// to 1.5 s of slack on sleep durations.
const proposerTestSleepSlack = 1500 * time.Millisecond

// fixedNow returns a time.Time at the given Unix second so tests are
// deterministic and not subject to whatever the runtime clock happens to be.
func fixedNow(unixSec int64) time.Time {
	return time.Unix(unixSec, 0)
}

func TestComputeProposalTiming_ReadyCases(t *testing.T) {
	skewClamp := int64(viewChangeSlot)
	tests := []struct {
		name          string
		parentTime    int64
		wallOffset    int64 // wall = parent + wallOffset
		wantTimestamp int64 // expected proposeAt.Unix()
	}{
		{"one_second_after_parent", 1000, 1, 1001},
		{"two_second_after_parent_normal", 1000, 2, 1002},
		{"five_second_after_parent", 1000, 5, 1005},
		{"at_step_boundary_no_clamp", 1000, maxProposerForwardStep, 1000 + maxProposerForwardStep},
		// > step but <= threshold → clamp to parent+step
		{"just_past_step_clamps", 1000, maxProposerForwardStep + 1, 1000 + maxProposerForwardStep},
		{"plus_15s_skew_clamps", 1000, 17, 1000 + maxProposerForwardStep},
		{"at_stall_threshold_clamps", 1000, skewClamp, 1000 + maxProposerForwardStep},
		// > threshold → stall recovery, NO clamp, propose at wall
		{"just_past_threshold_no_clamp", 1000, skewClamp + 1, 1000 + skewClamp + 1},
		{"short_stall_propose_at_wall", 1000, 60, 1060},
		{"five_minute_stall_propose_at_wall", 1000, 300, 1300},
		{"one_hour_stall_propose_at_wall", 1000, 3600, 4600},
		{"one_day_stall_propose_at_wall", 1000, 86400, 87400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := fixedNow(tc.parentTime + tc.wallOffset)
			d := computeProposalTiming(now, tc.parentTime, skewClamp, true)
			if !d.ready {
				t.Fatalf("expected ready, got %+v", d)
			}
			if d.giveUp {
				t.Fatalf("expected !giveUp, got %+v", d)
			}
			if d.sleep != 0 {
				t.Fatalf("expected zero sleep, got %v", d.sleep)
			}
			if got := d.proposeAt.Unix(); got != tc.wantTimestamp {
				t.Fatalf("proposeAt = %d, want %d", got, tc.wantTimestamp)
			}
		})
	}
}

func TestComputeProposalTiming_SleepCases(t *testing.T) {
	skewClamp := int64(viewChangeSlot)
	tests := []struct {
		name           string
		parentTime     int64
		wallOffset     int64 // negative or zero
		wantSleepLower time.Duration
		wantSleepUpper time.Duration
	}{
		{
			name:           "wall_equals_parent_sleep_1s",
			parentTime:     1000,
			wallOffset:     0,
			wantSleepLower: 1 * time.Second,
			wantSleepUpper: 1 * time.Second,
		},
		{
			name:           "wall_one_before_parent_sleep_2s",
			parentTime:     1000,
			wallOffset:     -1,
			wantSleepLower: 2 * time.Second,
			wantSleepUpper: 2 * time.Second,
		},
		{
			name:           "wall_five_before_parent_sleep_6s",
			parentTime:     1000,
			wallOffset:     -5,
			wantSleepLower: 6 * time.Second,
			wantSleepUpper: 6 * time.Second,
		},
		{
			name:           "wall_ten_before_parent_sleep_11s",
			parentTime:     1000,
			wallOffset:     -10,
			wantSleepLower: 11 * time.Second,
			wantSleepUpper: 11 * time.Second,
		},
		{
			name:           "at_catchup_cap_boundary",
			parentTime:     1000,
			wallOffset:     -(int64(maxProposerCatchupWait.Seconds()) - 1),
			wantSleepLower: maxProposerCatchupWait,
			wantSleepUpper: maxProposerCatchupWait,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := fixedNow(tc.parentTime + tc.wallOffset)
			d := computeProposalTiming(now, tc.parentTime, skewClamp, true)
			if d.ready {
				t.Fatalf("expected !ready, got %+v", d)
			}
			if d.giveUp {
				t.Fatalf("expected !giveUp, got %+v", d)
			}
			if d.sleep < tc.wantSleepLower-proposerTestSleepSlack ||
				d.sleep > tc.wantSleepUpper+proposerTestSleepSlack {
				t.Fatalf("sleep = %v, want in [%v, %v]",
					d.sleep, tc.wantSleepLower, tc.wantSleepUpper)
			}
		})
	}
}

func TestComputeProposalTiming_GiveUpCases(t *testing.T) {
	skewClamp := int64(viewChangeSlot)
	tests := []struct {
		name       string
		parentTime int64
		wallOffset int64 // must yield wait > maxProposerCatchupWait
	}{
		{
			name:       "wait_one_past_cap_gives_up",
			parentTime: 1000,
			wallOffset: -(int64(maxProposerCatchupWait.Seconds()) + 1),
		},
		{
			name:       "wait_thirty_seconds_gives_up",
			parentTime: 1000,
			wallOffset: -30,
		},
		{
			name:       "wait_one_minute_gives_up",
			parentTime: 1000,
			wallOffset: -60,
		},
		{
			name:       "wait_one_hour_gives_up",
			parentTime: 1000,
			wallOffset: -3600,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := fixedNow(tc.parentTime + tc.wallOffset)
			d := computeProposalTiming(now, tc.parentTime, skewClamp, true)
			if !d.giveUp {
				t.Fatalf("expected giveUp, got %+v", d)
			}
			if d.ready {
				t.Fatalf("expected !ready, got %+v", d)
			}
			if d.sleep != 0 {
				t.Fatalf("expected zero sleep on give-up, got %v", d.sleep)
			}
		})
	}
}

// TestComputeProposalTiming_ClampVsStallBoundary pins down the exact
// transition between "clamp to parent+step" and "propose at wall".
func TestComputeProposalTiming_ClampVsStallBoundary(t *testing.T) {
	parentTime := int64(1000)
	step := maxProposerForwardStep
	threshold := int64(viewChangeSlot)

	// At threshold (inclusive): clamp.
	at := computeProposalTiming(fixedNow(parentTime+threshold), parentTime, threshold, true)
	if !at.ready || at.proposeAt.Unix() != parentTime+step {
		t.Fatalf("at threshold: expected clamp to parent+step (%d), got %+v",
			parentTime+step, at)
	}

	// One past threshold: no clamp, propose at wall.
	past := computeProposalTiming(fixedNow(parentTime+threshold+1), parentTime, threshold, true)
	if !past.ready || past.proposeAt.Unix() != parentTime+threshold+1 {
		t.Fatalf("past threshold: expected no clamp (propose at %d), got %+v",
			parentTime+threshold+1, past)
	}
}

// TestComputeProposalTiming_AdversarialSkewedLeaderClamps verifies a +15s
// clock-skewed leader's proposed timestamp gets clamped to parent+step so
// honest validators (with synced clocks) will accept the block.
func TestComputeProposalTiming_AdversarialSkewedLeaderClamps(t *testing.T) {
	parentTime := int64(1000)
	skewedNow := fixedNow(parentTime + 15)

	d := computeProposalTiming(skewedNow, parentTime, int64(viewChangeSlot), true)
	if !d.ready {
		t.Fatalf("expected ready, got %+v", d)
	}
	if d.proposeAt.Unix() != parentTime+maxProposerForwardStep {
		t.Fatalf("expected clamp to parent+step (%d), got %d",
			parentTime+maxProposerForwardStep, d.proposeAt.Unix())
	}
}

// TestComputeProposalTiming_FastStallRecovery verifies that a long stall
// produces a block at wall clock (single-block recovery), not a clamped
// block that would take many rounds to catch up.
func TestComputeProposalTiming_FastStallRecovery(t *testing.T) {
	parentTime := int64(1000)
	wall := parentTime + 300
	d := computeProposalTiming(fixedNow(wall), parentTime, int64(viewChangeSlot), true)
	if !d.ready {
		t.Fatalf("expected ready, got %+v", d)
	}
	if d.proposeAt.Unix() != wall {
		t.Fatalf("expected propose at wall (%d) for stall recovery, got %d",
			wall, d.proposeAt.Unix())
	}
}

// TestComputeProposalTiming_SkewClampBand verifies +42s skew clamps with
// viewChangeSlot threshold instead of using stall recovery.
func TestComputeProposalTiming_SkewClampBand(t *testing.T) {
	parentTime := int64(1000)
	threshold := int64(viewChangeSlot)

	d := computeProposalTiming(fixedNow(parentTime+42), parentTime, threshold, true)
	if !d.ready || d.proposeAt.Unix() != parentTime+maxProposerForwardStep {
		t.Fatalf("+42s: expected clamp to %d, got %+v",
			parentTime+maxProposerForwardStep, d)
	}

	d = computeProposalTiming(fixedNow(parentTime+46), parentTime, threshold, true)
	if !d.ready || d.proposeAt.Unix() != parentTime+46 {
		t.Fatalf("+46s: expected wall, got %+v", d)
	}
}

// TestComputeProposalTiming_LegacyBeforeFork matches pre-fork leader behavior.
func TestComputeProposalTiming_LegacyBeforeFork(t *testing.T) {
	parentTime := int64(1000)

	// No clamp even with large positive skew.
	d := computeProposalTiming(fixedNow(parentTime+42), parentTime, 0, false)
	if !d.ready || d.proposeAt.Unix() != parentTime+42 {
		t.Fatalf("legacy +42s: expected wall, got %+v", d)
	}

	// Sleep without give-up cap when far behind parent.
	d = computeProposalTiming(fixedNow(parentTime-60), parentTime, 0, false)
	if d.ready || d.giveUp {
		t.Fatalf("legacy -60s: expected sleep, got %+v", d)
	}
	if d.sleep != 61*time.Second {
		t.Fatalf("legacy sleep = %v, want 61s", d.sleep)
	}
}

// TestComputeProposalTiming_CascadingAftermath verifies that when parent is
// slightly in the future of wall (after a cascading-skew round), the proposer
// sleeps the right amount and the wait is within the catch-up cap.
func TestComputeProposalTiming_CascadingAftermath(t *testing.T) {
	parentTime := int64(1000)
	wall := parentTime - 8
	d := computeProposalTiming(fixedNow(wall), parentTime, int64(viewChangeSlot), true)
	if d.ready || d.giveUp {
		t.Fatalf("expected sleep path, got %+v", d)
	}
	expected := time.Duration(parentTime+1-wall) * time.Second
	if d.sleep != expected {
		t.Fatalf("expected sleep %v, got %v", expected, d.sleep)
	}
	if d.sleep > maxProposerCatchupWait {
		t.Fatalf("sleep %v exceeds catchup cap %v", d.sleep, maxProposerCatchupWait)
	}
}

// TestComputeProposalTiming_HonestRecoveryAfterClampedLeader verifies the
// honest path after a clamped leader produced parent+step: the next leader's
// wall should already be at or past parent, so no sleep needed.
func TestComputeProposalTiming_HonestRecoveryAfterClampedLeader(t *testing.T) {
	parentPrev := int64(1000)
	parentTime := parentPrev + maxProposerForwardStep
	wall := parentPrev + 2
	d := computeProposalTiming(fixedNow(wall), parentTime, int64(viewChangeSlot), true)
	if d.ready || d.giveUp {
		t.Fatalf("expected sleep, got %+v", d)
	}
	expected := time.Duration(parentTime+1-wall) * time.Second
	if d.sleep != expected {
		t.Fatalf("expected %v sleep, got %v", expected, d.sleep)
	}
	if d.sleep > maxProposerCatchupWait {
		t.Fatalf("legitimate cascading aftermath sleep %v exceeds cap %v",
			d.sleep, maxProposerCatchupWait)
	}
}

// TestComputeProposalTiming_ConstantsAlignWithEngine pins down the relation
// between the proposer constants and the engine's maxBlockTimeStep. If these
// drift apart we get cross-version validation mismatches.
func TestComputeProposalTiming_ConstantsAlignWithEngine(t *testing.T) {
	if maxProposerForwardStep <= 0 {
		t.Fatalf("maxProposerForwardStep must be positive, got %d", maxProposerForwardStep)
	}
	if int64(viewChangeSlot) <= maxProposerForwardStep {
		t.Fatalf("viewChangeSlot (%d) must exceed maxProposerForwardStep (%d)",
			viewChangeSlot, maxProposerForwardStep)
	}
	if maxProposerCatchupWait < time.Duration(maxProposerForwardStep)*time.Second {
		t.Fatalf("maxProposerCatchupWait (%v) must be >= maxProposerForwardStep seconds (%ds) "+
			"to handle worst-case cascading-skew aftermath",
			maxProposerCatchupWait, maxProposerForwardStep)
	}
}
