package streammanager

import (
	"strings"
	"time"
)

// disconnectTracker detects mass stream removals that likely indicate a local
// network outage rather than many independent remote-peer failures.
type disconnectTracker struct {
	removalTimes     []time.Time
	localOutageUntil time.Time
	lastOutageStart  time.Time
}

// observeRemoval records a connection-loss stream removal and returns whether this
// removal falls inside a local-outage window. Punitive (bad-data / protocol) removals
// must not call this with connectionLoss=false counting — callers should only invoke
// when isConnectionLossReason(reason) is true.
//
// activeBefore is the number of streams (main+reserved) before this removal.
func (dt *disconnectTracker) observeRemoval(now time.Time, activeBefore int) (inLocalOutage bool, justEntered bool) {
	if !dt.localOutageUntil.IsZero() && now.Before(dt.localOutageUntil) {
		dt.record(now)
		return true, false
	}

	dt.prune(now)
	dt.record(now)

	threshold := massDisconnectThreshold(activeBefore + len(dt.removalTimes) - 1)
	// activeBefore + already-counted prior removals in the window approximates the
	// stream population at the start of the disconnect wave.
	if len(dt.removalTimes) < threshold {
		return false, false
	}

	// Rate-limit entering a new local-outage window.
	if !dt.lastOutageStart.IsZero() && now.Sub(dt.lastOutageStart) < localOutageMinInterval {
		return false, false
	}

	dt.localOutageUntil = now.Add(localOutageDuration)
	dt.lastOutageStart = now
	return true, true
}

func (dt *disconnectTracker) inLocalOutage(now time.Time) bool {
	return !dt.localOutageUntil.IsZero() && now.Before(dt.localOutageUntil)
}

func (dt *disconnectTracker) record(now time.Time) {
	dt.removalTimes = append(dt.removalTimes, now)
}

func (dt *disconnectTracker) prune(now time.Time) {
	cutoff := now.Add(-massDisconnectWindow)
	i := 0
	for i < len(dt.removalTimes) && dt.removalTimes[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		dt.removalTimes = append([]time.Time(nil), dt.removalTimes[i:]...)
	}
}

// massDisconnectThreshold returns how many removals in the window count as a mass event.
// Small stream sets use a lower floor so losing all/most peers still qualifies.
func massDisconnectThreshold(approxActiveAtWaveStart int) int {
	if approxActiveAtWaveStart <= 0 {
		return massDisconnectMinCount
	}
	if approxActiveAtWaveStart < massDisconnectMinCount {
		return approxActiveAtWaveStart
	}
	half := (approxActiveAtWaveStart + 1) / 2
	if half < massDisconnectMinCount {
		return massDisconnectMinCount
	}
	return half
}

// isPunitiveRemovalReason reports removals caused by bad/invalid peer behavior or
// protocol faults. These must keep hard cooldowns even during a local outage.
func isPunitiveRemovalReason(reason string) bool {
	r := strings.ToLower(reason)
	punitiveHints := []string{
		"too many failures",
		"nil block",
		"invalid block",
		"invalid stream",
		"protocol error",
		"critical protocol",
		"zero hashes",
		"all zero hashes",
		"empty blockbytes",
		"not valid",
		"unexpected blockbytes",
		"unmatched number",
		"unverifiable",
		"zero bytes block",
		"expected more hashes",
	}
	for _, hint := range punitiveHints {
		if strings.Contains(r, hint) {
			return true
		}
	}
	return false
}

// isConnectionLossReason reports removals that look like transport / local-network
// failures rather than malicious or buggy peer data. Only these may receive soft
// reconnect treatment under local-outage mode.
func isConnectionLossReason(reason string) bool {
	if reason == "" {
		return false
	}
	r := strings.ToLower(reason)
	if isPunitiveRemovalReason(r) {
		return false
	}
	connectionHints := []string{
		"read msg failed",
		"remote closed",
		"progress timeout",
		"too many recoverable errors",
		"stream error",
		"connection reset",
		"broken pipe",
		"local network",
		"network error",
		"disconnect",
	}
	for _, hint := range connectionHints {
		if strings.Contains(r, hint) {
			return true
		}
	}
	return false
}
