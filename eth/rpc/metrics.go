package rpc

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		requestCounterVec,
		requestErroredCounterVec,
		requestDurationHistVec,
	)
}

var (
	requestCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "rpc2",
			Name:      "request_count",
			Help:      "counters for each RPC method",
		},
		[]string{"method"},
	)

	requestErroredCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "rpc2",
			Name:      "err_count",
			Help:      "counters of errored RPC method",
		},
		[]string{"method"},
	)

	requestDurationHistVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "rpc2",
			Name:      "delay_histogram",
			Help:      "delays histogram in seconds",
			// buckets: 50ms, 100ms, 200ms, 400ms, 800ms, 1600ms, 3200ms, +INF
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 8),
		},
		[]string{"method"},
	)
)

func doMetricRequest(method string) *prometheus.Timer {
	pLabel := prometheus.Labels{
		"method": method,
	}
	requestCounterVec.With(pLabel).Inc()
	timer := prometheus.NewTimer(requestDurationHistVec.With(pLabel))
	return timer
}

func doMetricErroredRequest(method string) {
	pLabel := prometheus.Labels{
		"method": method,
	}
	requestErroredCounterVec.With(pLabel).Inc()
}

func doMetricDelayHist(timer *prometheus.Timer) {
	timer.ObserveDuration()
}
