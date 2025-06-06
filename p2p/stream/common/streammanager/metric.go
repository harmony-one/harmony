package streammanager

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		discoverCounterVec,
		discoveredPeersCounterVec,
		addedStreamsCounterVec,
		removedStreamsCounterVec,
		setupStreamDuration,
		numStreamsGaugeVec,
		numReservedStreamsGaugeVec,
	)
}

var (
	discoverCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "discover",
			Help:      "number of intentions to actively discover peers",
		},
		[]string{"topic"},
	)

	discoveredPeersCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "discover_peers",
			Help:      "number of peers discovered and connect actively",
		},
		[]string{"topic"},
	)

	addedStreamsCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "added_streams",
			Help:      "number of streams added in stream manager",
		},
		[]string{"topic"},
	)

	removedStreamsCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "removed_streams",
			Help:      "number of streams removed in stream manager",
		},
		[]string{"topic"},
	)

	streamCriticalErrorCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "stream_critical_errors",
			Help:      "number of critical errors of stream removals in stream manager",
		},
		[]string{"topic"},
	)

	streamRemovalReasonCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "stream_removal_reasons",
			Help:      "removal reason of streams in stream manager",
		},
		[]string{"reason", "critical"},
	)

	setupStreamDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "setup_stream_duration",
			Help:      "duration in seconds of setting up connection to a discovered peer",
			// buckets: 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1280ms, +INF
			Buckets: prometheus.ExponentialBuckets(0.02, 2, 8),
		},
		[]string{"topic"},
	)

	numStreamsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "num_streams",
			Help:      "number of connected streams",
		},
		[]string{"topic"},
	)

	numReservedStreamsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "num_reserved_streams",
			Help:      "number of reserved streams",
		},
		[]string{"topic"},
	)
)
