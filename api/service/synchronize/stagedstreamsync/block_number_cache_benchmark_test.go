package stagedstreamsync

import (
	"context"
	"fmt"
	"testing"
	"time"

	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// BenchmarkProtocolProvider implements ProtocolProvider for benchmarking
type BenchmarkProtocolProvider struct {
	cache map[sttypes.StreamID]uint64
}

func NewBenchmarkProtocolProvider() *BenchmarkProtocolProvider {
	cache := make(map[sttypes.StreamID]uint64)
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		streamID := sttypes.StreamID(fmt.Sprintf("stream-%d", i))
		cache[streamID] = uint64(10000 + i)
	}
	return &BenchmarkProtocolProvider{cache: cache}
}

func (b *BenchmarkProtocolProvider) GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error) {
	// Simulate network latency
	time.Sleep(1 * time.Millisecond)

	// Return a random stream ID and block number for benchmarking
	for streamID, blockNumber := range b.cache {
		return blockNumber, streamID, nil
	}
	return 0, "", fmt.Errorf("no streams available")
}

func BenchmarkGetBlockNumber_CacheHit(b *testing.B) {
	provider := NewBenchmarkProtocolProvider()
	cache := NewBlockNumberCache(provider, nil)
	defer cache.Stop()

	// Pre-populate cache
	streamID := sttypes.StreamID("benchmark-stream")
	targetBlock := uint64(5000)

	cache.mu.Lock()
	cache.cache[streamID] = &BlockInfo{
		BlockNumber: 10000,
		Timestamp:   time.Now(),
		LastUsed:    time.Now(),
		AccessCount: 1,
		IsValid:     true,
	}
	cache.mu.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGetBlockNumber_CacheMiss(b *testing.B) {
	provider := NewBenchmarkProtocolProvider()
	cache := NewBlockNumberCache(provider, nil)
	defer cache.Stop()

	streamID := sttypes.StreamID("benchmark-stream")
	targetBlock := uint64(5000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGetBlockNumber_Mixed(b *testing.B) {
	provider := NewBenchmarkProtocolProvider()
	cache := NewBlockNumberCache(provider, nil)
	defer cache.Stop()

	// Pre-populate cache with some entries
	for i := 0; i < 100; i++ {
		streamID := sttypes.StreamID(fmt.Sprintf("stream-%d", i))
		cache.mu.Lock()
		cache.cache[streamID] = &BlockInfo{
			BlockNumber: uint64(10000 + i),
			Timestamp:   time.Now(),
			LastUsed:    time.Now(),
			AccessCount: 1,
			IsValid:     true,
		}
		cache.mu.Unlock()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			streamID := sttypes.StreamID(fmt.Sprintf("stream-%d", i%150)) // Some hits, some misses
			targetBlock := uint64(5000 + i%1000)

			_, err := cache.GetBlockNumber(context.Background(), streamID, targetBlock)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkEvictOldestEntries(b *testing.B) {
	provider := NewBenchmarkProtocolProvider()
	config := &CacheConfig{
		MaxSize:         100,
		MaxAge:          1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	}
	cache := NewBlockNumberCache(provider, config)
	defer cache.Stop()

	// Pre-populate cache to trigger eviction
	for i := 0; i < 150; i++ {
		streamID := sttypes.StreamID(fmt.Sprintf("stream-%d", i))
		cache.mu.Lock()
		cache.cache[streamID] = &BlockInfo{
			BlockNumber: uint64(10000 + i),
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Minute),
			LastUsed:    time.Now(),
			AccessCount: 1,
			IsValid:     true,
		}
		cache.mu.Unlock()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.evictOldestEntries()
	}
}

func BenchmarkCleanupExpiredEntries(b *testing.B) {
	provider := NewBenchmarkProtocolProvider()
	config := &CacheConfig{
		MaxSize:         1000,
		MaxAge:          1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	}
	cache := NewBlockNumberCache(provider, config)
	defer cache.Stop()

	// Pre-populate cache with mixed ages
	now := time.Now()
	for i := 0; i < 1000; i++ {
		streamID := sttypes.StreamID(fmt.Sprintf("stream-%d", i))
		age := time.Duration(i%3) * 2 * time.Hour // 0, 2, or 4 hours
		cache.mu.Lock()
		cache.cache[streamID] = &BlockInfo{
			BlockNumber: uint64(10000 + i),
			Timestamp:   now.Add(-age),
			LastUsed:    now,
			AccessCount: 1,
			IsValid:     true,
		}
		cache.mu.Unlock()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.cleanupExpiredEntries()
	}
}
