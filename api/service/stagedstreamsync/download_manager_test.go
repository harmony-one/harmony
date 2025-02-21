package stagedstreamsync

import (
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
