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

	LastMileBlocksThreshold int    = 10
	SyncLoopBatchSize       uint32 = 30  // maximum size for one query of block hashes
	VerifyHeaderBatchSize   uint64 = 100 // block chain header verification batch size (not used for now)
	LastMileBlocksSize             = 50

	// SoftQueueCap is the soft cap of size in resultQueue. When the queue size is larger than this limit,
	// no more request will be assigned to workers to wait for InsertChain to finish.
	SoftQueueCap int = 100

	// number of get nodes by hashes for each request
	StatesPerRequest int = 100

	// maximum number of blocks for get receipts request
	ReceiptsPerRequest int = 10

	// DefaultConcurrency is the default settings for concurrency
	DefaultConcurrency int = 4

	// MaxTriesToFetchNodeData is the maximum number of tries to fetch node data
	MaxTriesToFetchNodeData int = 5

	// ShortRangeTimeout is the timeout for each short range sync, which allow short range sync
	// to restart automatically when stuck in `getBlockHashes`
	ShortRangeTimeout time.Duration = 1 * time.Minute

	// pivot block distance ranges
	MinPivotDistanceToHead uint64 = 1024
	MaxPivotDistanceToHead uint64 = 2048
)

// SyncMode represents the synchronization mode of the downloader.
// It is a uint32 as it is used with atomic operations.
type SyncMode uint32

const (
	FullSync SyncMode = iota // Synchronize the entire blockchain history from full blocks
	FastSync                 // Download all blocks and states
	SnapSync                 // Download the chain and the state via compact snapshots
)

type (
	// Config is the downloader config
	Config struct {
		// Only run stream sync protocol as a server.
		// TODO: remove this when stream sync is fully up.
		ServerOnly bool

		// Synchronization mode of the downloader
		SyncMode SyncMode

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

		// use memory db
		UseMemDB bool

		// log the stage progress
		LogProgress bool

		// logs every single process and error to help debugging stream sync
		// DebugMode is not accessible to the end user and is only an aid for development
		DebugMode bool
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
		c.Concurrency = c.MinStreams
	}
	if c.Concurrency > c.MinStreams {
		c.Concurrency = c.MinStreams
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
