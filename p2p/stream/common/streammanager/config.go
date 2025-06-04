package streammanager

import (
	"time"

	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
)

const (
	// checkInterval is the default interval for checking stream number. If the stream
	// number is smaller than softLoCap, an active discover through DHT will be triggered.
	checkInterval = 30 * time.Second
	// discTimeout is the timeout for one batch of discovery
	discTimeout = 10 * time.Second
	// connectTimeout is the timeout for setting up a stream with a discovered peer
	connectTimeout = 60 * time.Second
	// MaxReservedStreams is the maximum number of reserved streams
	MaxReservedStreams = 100
	// RemovalCooldownDuration defines the cooldown period (in minutes) before a removed stream can reconnect.
	RemovalCooldownDuration    = 5 * time.Minute
	MaxRemovalCooldownDuration = 60 * time.Minute

	// setupConcurrency limits concurrent stream setup goroutines
	setupConcurrency = 16
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
	// TrustedPeers are peer IDs considered trusted
	TrustedPeers map[libp2p_peer.ID]struct{}
}
