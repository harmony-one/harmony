package stagedstreamsync

import (
	"context"
	"fmt"
	"testing"
	"time"

	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProtocolProvider implements ProtocolProvider for testing
type MockProtocolProvider struct {
	mock.Mock
}

func (m *MockProtocolProvider) GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(uint64), args.Get(1).(sttypes.StreamID), func() error {
		if err := args.Get(2); err != nil {
			return err.(error)
		}
		return nil
	}()
}

func TestNewBlockNumberCache(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}

	// Test with default config
	cache := NewBlockNumberCache(mockProtocol, nil)
	require.NotNil(t, cache)
	assert.Equal(t, 1000, cache.config.MaxSize)
	assert.Equal(t, 24*time.Hour, cache.config.MaxAge)
	assert.Equal(t, uint64(1000), cache.config.MinBlockThreshold)

	// Test with custom config
	customConfig := &CacheConfig{
		MaxSize:           500,
		MaxAge:            12 * time.Hour,
		MinBlockThreshold: 500,
		CleanupInterval:   30 * time.Minute,
	}

	cache2 := NewBlockNumberCache(mockProtocol, customConfig)
	require.NotNil(t, cache2)
	assert.Equal(t, 500, cache2.config.MaxSize)
	assert.Equal(t, 12*time.Hour, cache2.config.MaxAge)
	assert.Equal(t, uint64(500), cache2.config.MinBlockThreshold)

	// Cleanup
	cache.Stop()
	cache2.Stop()
}

func TestGetBlockNumber_CacheHit(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")
	targetBlock := uint64(1000)

	// Pre-populate cache with sufficient block number
	cache.mu.Lock()
	cache.cache[streamID] = &BlockInfo{
		BlockNumber: 1200,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Test cache hit
	blockNumber, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
	require.NoError(t, err)
	assert.Equal(t, uint64(1200), blockNumber)

	// Verify access stats were updated
	cache.mu.RLock()
	info := cache.cache[streamID]
	cache.mu.RUnlock()
	assert.Equal(t, uint64(2), info.AccessCount)
}

func TestGetBlockNumber_CacheHitWithMinBlockThreshold(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")
	targetBlock := uint64(1000)

	// Pre-populate cache with block number within MinBlockThreshold
	cache.mu.Lock()
	cache.cache[streamID] = &BlockInfo{
		BlockNumber: 1050, // 50 blocks within threshold of 1000
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Test cache hit due to MinBlockThreshold
	blockNumber, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
	require.NoError(t, err)
	assert.Equal(t, uint64(1050), blockNumber)
}

func TestGetBlockNumber_CacheMiss(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")
	targetBlock := uint64(1000)

	// Mock protocol response
	mockProtocol.On("GetCurrentBlockNumber", mock.Anything, mock.Anything).Return(uint64(1200), streamID, nil)

	// Test cache miss - should fetch from protocol
	blockNumber, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
	require.NoError(t, err)
	assert.Equal(t, uint64(1200), blockNumber)

	// Verify it was cached
	cache.mu.RLock()
	info, exists := cache.cache[streamID]
	cache.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, uint64(1200), info.BlockNumber)
	assert.True(t, info.IsValid)

	mockProtocol.AssertExpectations(t)
}

func TestGetBlockNumber_CacheMissInsufficientBlocks(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")
	targetBlock := uint64(1000)

	// Mock protocol response with insufficient blocks
	mockProtocol.On("GetCurrentBlockNumber", mock.Anything, mock.Anything).Return(uint64(800), streamID, nil)

	// Test cache miss - should fetch from protocol but not cache insufficient blocks
	blockNumber, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
	require.NoError(t, err)
	assert.Equal(t, uint64(800), blockNumber)

	// Verify it was NOT cached (insufficient blocks)
	cache.mu.RLock()
	_, exists := cache.cache[streamID]
	cache.mu.RUnlock()
	assert.False(t, exists)

	mockProtocol.AssertExpectations(t)
}

func TestEvictOldestEntries(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	config := &CacheConfig{
		MaxSize:           3,
		MaxAge:            1 * time.Hour,
		MinBlockThreshold: 100,
		CleanupInterval:   1 * time.Minute,
	}
	cache := NewBlockNumberCache(mockProtocol, config)
	defer cache.Stop()

	// Add 4 entries (exceeds MaxSize of 3)
	for i := 0; i < 4; i++ {
		streamID := sttypes.StreamID(fmt.Sprintf("stream%d", i+1))
		cache.mu.Lock()
		cache.cache[streamID] = &BlockInfo{
			BlockNumber: uint64(1000 + i),
			Timestamp:   time.Now().Add(-time.Duration(3-i) * time.Minute), // stream1 is oldest
			LastUsed:    time.Now(),
			AccessCount: 1,
			IsValid:     true,
		}
		cache.mu.Unlock()
	}

	// Verify we have 4 entries
	assert.Equal(t, 4, len(cache.cache))

	// Add one more entry to trigger eviction
	cache.mu.Lock()
	cache.cache["stream5"] = &BlockInfo{
		BlockNumber: 2000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Trigger eviction
	cache.mu.Lock()
	cache.evictOldestEntries()
	cache.mu.Unlock()

	// Should have evicted exactly 2 entries (4+1-3=2) to get back to MaxSize
	assert.Equal(t, 3, len(cache.cache))

	// Verify oldest entries were removed
	cache.mu.RLock()
	_, exists1 := cache.cache["stream1"] // oldest
	_, exists2 := cache.cache["stream2"] // second oldest
	_, exists3 := cache.cache["stream3"] // should remain
	_, exists4 := cache.cache["stream4"] // should remain
	_, exists5 := cache.cache["stream5"] // newest, should remain
	cache.mu.RUnlock()

	assert.False(t, exists1) // oldest removed
	assert.False(t, exists2) // second oldest removed
	assert.True(t, exists3)  // kept
	assert.True(t, exists4)  // kept
	assert.True(t, exists5)  // kept
}

func TestCleanupExpiredEntries(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	config := &CacheConfig{
		MaxSize:           100,
		MaxAge:            1 * time.Hour,
		MinBlockThreshold: 100,
		CleanupInterval:   1 * time.Minute,
	}
	cache := NewBlockNumberCache(mockProtocol, config)
	defer cache.Stop()

	// Add entries with different ages
	now := time.Now()

	cache.mu.Lock()
	cache.cache["recent"] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   now,
		LastUsed:    now,
		AccessCount: 1,
		IsValid:     true,
	}
	cache.cache["old"] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   now.Add(-2 * time.Hour), // older than MaxAge
		LastUsed:    now,
		AccessCount: 1,
		IsValid:     true,
	}
	cache.cache["very-old"] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   now.Add(-3 * time.Hour), // older than MaxAge
		LastUsed:    now,
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Verify we have 3 entries
	assert.Equal(t, 3, len(cache.cache))

	// Trigger cleanup
	cache.cleanupExpiredEntries()

	// Should have removed 2 expired entries
	assert.Equal(t, 1, len(cache.cache))

	// Verify only recent entry remains
	cache.mu.RLock()
	_, existsRecent := cache.cache["recent"]
	_, existsOld := cache.cache["old"]
	_, existsVeryOld := cache.cache["very-old"]
	cache.mu.RUnlock()

	assert.True(t, existsRecent)
	assert.False(t, existsOld)
	assert.False(t, existsVeryOld)
}

