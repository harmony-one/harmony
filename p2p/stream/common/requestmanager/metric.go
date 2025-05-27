package requestmanager

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		numConnectedStreamsVec,
		numAvailableStreamsVec,
		numWaitingRequestsVec,
		numPendingRequestsVec,
	)
}

var (
	numConnectedStreamsVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "request_manager_num_connected_streams",
			Help:      "number of connected streams",
		},
		[]string{"topic"},
	)

	numAvailableStreamsVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "request_manager_num_available_streams",
			Help:      "number of available streams",
		},
		[]string{"topic"},
	)

	numWaitingRequestsVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "num_waiting_requests",
			Help:      "number of waiting requests",
		},
		[]string{"topic"},
	)

	numPendingRequestsVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "num_pending_requests",
			Help:      "number of pending requests",
		},
		[]string{"topic"},
	)
)
