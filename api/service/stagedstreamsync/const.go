package stagedstreamsync

import (
	"time"

	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const (
	BlocksPerRequest      int = 10 // number of blocks for each request
	BlocksPerInsertion    int = 50 // number of blocks for each insert batch
	BlockHashesPerRequest int = 20 // number of get block hashes for short range sync
	BlockByHashesUpperCap int = 10 // number of get blocks by hashes upper cap
	BlockByHashesLowerCap int = 3  // number of get blocks by hashes lower cap

	LastMileBlocksThreshold int = 10

	// SoftQueueCap is the soft cap of size in resultQueue. When the queue size is larger than this limit,
	// no more request will be assigned to workers to wait for InsertChain to finish.
	SoftQueueCap int = 100

	// DefaultConcurrency is the default settings for concurrency
	DefaultConcurrency int = 4

	// ShortRangeTimeout is the timeout for each short range sync, which allow short range sync
	// to restart automatically when stuck in `getBlockHashes`
	ShortRangeTimeout time.Duration = 1 * time.Minute
)

type (
	// Config is the downloader config
	Config struct {
		// Only run stream sync protocol as a server.
		// TODO: remove this when stream sync is fully up.
		ServerOnly bool

		// parameters
		Network              nodeconfig.NetworkType
		Concurrency          int // Number of concurrent sync requests
		MinStreams           int // Minimum number of streams to do sync
		InitStreams          int // Number of streams requirement for initial bootstrap
		MaxAdvertiseWaitTime int // maximum time duration between protocol advertisements
		// stream manager config
		SmSoftLowCap int
		SmHardLowCap int
		SmHiCap      int
		SmDiscBatch  int

		// config for beacon config
		BHConfig *BeaconHelperConfig

		// log the stage progress
		LogProgress bool
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
		c.Concurrency = DefaultConcurrency
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
