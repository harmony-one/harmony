package timeouts

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Named ..
type Named string

const (
	// ViewChange ..
	ViewChange Named = "vc-timeout"
	// Consensus ..
	Consensus Named = "consensus-timeout"
)

const (
	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration

	// ViewChangeDuration ..
	ViewChangeDuration time.Duration = 60 * time.Second
	// timeout duration for announce/prepare/commit

	// PhaseDuration ..
	PhaseDuration time.Duration = 60 * time.Second
)

// ComeDue ..
type ComeDue struct {
	Name      Named
	startTime atomic.Value
	limit     atomic.Value
}

// Start ..
func (d *ComeDue) Start() {
	d.startTime.Store(time.Now())
}

// SetDuration ..
func (d *ComeDue) SetDuration(dur time.Duration) {
	d.limit.Store(dur)
}

// func (d *ComeDue) WithinLimit() bool {
// 	elapsed := time.Since(d.startTime.Load().(time.Time))
// 	return elapsed.Round(time.Second) < d.duration
// }

// DueWithValue ..
type DueWithValue struct {
	ComeDue
	Value uint64
}

// Notify ..
func (r *Notifier) Notify(blkNum uint64, viewID uint64) <-chan DueWithValue {
	timedOut := make(chan DueWithValue)

	go func() {
		for {
			then := time.Now()
			time.Sleep(r.Consensus.limit.Load().(time.Duration))
			timedOut <- DueWithValue{ComeDue: r.Consensus, Value: blkNum}
			fmt.Println("notify occured this much later", time.Since(then).Round(time.Second))
			return
		}
	}()

	go func() {
		for {
			then := time.Now()
			time.Sleep(r.ViewChange.limit.Load().(time.Duration))
			timedOut <- DueWithValue{ComeDue: r.ViewChange, Value: viewID}
			fmt.Println("notify occured this much later", time.Since(then).Round(time.Second))
			return
		}
	}()

	return timedOut

}

// Notifier ..
type Notifier struct {
	Consensus  ComeDue
	ViewChange ComeDue
}

// NewNotifier ..
func NewNotifier() *Notifier {
	var d, v atomic.Value

	d.Store(PhaseDuration)
	v.Store(ViewChangeDuration)

	return &Notifier{
		Consensus: ComeDue{
			Name:  Consensus,
			limit: d,
		},
		ViewChange: ComeDue{
			Name:  ViewChange,
			limit: v,
		},
	}
}
