package core

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		receivedTxsCounter,
		stuckTxsCounter,
		invalidTxsCounterVec,
		knownTxsCounter,
		lowNonceTxCounter,
		pendingTxGauge,
		queuedTxGauge,
	)
}

var (
	receivedTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "received",
			Help:      "number of transactions received",
		},
	)

	stuckTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "stuck",
			Help:      "number of transactions observed to be stuck and broadcast again",
		},
	)

	invalidTxsCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "invalid",
			Help:      "transactions failed validation",
		},
		[]string{"err"},
	)

	knownTxsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "known",
			Help:      "number of known transaction received",
		},
	)

	lowNonceTxCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "low_nonce",
			Help:      "number of transactions removed because of low nonce (including processed transactions)",
		},
	)

	pendingTxGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "pending",
			Help:      "number of executable transactions",
		},
	)

	queuedTxGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "txPool",
			Name:      "queued",
			Help:      "number of queued non-executable transactions",
		},
	)
)
