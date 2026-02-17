package stagedstreamsync

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// BlockInfo represents cached block number with timestamp and metadata
type BlockInfo struct {
	BlockNumber uint64
	Timestamp   time.Time
	LastUsed    time.Time
	AccessCount uint64
	IsValid     bool
}

// CacheStats provides metrics about cache performance
type CacheStats struct {
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	Size        int
	MaxSize     int
	LastCleanup time.Time
}

// BlockNumberCache caches block numbers per stream to avoid redundant protocol queries.
// A cache hit occurs only when the cached block number >= the requested target,
// ensuring stale entries never prevent re-querying peers that may have advanced.
type BlockNumberCache struct {
	mu            sync.RWMutex
	cache         map[sttypes.StreamID]*BlockInfo
	protocol      ProtocolProvider
	config        *CacheConfig
	cleanupTicker *time.Ticker
	stats         CacheStats
	stopChan      chan struct{}
	stopOnce      sync.Once
}

// ProtocolProvider abstracts the sync protocol for querying remote block numbers
type ProtocolProvider interface {
	GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error)
}

// CacheConfig holds configuration for the cache
type CacheConfig struct {
	MaxSize         int           // Maximum number of entries in cache
	CleanupInterval time.Duration // How often to run cleanup
	MaxAge          time.Duration // Maximum age of cached entries
}

// DefaultCacheConfig returns a default cache configuration.
// MaxAge is long because a cache hit requires cached >= target, so stale entries
// are naturally bypassed and re-queried without relying on time-based expiration.
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         1000,
		MaxAge:          24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

// NewBlockNumberCache creates a new cache instance with configuration
func NewBlockNumberCache(protocol ProtocolProvider, config *CacheConfig) *BlockNumberCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &BlockNumberCache{
		cache:         make(map[sttypes.StreamID]*BlockInfo),
		protocol:      protocol,
		config:        config,
		cleanupTicker: time.NewTicker(config.CleanupInterval),
		stats: CacheStats{
			MaxSize: config.MaxSize,
		},
		stopChan: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.backgroundCleanup()

	return cache
}

// GetBlockNumber returns the block number for a stream.
// Returns a cached value if it is >= targetBlock, otherwise re-queries the stream.
func (c *BlockNumberCache) GetBlockNumber(ctx context.Context, streamID sttypes.StreamID, targetBlock uint64) (uint64, error) {
	c.mu.RLock()
	if info, exists := c.cache[streamID]; exists && info.IsValid {
		// Check if the cached block number is sufficient for the target
		if info.BlockNumber >= targetBlock {
			// Update access statistics
			c.mu.RUnlock()
			c.updateAccessStats(streamID)
			return info.BlockNumber, nil
		}
		// Cached block number is less than target - must re-query.
		// The peer may have advanced since we last checked.
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.stats.Misses++
	c.mu.Unlock()

	// Fetch fresh block number from protocol for the specific stream
	blockNumber, _, err := c.protocol.GetCurrentBlockNumber(ctx, syncproto.WithWhitelist([]sttypes.StreamID{streamID}))
	if err != nil {
		return 0, err
	}

	// Only cache if the block number is sufficient for the target
	if blockNumber >= targetBlock {
		c.mu.Lock()
		defer c.mu.Unlock()

		// Check if we need to evict before adding
		if len(c.cache) >= c.config.MaxSize {
			c.evictOldestEntries()
		}

		c.cache[streamID] = &BlockInfo{
			BlockNumber: blockNumber,
			Timestamp:   time.Now(),
			LastUsed:    time.Now(),
			AccessCount: 1,
			IsValid:     true,
		}
	}

	return blockNumber, nil
}

// doGetCurrentNumberRequest queries any available stream for its current block number
// and caches the result. Used by estimateCurrentNumber to poll network height.
func (c *BlockNumberCache) doGetCurrentNumberRequest(ctx context.Context) (uint64, sttypes.StreamID, error) {
	bn, stid, err := c.protocol.GetCurrentBlockNumber(ctx, syncproto.WithHighPriority())
	if err != nil {
		return 0, stid, err
	}

	// Cache the new value
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict before adding
	if len(c.cache) >= c.config.MaxSize {
		c.evictOldestEntries()
	}

	c.cache[stid] = &BlockInfo{
		BlockNumber: bn,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}

	return bn, stid, nil
}

// evictOldestEntries removes the oldest entries to make room for new ones
// Ensures at least one slot is available after eviction
func (c *BlockNumberCache) evictOldestEntries() {
	if len(c.cache) < c.config.MaxSize {
		return
	}

	// Calculate how many entries to remove (at least 1 to make room for new entry)
	entriesToRemove := len(c.cache) - c.config.MaxSize + 1

	// Create a slice of entries with their timestamps for sorting
	type entryWithTime struct {
		streamID sttypes.StreamID
		info     *BlockInfo
	}

	entries := make([]entryWithTime, 0, len(c.cache))
	for streamID, info := range c.cache {
		entries = append(entries, entryWithTime{streamID, info})
	}

	// Sort by timestamp (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].info.Timestamp.Before(entries[j].info.Timestamp)
	})

	// Remove the oldest entries
	for i := 0; i < entriesToRemove; i++ {
		delete(c.cache, entries[i].streamID)
		c.stats.Evictions++
	}
}

// backgroundCleanup runs periodic cleanup of expired entries
func (c *BlockNumberCache) backgroundCleanup() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanupExpiredEntries()
		case <-c.stopChan:
			c.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupExpiredEntries removes expired entries based on MaxAge
func (c *BlockNumberCache) cleanupExpiredEntries() {
	now := time.Now()
	expiredCount := 0

	c.mu.Lock()
	defer c.mu.Unlock()

	for streamID, info := range c.cache {
		if now.Sub(info.Timestamp) > c.config.MaxAge {
			delete(c.cache, streamID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		c.stats.Evictions += uint64(expiredCount)
	}
	c.stats.LastCleanup = time.Now()
}

// GetStats returns current cache statistics
func (c *BlockNumberCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Size = len(c.cache)
	return stats
}

// InvalidateStream marks a stream as invalid in the cache
func (c *BlockNumberCache) InvalidateStream(streamID sttypes.StreamID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if info, exists := c.cache[streamID]; exists {
		info.IsValid = false
		info.Timestamp = time.Now()
		c.cache[streamID] = info

		utils.Logger().Debug().
			Str("streamID", string(streamID)).
			Msg("Invalidated stream in block number cache")
	}
}

// RemoveStream completely removes a stream from the cache
func (c *BlockNumberCache) RemoveStream(streamID sttypes.StreamID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.cache[streamID]; exists {
		delete(c.cache, streamID)
		c.stats.Size = len(c.cache)

		utils.Logger().Debug().
			Str("streamID", string(streamID)).
			Msg("Removed stream from block number cache")
	}
}

// Reset clears all cache entries and resets statistics
func (c *BlockNumberCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldSize := len(c.cache)
	for k := range c.cache {
		delete(c.cache, k)
	}

	c.stats = CacheStats{
		MaxSize: c.config.MaxSize,
	}

	utils.Logger().Info().
		Int("cleared", oldSize).
		Msg("Block number cache reset")
}

// Stop stops the background cleanup goroutine. Safe to call multiple times.
func (c *BlockNumberCache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopChan)
	})
}

// updateAccessStats updates the access statistics for a stream
func (c *BlockNumberCache) updateAccessStats(streamID sttypes.StreamID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if info, exists := c.cache[streamID]; exists {
		info.AccessCount++
		info.LastUsed = time.Now()
		c.cache[streamID] = info
	}
	c.stats.Hits++
}
