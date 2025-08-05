package stagedstreamsync

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/vm"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

func createTestBlockChain() *core.BlockChainImpl {
	key, _ := crypto.GenerateKey()
	gspec := core.Genesis{
		Config:  params.TestChainConfig,
		Factory: blockfactory.ForTest,
		Alloc: core.GenesisAlloc{
			crypto.PubkeyToAddress(key.PublicKey): {
				Balance: big.NewInt(8e18),
			},
		},
		GasLimit: 1e18,
		ShardID:  0,
	}
	database := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(database)
	_ = genesis
	engine := chain2.NewEngine()
	cacheConfig := &core.CacheConfig{SnapshotLimit: 0}
	blockchain, _ := core.NewBlockChain(database, nil, nil, cacheConfig, gspec.Config, engine, vm.Config{})
	return blockchain
}

func TestNewDownloadManager(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()

	dm := newDownloadManager(chain, 1, 1000, 10, logger)

	assert.NotNil(t, dm)
	assert.Equal(t, uint64(1000), dm.targetBN)
	assert.Equal(t, 10, dm.batchSize)
	assert.NotNil(t, dm.requesting)
	assert.NotNil(t, dm.processing)
	assert.NotNil(t, dm.details)
	assert.NotNil(t, dm.retries)
	assert.NotNil(t, dm.rq)
}

func makeTestPrioritizedNumbers(bns []uint64) *prioritizedNumbers {
	pn := newPrioritizedNumbers()
	for _, bn := range bns {
		pn.push(bn)
	}
	return pn
}

func TestGetNextBatch(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()
	retries := makeTestPrioritizedNumbers([]uint64{5, 6})
	rq := makeTestResultQueue([]uint64{5, 6})

	// Simulate current height
	// curHeight is 3
	dm := newDownloadManager(chain, 3, 1000, 3, logger)
	dm.retries = retries
	dm.rq = rq

	// Call GetNextBatch
	batch := dm.GetNextBatch()

	// Verify batch
	expectedBatch := []uint64{5, 6, 4}
	assert.Equal(t, expectedBatch, batch)
	assert.Contains(t, dm.requesting, uint64(5))
	assert.Contains(t, dm.requesting, uint64(6))
	assert.Contains(t, dm.requesting, uint64(4))
}

func TestHandleRequestError(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()
	dm := newDownloadManager(chain, 1, 1000, 3, logger)
	dm.retries = makeTestPrioritizedNumbers([]uint64{})

	dm.requesting[5] = struct{}{}
	dm.requesting[6] = struct{}{}

	dm.HandleRequestError([]uint64{5, 6}, nil, sttypes.StreamID("test"))

	assert.NotContains(t, dm.requesting, uint64(5))
	assert.NotContains(t, dm.requesting, uint64(6))
	assert.Equal(t, true, dm.retries.contains(uint64(5)))
	assert.Equal(t, true, dm.retries.contains(uint64(6)))
}

func TestHandleRequestResult(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()
	dm := newDownloadManager(chain, 1, 1000, 3, logger)

	dm.requesting[5] = struct{}{}
	dm.requesting[6] = struct{}{}

	blockBytes := [][]byte{{1, 2, 3}, {0}}
	sigBytes := [][]byte{{4, 5, 6}, {7, 8, 9}}

	err := dm.HandleRequestResult([]uint64{5, 6}, blockBytes, sigBytes, 1, sttypes.StreamID("test"))

	assert.NoError(t, err)
	assert.NotContains(t, dm.requesting, uint64(5))
	assert.NotContains(t, dm.requesting, uint64(6))
	assert.Contains(t, dm.processing, uint64(5))
	assert.Equal(t, true, dm.retries.contains(6))
}

func TestSetDownloadDetailsAndGetDownloadDetails(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()
	dm := newDownloadManager(chain, 1, 1000, 3, logger)

	dm.SetDownloadDetails([]uint64{10}, 1, sttypes.StreamID("stream1"))

	workerID, streamID, err := dm.GetDownloadDetails(10)
	assert.NoError(t, err)
	assert.Equal(t, 1, workerID)
	assert.Equal(t, sttypes.StreamID("stream1"), streamID)
}

