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
	// RemovalCooldownDuration defines the cooldown period before a removed stream can reconnect.
	RemovalCooldownDuration = 5 * time.Minute
	// MaxRemovalCooldownDuration is the upper bound for removal cooldowns.
	// Intended to stay below stagedstreamsync.StreamDiscoveryWatchdogTimeout.
	MaxRemovalCooldownDuration = 15 * time.Minute

	// Mass-disconnect / local-outage detection parameters.
	massDisconnectWindow   = 45 * time.Second
	massDisconnectMinCount = 3
	// localOutageDuration is how long connection-loss removals use soft reconnect.
	localOutageDuration = 2 * time.Minute
	// localOutageDiscHoldoff is the delay before rediscovery after a mass disconnect.
	localOutageDiscHoldoff = 30 * time.Second
	// localOutageMinInterval is the minimum time between local-outage windows.
	localOutageMinInterval = 10 * time.Minute

	// streamRegistrationWait is the max wait for async stream registration after
	// trusted peer NewStream succeeds.
	streamRegistrationWait = 5 * time.Second
	// streamRegistrationPoll is the polling interval while waiting for registrations.
	streamRegistrationPoll = 50 * time.Millisecond

	// setupConcurrency limits concurrent stream setup goroutines
	setupConcurrency = 16
	// trustedPeersCheckInterval is the interval to check for trusted peers initialization status
	trustedPeersCheckInterval = 500 * time.Millisecond
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
	// IsTrustedPeer is a function that checks if a peer ID is trusted.
	// This allows dynamic updates when trusted peers are added after initialization.
	// If nil, no peer will be considered trusted.
	IsTrustedPeer func(libp2p_peer.ID) bool
	// GetTrustedPeers is a function that returns the list of trusted peer IDs.
	// Used for bootstrap to proactively connect to trusted peers.
	// If nil, no trusted peers will be processed during bootstrap.
	GetTrustedPeers func() []libp2p_peer.ID
	// TrustedPeersInitiated is a function that returns true if trusted peers initialization is complete.
	// The stream manager waits for this to return true before starting bootstrap discovery.
	// If nil, the stream manager will not wait for trusted peers.
	TrustedPeersInitiated func() bool
	// TrustedMinPeers is the minimum number of trusted peer streams to establish.
	// Once this number is reached, the stream manager will proceed to discover other peers.
	// If 0 or negative, all available trusted peers will be processed.
	TrustedMinPeers int
}
