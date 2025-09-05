package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// CrossLinkPendingQueueGauge is used to monitor the current size of pending crosslink queue
	CrossLinkPendingQueueGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "crosslink_pending_queue_size",
			Help:      "current number of crosslinks in pending queue",
		},
	)
)

func init() {
	// Register the crosslink metrics
	prometheus.MustRegister(CrossLinkPendingQueueGauge)
}

// UpdatePendingCrossLinkQueueGauge updates the pending crosslink queue size gauge
func UpdatePendingCrossLinkQueueGauge(size int) {
	CrossLinkPendingQueueGauge.Set(float64(size))
}