func TestSetRootHashAndGetRootHash(t *testing.T) {
	logger := zerolog.Nop()
	chain := createTestBlockChain()
	dm := newDownloadManager(chain, 1, 1000, 3, logger)

	hash := common.HexToHash("0x12345")
	dm.details[10] = &DownloadDetails{}
	dm.SetRootHash(10, hash)

	retrievedHash := dm.GetRootHash(10)
	assert.Equal(t, hash, retrievedHash)
}

func TestDownloadManagerCleanup(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Add some download details
	dm.SetDownloadDetails([]uint64{1, 2, 3}, 1, "stream1")
	dm.SetDownloadDetails([]uint64{4, 5}, 2, "stream2")

	// Verify details were added
	if dm.GetDetailsSize() != 5 {
		t.Errorf("Expected 5 details, got %d", dm.GetDetailsSize())
	}

	// Clean up specific block
	dm.CleanupDetails(1)
	if dm.GetDetailsSize() != 4 {
		t.Errorf("Expected 4 details after cleanup, got %d", dm.GetDetailsSize())
	}

	// Clean up all details
	dm.CleanupAllDetails()
	if dm.GetDetailsSize() != 0 {
		t.Errorf("Expected 0 details after full cleanup, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerRemoveFromProcessing(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Add blocks to processing with valid block data (length > 1)
	dm.HandleRequestResult([]uint64{1, 2, 3}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, 1, "stream1")

	// Verify blocks are in processing
	if len(dm.processing) != 3 {
		t.Errorf("Expected 3 blocks in processing, got %d", len(dm.processing))
	}

	// HandleRequestResult should add details for successfully processed blocks
	if dm.GetDetailsSize() != 3 {
		t.Errorf("Expected 3 details after HandleRequestResult, got %d", dm.GetDetailsSize())
	}

	// Simulate what happens in the final stage (Finish) - clean up all details
	dm.CleanupAllDetails()

	// Verify all details were cleaned up
	if dm.GetDetailsSize() != 0 {
		t.Errorf("Expected 0 details after final cleanup, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerManualCleanup(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Add some download details manually
	dm.SetDownloadDetails([]uint64{1, 2, 3}, 1, "stream1")

	// Verify details were added
	if dm.GetDetailsSize() != 3 {
		t.Errorf("Expected 3 details, got %d", dm.GetDetailsSize())
	}

	// Test manual cleanup methods
	dm.CleanupDetails(1)
	if dm.GetDetailsSize() != 2 {
		t.Errorf("Expected 2 details after cleanup, got %d", dm.GetDetailsSize())
	}

	dm.CleanupAllDetails()
	if dm.GetDetailsSize() != 0 {
		t.Errorf("Expected 0 details after full cleanup, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerCleanupCompletedBlocks(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Add some blocks to processing and details with valid block data
	dm.HandleRequestResult([]uint64{1, 2, 3}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, 1, "stream1")

	// Verify we have 3 details initially
	if dm.GetDetailsSize() != 3 {
		t.Errorf("Expected 3 details initially, got %d", dm.GetDetailsSize())
	}

	// Mark some blocks as completed (this removes them from both processing and details)
	dm.MarkBlockCompleted(1)
	dm.MarkBlockCompleted(2)

	// After MarkBlockCompleted, we should have 1 detail left (for block 3)
	if dm.GetDetailsSize() != 1 {
		t.Errorf("Expected 1 detail after MarkBlockCompleted, got %d", dm.GetDetailsSize())
	}

	// Clean up completed blocks (this won't do anything since MarkBlockCompleted already cleaned them)
	dm.CleanupCompletedBlocks()

	// Should still have 1 detail left (for block 3 which is still in processing)
	if dm.GetDetailsSize() != 1 {
		t.Errorf("Expected 1 detail after cleanup, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerCornerCases(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Test 1: Multiple additions and removals
	dm.SetDownloadDetails([]uint64{1, 2, 3}, 1, "stream1")
	dm.SetDownloadDetails([]uint64{1, 2, 3}, 2, "stream2") // Should replace existing
	if dm.GetDetailsSize() != 3 {
		t.Errorf("Expected 3 details after replacement, got %d", dm.GetDetailsSize())
	}

	// Test 2: HandleRequestResult with mixed valid/invalid blocks
	// Block 4: invalid (empty), Block 5: valid (length > 1), Block 6: invalid (empty)
	dm.HandleRequestResult([]uint64{4, 5, 6}, [][]byte{{}, {1, 2}, {}}, [][]byte{{}, {1, 2}, {}}, 1, "stream1")
	// Only block 5 should be in processing and have details (block 4 and 6 go to retries)
	if dm.GetDetailsSize() != 4 { // 3 from SetDownloadDetails + 1 from HandleRequestResult (only block 5)
		t.Errorf("Expected 4 details after HandleRequestResult, got %d", dm.GetDetailsSize())
	}

	// Test 3: CleanupCompletedBlocks should only clean up non-processing blocks
	// Blocks 1, 2, 3 from SetDownloadDetails are not in processing, so they get cleaned up
	// Block 5 from HandleRequestResult is in processing, so it stays
	dm.CleanupCompletedBlocks()
	// Should have 1 detail left (only block 5 which is in processing)
	if dm.GetDetailsSize() != 1 {
		t.Errorf("Expected 1 detail after CleanupCompletedBlocks, got %d", dm.GetDetailsSize())
	}

	// Test 4: MarkBlockCompleted (removes from both processing and details)
	dm.MarkBlockCompleted(5)
	if dm.GetDetailsSize() != 0 { // Block 5 removed, no details left
		t.Errorf("Expected 0 details after MarkBlockCompleted, got %d", dm.GetDetailsSize())
	}

	// Test 5: CleanupAllDetails
	dm.CleanupAllDetails()
	if dm.GetDetailsSize() != 0 {
		t.Errorf("Expected 0 details after CleanupAllDetails, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerMemoryLeakScenarios(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Scenario 1: Large number of blocks
	for i := uint64(1); i <= 1000; i++ {
		dm.SetDownloadDetails([]uint64{i}, int(i%10), sttypes.StreamID(fmt.Sprintf("stream%d", i)))
	}

	if dm.GetDetailsSize() != 1000 {
		t.Errorf("Expected 1000 details, got %d", dm.GetDetailsSize())
	}

	// Cleanup should work for large numbers
	dm.CleanupAllDetails()
	if dm.GetDetailsSize() != 0 {
		t.Errorf("Expected 0 details after cleanup, got %d", dm.GetDetailsSize())
	}

	// Scenario 2: Mixed processing states
	dm.HandleRequestResult([]uint64{1, 2, 3}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, 1, "stream1")
	dm.HandleRequestResult([]uint64{4, 5, 6}, [][]byte{{}, {1, 2}, {}}, [][]byte{{}, {1, 2}, {}}, 1, "stream1")

	// Mark some as completed (this removes them from both processing and details)
	dm.MarkBlockCompleted(1)
	dm.MarkBlockCompleted(5)

	// CleanupCompletedBlocks should only clean up completed blocks
	dm.CleanupCompletedBlocks()
	// Should have details for blocks still in processing (2, 3, 6) - block 4 went to retries
	if dm.GetDetailsSize() != 2 { // Blocks 1 and 5 were removed by MarkBlockCompleted, blocks 2, 3, 6 remain
		t.Errorf("Expected 2 details after mixed cleanup, got %d", dm.GetDetailsSize())
	}
}

func TestDownloadManagerCleanupCompletedBlocksManual(t *testing.T) {
	dm := newDownloadManager(nil, 0, 100, 10, zerolog.Nop())

	// Add some blocks to processing and details
	dm.HandleRequestResult([]uint64{1, 2, 3}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, [][]byte{{1, 2}, {1, 2}, {1, 2}}, 1, "stream1")

	// Verify we have 3 details initially
	if dm.GetDetailsSize() != 3 {
		t.Errorf("Expected 3 details initially, got %d", dm.GetDetailsSize())
	}

	// Manually remove block 1 from processing but not from details
	dm.lock.Lock()
	delete(dm.processing, 1)
	dm.lock.Unlock()

	// Clean up completed blocks (should remove block 1's details since it's no longer in processing)
	dm.CleanupCompletedBlocks()

	// Should have 2 details left (for blocks 2 and 3 which are still in processing)
	if dm.GetDetailsSize() != 2 {
		t.Errorf("Expected 2 details after cleanup, got %d", dm.GetDetailsSize())
	}
}
