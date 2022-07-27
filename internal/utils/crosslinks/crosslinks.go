package crosslinks

import (
	"sync/atomic"

	"github.com/harmony-one/harmony/core/types"
)

// Crosslinks is helper for processing crosslinks. It has meaning only for shards.
type Crosslinks struct {
	// This number stores value we have already sent.
	// It can be ahead of CrosslinkHeartbeat.LatestContinuousBlockNum, because processing of crosslink takes some time.
	latestSentCrosslinkBlockNumber uint64
	// Latest received valid heartbeat signal.
	lastKnownCrosslinkHeartbeatSignal atomic.Value
}

// New creates new Crosslinks.
func New() *Crosslinks {
	return &Crosslinks{}
}

// LatestSentCrosslinkBlockNumber returns last set value for crosslink block number or zero if not.
func (a *Crosslinks) LatestSentCrosslinkBlockNumber() uint64 {
	return atomic.LoadUint64(&a.latestSentCrosslinkBlockNumber)
}

// SetLatestSentCrosslinkBlockNumber sets last crosslink block number.
func (a *Crosslinks) SetLatestSentCrosslinkBlockNumber(number uint64) {
	atomic.StoreUint64(&a.latestSentCrosslinkBlockNumber, number)
}

// LastKnownCrosslinkHeartbeatSignal returns last set value for heartbeat or nil if not.
func (a *Crosslinks) LastKnownCrosslinkHeartbeatSignal() *types.CrosslinkHeartbeat {
	val := a.lastKnownCrosslinkHeartbeatSignal.Load()
	if val == nil {
		return nil
	}
	return val.(*types.CrosslinkHeartbeat)
}

// SetLastKnownCrosslinkHeartbeatSignal sets last known heartbeat.
func (a *Crosslinks) SetLastKnownCrosslinkHeartbeatSignal(signal *types.CrosslinkHeartbeat) {
	a.lastKnownCrosslinkHeartbeatSignal.Store(signal)
}
