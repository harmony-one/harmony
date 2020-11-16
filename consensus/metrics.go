package consensus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// consensusCounterVec is used to keep track of consensus reached
	consensusCounterVec = promauto.NewCounterVec(
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
	// consensusVCCounterVec is used to keep track of number of view change
	consensusVCCounterVec = promauto.NewCounterVec(
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
	// consensusSyncCounterVec is used to keep track of consensus syncing state
	consensusSyncCounterVec = promauto.NewCounterVec(
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
	// consensusGaugeVec is used to keep track of gauge number of the consensus
	consensusGaugeVec = promauto.NewGaugeVec(
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
	// consensusFinalityHistogram is used to keep track of finality
	// 10 ExponentialBuckets are in the unit of millisecond:
	// 800, 1000, 1250, 1562, 1953, 2441, 3051, 3814, 4768, 5960, inf
	consensusFinalityHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "finality",
			Help:      "the latency of the finality",
			Buckets:   prometheus.ExponentialBuckets(800, 1.25, 10),
		},
	)
)

// UpdateValidatorMetrics will udpate validator metrics
func (consensus *Consensus) UpdateValidatorMetrics(numSig float64, blockNum float64) {
	consensusCounterVec.With(prometheus.Labels{"consensus": "bingo"}).Inc()
	consensusGaugeVec.With(prometheus.Labels{"consensus": "signatures"}).Set(numSig)
	consensusCounterVec.With(prometheus.Labels{"consensus": "signatures"}).Add(numSig)
	consensusGaugeVec.With(prometheus.Labels{"consensus": "block_num"}).Set(blockNum)
}

// UpdateLeaderMetrics will udpate leader metrics
func (consensus *Consensus) UpdateLeaderMetrics(numCommits float64, blockNum float64) {
	consensusCounterVec.With(prometheus.Labels{"consensus": "hooray"}).Inc()
	consensusGaugeVec.With(prometheus.Labels{"consensus": "block_num"}).Set(blockNum)
	consensusCounterVec.With(prometheus.Labels{"consensus": "num_commits"}).Add(numCommits)
	consensusGaugeVec.With(prometheus.Labels{"consensus": "num_commits"}).Set(numCommits)
}
