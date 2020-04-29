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
}

// Start ..
func (d *ComeDue) Start(value uint64) {
	d.startTime.Store(time.Now())
	d.startValue.Store(value)
}

// SetDuration ..
func (d *ComeDue) SetDuration(dur time.Duration) {
	d.limit.Store(dur)
}

// Notify ..
func (d *ComeDue) Notify() <-chan uint64 {
	timedOut := make(chan uint64)

	go func() {
		for {

			time.AfterFunc(d.limit.Load().(time.Duration), func() {
				elapsed := time.Since(d.startTime.Load().(time.Time))
				if elapsed.Round(time.Second) > d.limit.Load().(time.Duration) {
					timedOut <- d.startValue.Load().(uint64)
				}
			})

		}
	}()

	return timedOut
}

// func (d *ComeDue) WithinLimit() bool {
// }

// DueWithValue ..
// type DueWithValue struct {
// 	Value uint64
// }

// Notify ..
// func (r *Notifier) Notify() <-chan DueWithValue {

// 	time.AfterFunc(r.Consensus.limit.Load().(time.Duration), func() {
// 		timedOut <- DueWithValue{
// 			Name:  r.Consensus.Name,
// 			Value: r.Consensus.startValue.Load().(uint64),
// 		}
// 	})

// 	// go func() {
// 	// 	for {
// 	// 		then := time.Now()
// 	// 		time.Sleep(r.ViewChange.limit.Load().(time.Duration))
// 	// 		timedOut <- DueWithValue{
// 	// 			Name:  r.ViewChange.Name,
// 	// 			Value: r.ViewChange.startValue.Load().(uint64),
// 	// 		}
// 	// 		fmt.Println("notify occured this much later", time.Since(then).Round(time.Second))
// 	// 		return
// 	// 	}
// 	// }()

// 	return timedOut

// }

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
		},
		ViewChange: ComeDue{
			Name:       ViewChange,
			limit:      v,
			startValue: s2,
		},
	}
}
