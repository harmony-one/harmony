package core

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		deletedValidatorSnapshot,
		skipValidatorSnapshot,
		deletedBlockCount,
		prunerMaxBlock,
		deletedBlockCountUsedTime,
		compactBlockCountUsedTime,
	)
}

var (
	deletedValidatorSnapshot = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "blockchain_pruner",
			Name:      "deleted_validator_snapshot",
			Help:      "number of deleted validator snapshot count",
		},
	)

	skipValidatorSnapshot = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "skip_validator_snapshot",
			Help:      "number of skip validator snapshot count",
		},
	)

	deletedBlockCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "blockchain_pruner",
			Name:      "deleted_block_count",
			Help:      "number of deleted block count",
		},
	)

	prunerMaxBlock = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "stream",
			Name:      "pruner_max_block",
			Help:      "number of largest pruner block",
		},
	)

	deletedBlockCountUsedTime = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "blockchain_pruner",
			Name:      "deleted_block_count_used_time",
			Help:      "sum of deleted block used time in ms",
		},
	)

	compactBlockCountUsedTime = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "blockchain_pruner",
			Name:      "compact_block_count_used_time",
			Help:      "sum of compact block time in ms",
		},
	)
)
