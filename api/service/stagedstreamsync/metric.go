package stagedstreamsync

import (
	"fmt"

	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prom.PromRegistry().MustRegister(
		consensusTriggeredDownloadCounterVec,
		longRangeSyncedBlockCounterVec,
		longRangeFailInsertedBlockCounterVec,
		numShortRangeCounterVec,
		numFailedDownloadCounterVec,
		numBlocksInsertedShortRangeHistogramVec,
		numBlocksInsertedBeaconHelperCounter,
	)
}

var (
	consensusTriggeredDownloadCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "consensus_trigger",
			Help:      "number of times consensus triggered download task",
		},
		[]string{"ShardID"},
	)

	longRangeSyncedBlockCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "num_blocks_synced_long_range",
			Help:      "number of blocks synced in long range sync",
		},
		[]string{"ShardID"},
	)

	longRangeFailInsertedBlockCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "num_blocks_failed_long_range",
			Help:      "number of blocks failed to insert into change in long range sync",
		},
		[]string{"ShardID", "error"},
	)

	numShortRangeCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "num_short_range",
			Help:      "number of short range sync is triggered",
		},
		[]string{"ShardID"},
	)

	numFailedDownloadCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "failed_download",
			Help:      "number of downloading is failed",
		},
		[]string{"ShardID", "error"},
	)

	numBlocksInsertedShortRangeHistogramVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "num_blocks_inserted_short_range",
			Help:      "number of blocks inserted for each short range sync",
			// Buckets: 0, 1, 2, 4, +INF (capped at 10)
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 5),
		},
		[]string{"ShardID"},
	)

	numBlocksInsertedBeaconHelperCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "staged_stream_sync",
			Name:      "num_blocks_inserted_beacon_helper",
			Help:      "number of blocks inserted from beacon helper",
		},
	)
)

func (d *Downloader) promLabels() prometheus.Labels {
	sid := d.bc.ShardID()
	return prometheus.Labels{"ShardID": fmt.Sprintf("%d", sid)}
}
