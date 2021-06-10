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

	rateLimitRejectedCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "msg_rate_limited",
			Help:      "number of pub-sub message rejected and dropped due to rate limit",
		},
		[]string{
			"peer",
		},
	)

	blacklistRejectedCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "p2p",
			Name:      "rate_limited",
			Help:      "number of pub-sub message rejected and dropped since the sender is blacklisted",
		},
		[]string{
			"peer",
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
		)
	})
}
