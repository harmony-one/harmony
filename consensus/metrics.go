package consensus

import (
	"fmt"
	"sync"

	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	preimageStartGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "hmy",
			Subsystem:   "blockchain",
			Name:        "preimage_start",
			Help:        "the first block for which pre-image generation ran locally",
			ConstLabels: map[string]string{},
		},
		[]string{
			"shard",
		},
	)
	preimageEndGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "blockchain",
			Name:      "preimage_end",
			Help:      "the last block for which pre-image generation ran locally",
		},
		[]string{
			"shard",
		},
	)
	verifiedPreimagesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "blockchain",
			Name:      "verified_preimages",
			Help:      "the number of verified preimages",
		},
		[]string{
			"shard",
		},
	)
	lastPreimageImportGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "blockchain",
			Name:      "last_preimage_import",
			Help:      "the last known block for which preimages were imported",
		},
		[]string{
			"shard",
		},
	)

	// consensusCounterVec is used to keep track of consensus reached
	consensusCounterVec = prometheus.NewCounterVec(
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
	consensusVCCounterVec = prometheus.NewCounterVec(
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
	consensusSyncCounterVec = prometheus.NewCounterVec(
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
	consensusGaugeVec = prometheus.NewGaugeVec(
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
	// consensusPubkeyVec is used to keep track of bls pubkeys
	consensusPubkeyVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "blskeys",
			Help:      "list of bls pubkey",
		},
		[]string{
			"index", "pubkey",
		},
	)
	// consensusFinalityHistogram is used to keep track of finality
	// 10 ExponentialBuckets are in the unit of millisecond:
	// 800, 1000, 1250, 1562, 1953, 2441, 3051, 3814, 4768, 5960, inf
	consensusFinalityHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "consensus",
			Name:      "finality",
			Help:      "the latency of the finality",
			Buckets:   prometheus.ExponentialBuckets(800, 1.25, 10),
		},
	)

	onceMetrics sync.Once

	// TODO: add last consensus timestamp, add view ID
	// add last view change timestamp
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
func (consensus *Consensus) UpdatePreimageGenerationMetrics(
	preimageStart uint64,
	preimageEnd uint64,
	lastPreimageImport uint64,
	verifiedAddresses uint64,
	shard uint32,
) {
	if lastPreimageImport > 0 {
		lastPreimageImportGauge.With(prometheus.Labels{"shard": fmt.Sprintf("%d", shard)}).Set(float64(lastPreimageImport))
	}
	if preimageStart > 0 {
		preimageStartGauge.With(prometheus.Labels{"shard": fmt.Sprintf("%d", shard)}).Set(float64(preimageStart))
	}
	if preimageEnd > 0 {
		preimageEndGauge.With(prometheus.Labels{"shard": fmt.Sprintf("%d", shard)}).Set(float64(preimageEnd))
	}
	if verifiedAddresses > 0 {
		verifiedPreimagesGauge.With(prometheus.Labels{"shard": fmt.Sprintf("%d", shard)}).Set(float64(verifiedAddresses))
	}
}

// AddPubkeyMetrics add the list of blskeys to prometheus metrics
func (consensus *Consensus) AddPubkeyMetrics() {
	keys := consensus.GetPublicKeys()
	for i, key := range keys {
		index := fmt.Sprintf("%d", i)
		consensusPubkeyVec.With(prometheus.Labels{"index": index, "pubkey": key.Bytes.Hex()}).Set(float64(i))
	}
}

func initMetrics() {
	onceMetrics.Do(func() {
		prom.PromRegistry().MustRegister(
			consensusCounterVec,
			consensusVCCounterVec,
			consensusSyncCounterVec,
			consensusGaugeVec,
			consensusPubkeyVec,
			consensusFinalityHistogram,
			lastPreimageImportGauge,
			preimageEndGauge,
			preimageStartGauge,
			verifiedPreimagesGauge,
		)
	})
}
