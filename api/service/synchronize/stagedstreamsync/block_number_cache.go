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

// BlockNumberCache caches block numbers per stream with improved memory management
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

// ProtocolProvider defines interface to get block number
type ProtocolProvider interface {
	GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error)
}

// CacheConfig holds configuration for the cache
type CacheConfig struct {
	MaxSize           int           // Maximum number of entries in cache
	CleanupInterval   time.Duration // How often to run cleanup
	MaxAge            time.Duration // Maximum age of cached entries
	MinBlockThreshold uint64        // Minimum block number to consider for caching
}

// DefaultCacheConfig returns a default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:           1000,           // Increased from 100 to handle more peers
		MaxAge:            24 * time.Hour, // Much longer since we're syncing long ranges
		MinBlockThreshold: 1000,           // Only cache if peer has 1000+ blocks more than target
		CleanupInterval:   1 * time.Hour,  // Cleanup every hour instead of every minute
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

// GetBlockNumber retrieves the cached block number for a stream
// If not cached or expired, it fetches from the protocol provider
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
		// If cached block number is less than target, we need to query again
		// unless it's within the MinBlockThreshold
		if c.config.MinBlockThreshold > 0 && (targetBlock-info.BlockNumber) <= c.config.MinBlockThreshold {
			c.mu.RUnlock()
			c.updateAccessStats(streamID)
			return info.BlockNumber, nil
		}
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

// doGetCurrentNumberRequest returns estimated current block number and corresponding stream
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
		// Use a much longer MaxAge since we're syncing long ranges
		// For example: syncing from 0 to 1,000,000, peer has 1,200,000
		// We don't need to query again until we catch up to 1,200,000
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

// Stop stops the background cleanup goroutine
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
