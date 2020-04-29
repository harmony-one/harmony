package timeouts

import (
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

// DueWithValue ..
// type DueWithValue struct {
// 	Value uint64
// }

const (
	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration

	// ViewChangeDuration ..
	ViewChangeDuration time.Duration = 60 * time.Second
	// timeout duration for announce/prepare/commit

	// PhaseDuration ..
	PhaseDuration time.Duration = 15 * time.Second
)

// ComeDue ..
type ComeDue struct {
	Name       Named
	startTime  atomic.Value
	limit      atomic.Value
	startValue atomic.Value
	TimedOut   chan uint64
}

// Start ..
func (d *ComeDue) Start(value uint64) {
	var t *time.Timer

	if t != nil {
		t.Stop()
	}

	n := time.Now()

	func() {
		d.startTime.Store(n)
		d.startValue.Store(value)

		t = time.AfterFunc(time.Until(n.Add(d.limit.Load().(time.Duration))), func() {
			d.TimedOut <- value
			// t.Stop()
		})

	}()
}

// SetDuration ..
func (d *ComeDue) SetDuration(dur time.Duration) {
	d.limit.Store(dur)
}

// Notifier ..
type Notifier struct {
	Consensus  ComeDue
	ViewChange ComeDue
}

// NewNotifier ..
func NewNotifier() *Notifier {
	var d, v, s1, s2 atomic.Value

	d.Store(PhaseDuration)
	v.Store(ViewChangeDuration)
	s1.Store(uint64(0))
	s2.Store(uint64(0))

	return &Notifier{
		Consensus: ComeDue{
			Name:       Consensus,
			limit:      d,
			startValue: s1,
			TimedOut:   make(chan uint64),
		},
		ViewChange: ComeDue{
			Name:       ViewChange,
			limit:      v,
			startValue: s2,
			TimedOut:   make(chan uint64),
		},
	}
}
