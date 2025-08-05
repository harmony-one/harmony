package stagedstreamsync

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/stretchr/testify/assert"
)

func TestGetBlocksByHashManagerMemoryLeakPrevention(t *testing.T) {
	// Create test hashes and whitelist
	hashes := []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}
	whitelist := []sttypes.StreamID{"stream1", "stream2"}

	m := newGetBlocksByHashManager(hashes, whitelist)

	// Test initial state
	pendings, results, whitelistSize := m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 0, results)
	assert.Equal(t, 2, whitelistSize)

	// Add some results
	blocks := []*types.Block{
		makeTestBlock(1),
		makeTestBlock(2),
	}
	m.addResult(hashes[:2], blocks, "stream1")

	// Verify results were added
	pendings, results, _ = m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 2, results)

	// Test cleanup stale results
	removed := m.CleanupStaleResults()
	assert.Equal(t, 0, removed) // No stale results yet

	// Test cleanup pendings
	removed = m.CleanupPendings()
	assert.Equal(t, 0, removed) // No stale pendings yet

	// Test clear all data
	m.ClearAllData()
	pendings, results, whitelistSize = m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 0, results)
	assert.Equal(t, 0, whitelistSize)
}

func TestGetBlocksByHashManagerStaleDataCleanup(t *testing.T) {
	// Create test hashes and whitelist
	hashes := []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}
	whitelist := []sttypes.StreamID{"stream1", "stream2"}

	m := newGetBlocksByHashManager(hashes, whitelist)

	// Add some results and pendings
	blocks := []*types.Block{
		makeTestBlock(1),
		makeTestBlock(2),
	}
	m.addResult(hashes[:2], blocks, "stream1")

	// Manually add some stale data (simulating leftover data)
	m.results[common.HexToHash("0x999")] = blockResult{
		block: makeTestBlock(999),
		stid:  "stream1",
	}
	m.pendings[common.HexToHash("0x888")] = struct{}{}

	// Verify stale data exists
	pendings, results, _ := m.GetDataSize()
	assert.Equal(t, 1, pendings)
	assert.Equal(t, 3, results) // 2 valid + 1 stale

	// Clean up stale data
	removed := m.CleanupStaleResults()
	assert.Equal(t, 1, removed) // 1 stale result removed

	removed = m.CleanupPendings()
	assert.Equal(t, 1, removed) // 1 stale pending removed

	// Verify cleanup worked
	pendings, results, _ = m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 2, results) // Only valid results remain
}

func TestGetBlocksByHashManagerConcurrentCleanup(t *testing.T) {
	// Create test hashes and whitelist
	hashes := []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}
	whitelist := []sttypes.StreamID{"stream1", "stream2"}

	m := newGetBlocksByHashManager(hashes, whitelist)

	// Add some data
	blocks := []*types.Block{
		makeTestBlock(1),
		makeTestBlock(2),
	}
	m.addResult(hashes[:2], blocks, "stream1")

	// Test concurrent cleanup operations
	done := make(chan bool, 2)

	go func() {
		m.CleanupStaleResults()
		done <- true
	}()

	go func() {
		m.CleanupPendings()
		done <- true
	}()

	// Wait for both operations to complete
	<-done
	<-done

	// Verify no data corruption
	pendings, results, whitelistSize := m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 2, results)
	assert.Equal(t, 2, whitelistSize)
}

func TestGetBlocksByHashManagerClearAllData(t *testing.T) {
	// Create test hashes and whitelist
	hashes := []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}
	whitelist := []sttypes.StreamID{"stream1", "stream2"}

	m := newGetBlocksByHashManager(hashes, whitelist)

	// Add some data
	blocks := []*types.Block{
		makeTestBlock(1),
		makeTestBlock(2),
	}
	m.addResult(hashes[:2], blocks, "stream1")

	// Verify data exists
	pendings, results, whitelistSize := m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 2, results)
	assert.Equal(t, 2, whitelistSize)

	// Clear all data
	m.ClearAllData()

	// Verify all data is cleared
	pendings, results, whitelistSize = m.GetDataSize()
	assert.Equal(t, 0, pendings)
	assert.Equal(t, 0, results)
	assert.Equal(t, 0, whitelistSize)
}
