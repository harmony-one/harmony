package stagedstreamsync

import (
	"context"
	"sync"
	"time"

	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// BlockInfo represents cached block number with timestamp
type BlockInfo struct {
	BlockNumber uint64
	Timestamp   time.Time
}

// BlockNumberCache caches block numbers per stream
type BlockNumberCache struct {
	mu       sync.RWMutex
	cache    map[sttypes.StreamID]BlockInfo
	protocol ProtocolProvider // should match your sss.protocol interface
}

// ProtocolProvider defines interface to get block number
type ProtocolProvider interface {
	GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error)
}

// NewBlockNumberCache creates a new cache instance
func NewBlockNumberCache(protocol ProtocolProvider) *BlockNumberCache {
	return &BlockNumberCache{
		cache:    make(map[sttypes.StreamID]BlockInfo),
		protocol: protocol,
	}
}

// GetBlockNumber returns block number for a stream with optional caching rules
// If `minBlock` is 0, it is ignored
// If `maxAge` is 0, it is ignored
func (b *BlockNumberCache) GetBlockNumber(
	ctx context.Context,
	streamID sttypes.StreamID,
	minBlock uint64,
	maxAge time.Duration,
) (uint64, error) {
	now := time.Now()

	b.mu.RLock()
	info, exists := b.cache[streamID]
	b.mu.RUnlock()

	// Check if cache can be used
	useCache := exists &&
		(minBlock == 0 || info.BlockNumber >= minBlock) &&
		(maxAge == 0 || now.Sub(info.Timestamp) <= maxAge)

	if useCache {
		return info.BlockNumber, nil
	}

	// Fetch fresh block number
	blockNum, stid, err := b.protocol.GetCurrentBlockNumber(ctx, syncproto.WithWhitelist([]sttypes.StreamID{streamID}))
	if err != nil {
		return 0, err
	}

	// Cache the new value
	b.mu.Lock()
	b.cache[stid] = BlockInfo{
		BlockNumber: blockNum,
		Timestamp:   now,
	}
	b.mu.Unlock()

	return blockNum, nil
}

// doGetCurrentNumberRequest returns estimated current block number and corresponding stream
func (b *BlockNumberCache) doGetCurrentNumberRequest(ctx context.Context) (uint64, sttypes.StreamID, error) {
	bn, stid, err := b.protocol.GetCurrentBlockNumber(ctx, syncproto.WithHighPriority())
	if err != nil {
		return 0, stid, err
	}
	// Cache the new value
	b.mu.Lock()
	defer b.mu.Unlock()

	b.cache[stid] = BlockInfo{
		BlockNumber: bn,
		Timestamp:   time.Now(),
	}

	return bn, stid, nil
}
