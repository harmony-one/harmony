package pubsub

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

const (
	defaultMetricInterval = 30 * time.Second
)

// psMetric is the metric for p2p pub-sub validation.
type psMetric struct {
	topic       Topic
	logInterval time.Duration

	numAccepted uint64
	numIgnored  uint64
	numRejected uint64

	resetC chan struct{}
	stopC  chan struct{}
	log    zerolog.Logger
}

func newPsMetric(topic Topic, interval time.Duration, log zerolog.Logger) *psMetric {
	return &psMetric{
		topic:       topic,
		logInterval: interval,

		numAccepted: 0,
		numIgnored:  0,
		numRejected: 0,

		resetC: make(chan struct{}, 1),
		stopC:  make(chan struct{}),
		log:    log,
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
	m.log.Info().Str("duration", duration.String()).
		Uint64("accepted", accept).
		Uint64("ignored", ignore).
		Uint64("rejected", reject).
		Msg(fmt.Sprintf("PubSub [%v] validation report", m.topic))
}
