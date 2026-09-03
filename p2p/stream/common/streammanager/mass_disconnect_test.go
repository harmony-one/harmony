package streammanager

import (
	"testing"
	"time"
)

func TestMassDisconnectThreshold(t *testing.T) {
	tests := []struct {
		active int
		want   int
	}{
		{0, massDisconnectMinCount},
		{1, 1},
		{2, 2},
		{3, massDisconnectMinCount},
		{4, massDisconnectMinCount},
		{8, 4},
		{10, 5},
	}
	for _, tt := range tests {
		if got := massDisconnectThreshold(tt.active); got != tt.want {
			t.Fatalf("active=%d: got %d want %d", tt.active, got, tt.want)
		}
	}
}

func TestIsConnectionLossReason(t *testing.T) {
	loss := []string{
		"force close: remote closed stream",
		"force close: read msg failed",
		"force close: progress timeout",
		"force close: connection reset",
		"force close: broken pipe",
	}
	for _, r := range loss {
		if !isConnectionLossReason(r) {
			t.Fatalf("expected connection-loss: %q", r)
		}
	}
	punitive := []string{
		"force close: too many failures",
		"force close: nil block hashes",
		"force close: identifySyncedStreams: critical protocol error",
		"force close: invalid block is received from stream",
		"force close: downloadRawBlocks received blockBytes are not valid",
		"reset", // bare / ambiguous reasons are not treated as connection-loss
	}
	for _, r := range punitive {
		if isConnectionLossReason(r) {
			t.Fatalf("expected punitive/non-loss: %q", r)
		}
	}
}

func TestDisconnectTracker_MassDetectsLocalOutage(t *testing.T) {
	var dt disconnectTracker
	now := time.Now()

	// 8 streams: threshold = 4
	for i := 0; i < 3; i++ {
		in, entered := dt.observeRemoval(now.Add(time.Duration(i)*time.Second), 8-i)
		if in || entered {
			t.Fatalf("removal %d should not trigger local outage yet", i+1)
		}
	}
	in, entered := dt.observeRemoval(now.Add(4*time.Second), 5)
	if !in || !entered {
		t.Fatalf("4th removal should enter local outage, in=%v entered=%v", in, entered)
	}
	if !dt.inLocalOutage(now.Add(5 * time.Second)) {
		t.Fatal("expected active local outage")
	}

	// Subsequent removal while in outage stays in outage without re-entering.
	in, entered = dt.observeRemoval(now.Add(6*time.Second), 4)
	if !in || entered {
		t.Fatalf("while in outage: in=%v entered=%v", in, entered)
	}
}

func TestDisconnectTracker_RateLimitsOutageEntry(t *testing.T) {
	var dt disconnectTracker
	now := time.Now()

	// Enter outage.
	for i := 0; i < 4; i++ {
		dt.observeRemoval(now.Add(time.Duration(i)*time.Second), 8-i)
	}
	if !dt.inLocalOutage(now.Add(4 * time.Second)) {
		t.Fatal("expected first outage")
	}

	// Expire outage window but stay inside min interval.
	dt.localOutageUntil = now
	dt.removalTimes = nil
	later := now.Add(localOutageDuration + time.Minute) // still < 10m from lastOutageStart
	for i := 0; i < 4; i++ {
		in, entered := dt.observeRemoval(later.Add(time.Duration(i)*time.Second), 8-i)
		if in || entered {
			t.Fatalf("rate-limited re-entry should not start outage at removal %d", i+1)
		}
	}

	// After min interval, a new wave may enter outage again.
	dt.removalTimes = nil
	reopen := now.Add(localOutageMinInterval + time.Second)
	for i := 0; i < 3; i++ {
		dt.observeRemoval(reopen.Add(time.Duration(i)*time.Second), 8-i)
	}
	in, entered := dt.observeRemoval(reopen.Add(4*time.Second), 5)
	if !in || !entered {
		t.Fatalf("after min interval should allow new outage, in=%v entered=%v", in, entered)
	}
}

func TestDisconnectTracker_SmallSetLosingAllPeers(t *testing.T) {
	var dt disconnectTracker
	now := time.Now()

	in, entered := dt.observeRemoval(now, 2)
	if in || entered {
		t.Fatal("first of two should not trigger yet")
	}
	in, entered = dt.observeRemoval(now.Add(time.Second), 1)
	if !in || !entered {
		t.Fatalf("losing both peers should trigger local outage, in=%v entered=%v", in, entered)
	}
}

