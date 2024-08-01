package downloader

import (
	"time"

	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const (
	numBlocksByNumPerRequest int = 10 // number of blocks for each request
	blocksPerInsert          int = 50 // number of blocks for each insert batch

	numBlockHashesPerRequest  int = 20 // number of get block hashes for short range sync
	numBlocksByHashesUpperCap int = 10 // number of get blocks by hashes upper cap
	numBlocksByHashesLowerCap int = 3  // number of get blocks by hashes lower cap

	lastMileThres int = 10

	// soft cap of size in resultQueue. When the queue size is larger than this limit,
	// no more request will be assigned to workers to wait for InsertChain to finish.
	softQueueCap int = 100

	// defaultConcurrency is the default settings for concurrency
	defaultConcurrency = 4

	// shortRangeTimeout is the timeout for each short range sync, which allow short range sync
	// to restart automatically when stuck in `getBlockHashes`
	shortRangeTimeout = 1 * time.Minute
)

type (
	// Config is the downloader config
	Config struct {
		// Only run stream sync protocol as a server.
		// TODO: remove this when stream sync is fully up.
		ServerOnly bool
		// use staged sync
		Staged bool
		// parameters
		Network     nodeconfig.NetworkType
		Concurrency int // Number of concurrent sync requests
		MinStreams  int // Minimum number of streams to do sync
		InitStreams int // Number of streams requirement for initial bootstrap

		// stream manager config
		SmSoftLowCap int
		SmHardLowCap int
		SmHiCap      int
		SmDiscBatch  int

		// config for beacon config
		BHConfig *BeaconHelperConfig
	}

	// BeaconHelperConfig is the extra config used for beaconHelper which uses
	// pub-sub block message to do sync.
	BeaconHelperConfig struct {
		BlockC     <-chan *types.Block
		InsertHook func()
	}
)

func (c *Config) fixValues() {
	if c.Concurrency == 0 {
		c.Concurrency = defaultConcurrency
	}
	if c.Concurrency > c.MinStreams {
		c.MinStreams = c.Concurrency
	}
	if c.MinStreams > c.InitStreams {
		c.InitStreams = c.MinStreams
	}
	if c.MinStreams > c.SmSoftLowCap {
		c.SmSoftLowCap = c.MinStreams
	}
	if c.MinStreams > c.SmHardLowCap {
		c.SmHardLowCap = c.MinStreams
	}
}
