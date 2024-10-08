package node

import (
	"sync"

	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// nodeStringCounterVec is used to add version string or other static string
	// info into the metrics api
	nodeStringCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "node",
			Name:      "metadata",
			Help:      "a list of node metadata",
		},
		[]string{"key", "value"},
	)
	// nodeP2PMessageCounterVec is used to keep track of all p2p messages received
	nodeP2PMessageCounterVec = prometheus.NewCounterVec(
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
	// nodeConsensusMessageCounterVec is used to keep track of consensus p2p messages received
	nodeConsensusMessageCounterVec = prometheus.NewCounterVec(
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

	// nodeNodeMessageCounterVec is used to keep track of node p2p messages received
	nodeNodeMessageCounterVec = prometheus.NewCounterVec(
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

	// nodeCrossLinkMessageCounterVec is used to keep track of node new/invalid/duplicate crosslink messages received
	nodeCrossLinkMessageCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "crosslink_msg",
			Help:      "number of crosslink messages",
		},
		[]string{
			"type",
		},
	)

	onceMetrics sync.Once
)

func initMetrics() {
	onceMetrics.Do(func() {
		prom.PromRegistry().MustRegister(
			nodeStringCounterVec,
			nodeP2PMessageCounterVec,
			nodeConsensusMessageCounterVec,
			nodeNodeMessageCounterVec,
			nodeCrossLinkMessageCounterVec,
		)
	})
}
