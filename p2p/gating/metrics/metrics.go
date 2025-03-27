// Package metrics provides a set of metrics for the op-node.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	PeerUnbans = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "hmy",
		Subsystem: "p2p",
		Name:      "peer_unbans",
		Help:      "Count of peer unbans",
	},
	)

	IPUnbans = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "hmy",
		Subsystem: "p2p",
		Name:      "ip_unbans",
		Help:      "Count of IP unbans",
	},
	)
)

func RecordPeerUnban() {
	PeerUnbans.Inc()
}

func RecordIPUnban() {
	IPUnbans.Inc()
}