func TestHandleRemoveStream_MassDisconnectSkipsCriticalCooldownForConnectionLoss(t *testing.T) {
	sm := newTestStreamManager()
	sm.pf = newTestPeerFinder(nil, emptyDelayFunc)
	sm.config.HardLoCap = 2
	sm.config.SoftLoCap = 2
	sm.Start()
	defer sm.Close()

	const lossReason = "force close: remote closed stream"

	for i := 1; i <= 4; i++ {
		if err := sm.NewStream(newTestStream(makeStreamID(i), testProtoID)); err != nil {
			t.Fatalf("add stream %d: %v", i, err)
		}
	}

	if err := sm.RemoveStream(makeStreamID(1), lossReason, true); err != nil {
		t.Fatalf("remove 1: %v", err)
	}
	info, ok := sm.removedStreams.Get(makeStreamID(1))
	if !ok {
		t.Fatal("expected removal info for stream 1")
	}
	if info.HasExpired() {
		t.Fatal("first critical connection-loss should still have cooldown before mass detect")
	}

	if err := sm.RemoveStream(makeStreamID(2), lossReason, true); err != nil {
		t.Fatalf("remove 2: %v", err)
	}
	if err := sm.RemoveStream(makeStreamID(3), lossReason, true); err != nil {
		t.Fatalf("remove 3: %v", err)
	}

	info3, ok := sm.removedStreams.Get(makeStreamID(3))
	if !ok {
		t.Fatal("expected removal info for stream 3")
	}
	if !info3.HasExpired() {
		t.Fatal("mass-disconnect connection-loss should allow immediate reconnect")
	}
	if !sm.disconnectTracker.inLocalOutage(time.Now()) {
		t.Fatal("expected local outage after mass disconnect")
	}

	if err := sm.RemoveStream(makeStreamID(4), lossReason, true); err != nil {
		t.Fatalf("remove 4: %v", err)
	}
	info4, ok := sm.removedStreams.Get(makeStreamID(4))
	if !ok {
		t.Fatal("expected removal info for stream 4")
	}
	if !info4.HasExpired() {
		t.Fatal("connection-loss during local outage should allow immediate reconnect")
	}
}

func TestHandleRemoveStream_PunitiveKeepsCriticalCooldownDuringOutage(t *testing.T) {
	sm := newTestStreamManager()
	sm.pf = newTestPeerFinder(nil, emptyDelayFunc)
	sm.config.HardLoCap = 2
	sm.config.SoftLoCap = 2
	sm.Start()
	defer sm.Close()

	const lossReason = "force close: remote closed stream"
	const badReason = "force close: identifySyncedStreams: critical protocol error"

	for i := 1; i <= 5; i++ {
		if err := sm.NewStream(newTestStream(makeStreamID(i), testProtoID)); err != nil {
			t.Fatalf("add stream %d: %v", i, err)
		}
	}

	// Trigger local outage with connection-loss removals.
	for i := 1; i <= 3; i++ {
		if err := sm.RemoveStream(makeStreamID(i), lossReason, true); err != nil {
			t.Fatalf("loss remove %d: %v", i, err)
		}
	}
	if !sm.disconnectTracker.inLocalOutage(time.Now()) {
		t.Fatal("expected local outage")
	}

	if err := sm.RemoveStream(makeStreamID(4), badReason, true); err != nil {
		t.Fatalf("punitive remove: %v", err)
	}
	info, ok := sm.removedStreams.Get(makeStreamID(4))
	if !ok {
		t.Fatal("expected removal info for punitive peer")
	}
	if info.HasExpired() {
		t.Fatal("punitive removal during local outage must keep critical cooldown")
	}
}

func TestHandleRemoveStream_PunitiveDoesNotTriggerMassDisconnect(t *testing.T) {
	sm := newTestStreamManager()
	sm.pf = newTestPeerFinder(nil, emptyDelayFunc)
	sm.config.HardLoCap = 2
	sm.config.SoftLoCap = 2
	sm.Start()
	defer sm.Close()

	const badReason = "force close: nil block hashes"
	for i := 1; i <= 4; i++ {
		if err := sm.NewStream(newTestStream(makeStreamID(i), testProtoID)); err != nil {
			t.Fatalf("add stream %d: %v", i, err)
		}
	}
	for i := 1; i <= 4; i++ {
		if err := sm.RemoveStream(makeStreamID(i), badReason, true); err != nil {
			t.Fatalf("remove %d: %v", i, err)
		}
	}
	if sm.disconnectTracker.inLocalOutage(time.Now()) {
		t.Fatal("punitive removals must not open a local-outage window")
	}
	for i := 1; i <= 4; i++ {
		info, ok := sm.removedStreams.Get(makeStreamID(i))
		if !ok {
			t.Fatalf("missing removal info %d", i)
		}
		if info.HasExpired() {
			t.Fatalf("punitive peer %d should still be on critical cooldown", i)
		}
	}
}