func TestCacheStats(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	// Get initial stats
	stats := cache.GetStats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 1000, stats.MaxSize)
	assert.Equal(t, uint64(0), stats.Hits)
	assert.Equal(t, uint64(0), stats.Misses)
	assert.Equal(t, uint64(0), stats.Evictions)

	// Add some entries
	cache.mu.Lock()
	cache.cache["stream1"] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Get updated stats
	stats = cache.GetStats()
	assert.Equal(t, 1, stats.Size)
}

func TestInvalidateStream(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")

	// Add valid entry
	cache.mu.Lock()
	cache.cache[streamID] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Invalidate stream
	cache.InvalidateStream(streamID)

	// Verify it's marked as invalid
	cache.mu.RLock()
	info := cache.cache[streamID]
	cache.mu.RUnlock()
	assert.False(t, info.IsValid)
}

func TestRemoveStream(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("test-stream")

	// Add entry
	cache.mu.Lock()
	cache.cache[streamID] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Verify it exists
	assert.Equal(t, 1, len(cache.cache))

	// Remove stream
	cache.RemoveStream(streamID)

	// Verify it's gone
	assert.Equal(t, 0, len(cache.cache))
}

func TestReset(t *testing.T) {
	mockProtocol := &MockProtocolProvider{}
	cache := NewBlockNumberCache(mockProtocol, nil)
	defer cache.Stop()

	// Add some entries
	cache.mu.Lock()
	cache.cache["stream1"] = &BlockInfo{
		BlockNumber: 1000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.cache["stream2"] = &BlockInfo{
		BlockNumber: 2000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	// Verify we have entries
	assert.Equal(t, 2, len(cache.cache))

	// Reset cache
	cache.Reset()

	// Verify cache is empty
	assert.Equal(t, 0, len(cache.cache))

	// Verify stats are reset
	stats := cache.GetStats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, uint64(0), stats.Hits)
	assert.Equal(t, uint64(0), stats.Misses)
}
