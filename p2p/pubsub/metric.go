package pubsub

import (
	"fmt"
	"sync/atomic"
	"time"
)

const (
	defaultMetricInterval = 30 * time.Second
)

// psMetric is the metric for p2p pub-sub validation.
type psMetric struct {
	tr          *topicRunner
	logInterval time.Duration

	numAccepted uint64
	numIgnored  uint64
	numRejected uint64

	resetC chan struct{}
	stopC  chan struct{}
}

func newPsMetric(tr *topicRunner, interval time.Duration) *psMetric {
	return &psMetric{
		tr:          tr,
		logInterval: interval,

		numAccepted: 0,
		numIgnored:  0,
		numRejected: 0,

		resetC: make(chan struct{}, 1),
		stopC:  make(chan struct{}),
	}
}

func (m *psMetric) run() {
	var (
		tick      = time.Tick(m.logInterval)
		timeStart = time.Now()
	)
	for {
		select {
		case <-tick:
			accept := atomic.SwapUint64(&m.numAccepted, 0)
			ignore := atomic.SwapUint64(&m.numIgnored, 0)
			reject := atomic.SwapUint64(&m.numRejected, 0)
			duration := time.Since(timeStart)
			m.writeLog(accept, ignore, reject, duration)

			for _, handler := range m.tr.getHandlers() {
				handler.Report()
			}

		case <-m.stopC:
			return
		}
	}
}

func (m *psMetric) stop() {
	m.stopC <- struct{}{}
}

func (m *psMetric) recordValidateResult(msg *message, action ValidateAction, err error) {
	switch action {
	case MsgAccept:
		atomic.AddUint64(&m.numAccepted, 1)
	case MsgIgnore:
		atomic.AddUint64(&m.numIgnored, 1)
	case MsgReject:
		atomic.AddUint64(&m.numRejected, 1)
	}
	return
}

func (m *psMetric) writeLog(accept, ignore, reject uint64, duration time.Duration) {
	m.tr.log.Info().Str("duration", duration.String()).
		Uint64("accepted", accept).
		Uint64("ignored", ignore).
		Uint64("rejected", reject).
		Msg(fmt.Sprintf("PubSub [%v] validation report", m.tr.topic))
}
