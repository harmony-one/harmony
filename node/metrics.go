package node

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// NodeStringCounterVec is used to add version string or other static string
	// info into the metrics api
	NodeStringCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "node",
			Name:      "metadata",
			Help:      "a list of node metadata",
		},
		[]string{"key", "value"},
	)
	// NodeP2PMessageCounterVec is used to keep track of all p2p messages received
	NodeP2PMessageCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "message",
			Help:      "number of p2p messages",
		},
		[]string{
			"type",
		},
	)
	// NodeConsensusMessageCounterVec is used to keep track of consensus p2p messages received
	NodeConsensusMessageCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "consensus_msg",
			Help:      "number of consensus messages",
		},
		[]string{
			"type",
		},
	)

	// NodeNodeMessageCounterVec is used to keep track of node p2p messages received
	NodeNodeMessageCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "node_msg",
			Help:      "number of node messages",
		},
		[]string{
			"type",
		},
	)
)
