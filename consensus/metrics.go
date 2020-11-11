package consensus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ConsensusCounterVec is used to keep track of consensus reached
	ConsensusCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "bingo",
			Help:      "counter of consensus",
		},
		[]string{
			"consensus",
		},
	)
	// ConsensusVCCounterVec is used to keep track of number of view change
	ConsensusVCCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "viewchange",
			Help:      "counter of view chagne",
		},
		[]string{
			"viewchange",
		},
	)
	// ConsensusSyncCounterVec is used to keep track of consensus syncing state
	ConsensusSyncCounterVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "sync",
			Help:      "counter of blockchain syncing state",
		},
		[]string{
			"consensus",
		},
	)
	// ConsensusGaugeVec is used to keep track of gauge number of the consensus
	ConsensusGaugeVec = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "signatures",
			Help:      "number of signatures or commits",
		},
		[]string{
			"consensus",
		},
	)
	// ConsensusFinalityHistogram is used to keep track of finality
	// 10 ExponentialBuckets are in the unit of millisecond:
	// 800, 1000, 1250, 1562, 1953, 2441, 3051, 3814, 4768, 5960, inf
	ConsensusFinalityHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "finality",
			Help:      "the latency of the finality",
			Buckets:   prometheus.ExponentialBuckets(800, 1.25, 10),
		},
	)
)
