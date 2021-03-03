package streammanager

import "time"

const (
	// checkInterval is the default interval for checking stream number. If the stream
	// number is smaller than softLoCap, an active discover through DHT will be triggered.
	checkInterval = 30 * time.Second
	// discTimeout is the timeout for one batch of discovery
	discTimeout = 10 * time.Second
	// connectTimeout is the timeout for setting up a stream with a discovered peer
	connectTimeout = 60 * time.Second
)

// Config is the config for stream manager
type Config struct {
	// HardLoCap is low cap of stream number that immediately trigger discovery
	HardLoCap int
	// SoftLoCap is low cap of stream number that will trigger discovery during stream check
	SoftLoCap int
	// HiCap is the high cap of stream number
	HiCap int
	// DiscBatch is the size of each discovery
	DiscBatch int
}
