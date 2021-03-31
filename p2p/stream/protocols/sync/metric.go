package sync

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		numClientRequestCounterVec,
		failedClientRequestCounterVec,
		clientRequestDurationVec,
		serverRequestCounterVec,
	)
}

var (
	numClientRequestCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream_sync",
			Name:      "client_request",
			Help:      "number of outgoing requests as a client",
		},
		[]string{"topic", "request_type"},
	)

	failedClientRequestCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream_sync",
			Name:      "failed_client_request",
			Help:      "failed outgoing request as a client",
		},
		[]string{"topic", "request_type", "error"},
	)

	clientRequestDurationVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "stream_sync",
			Name:      "client_request_delay",
			Help:      "delay in seconds to do sync requests as a client",
			// buckets: 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1280ms, +INF
			Buckets: prometheus.ExponentialBuckets(0.02, 2, 8),
		},
		[]string{"topic", "request_type"},
	)

	serverRequestCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream_sync",
			Name:      "server_request",
			Help:      "number of incoming request as a server",
		},
		[]string{"topic", "request_type"},
	)
)

func (p *Protocol) doMetricClientRequest(reqType string) *prometheus.Timer {
	pLabel := p.getClientPromLabel(reqType)
	numClientRequestCounterVec.With(pLabel).Inc()
	timer := prometheus.NewTimer(clientRequestDurationVec.With(pLabel))
	return timer
}

func (p *Protocol) doMetricPostClientRequest(reqType string, err error, timer *prometheus.Timer) {
	timer.ObserveDuration()
	pLabel := p.getClientPromLabel(reqType)
	if err != nil {
		pLabel["error"] = err.Error()
		failedClientRequestCounterVec.With(pLabel).Inc()
	}
}

func (p *Protocol) getClientPromLabel(reqType string) prometheus.Labels {
	return prometheus.Labels{
		"topic":        string(p.ProtoID()),
		"request_type": reqType,
	}
}
