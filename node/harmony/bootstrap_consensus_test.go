package node

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForEnoughConsensusPeersChecksImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	connected, known, err := waitForEnoughConsensusPeers(ctx, 4, 3*time.Second, func() (int, int) {
		return 8, 9
	})
	if err != nil {
		t.Fatalf("waitForEnoughConsensusPeers returned an error: %v", err)
	}
	if connected != 8 || known != 9 {
		t.Fatalf("unexpected peer counts: connected=%d known=%d", connected, known)
	}
}

func TestWaitForEnoughConsensusPeersRechecksUntilThreshold(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	checks := 0
	connected, known, err := waitForEnoughConsensusPeers(ctx, 4, time.Millisecond, func() (int, int) {
		checks++
		if checks == 1 {
			return 3, 8
		}
		return 4, 9
	})
	if err != nil {
		t.Fatalf("waitForEnoughConsensusPeers returned an error: %v", err)
	}
	if checks != 2 {
		t.Fatalf("expected 2 peer checks, got %d", checks)
	}
	if connected != 4 || known != 9 {
		t.Fatalf("unexpected peer counts: connected=%d known=%d", connected, known)
	}
}

func TestWaitForEnoughConsensusPeersStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := waitForEnoughConsensusPeers(ctx, 4, time.Hour, func() (int, int) {
		return 3, 8
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
