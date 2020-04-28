package worker

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(8000000000000000000)

	chainConfig  = params.TestChainConfig
	blockFactory = blockfactory.ForTest
)

func TestNewWorker(t *testing.T) {
	// Setup a new blockchain with genesis block containing test token on test address
	var (
		database = ethdb.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)

	genesis := gspec.MustCommit(database)
	_ = genesis
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, chain2.Engine, vm.Config{}, nil)

	// Create a new worker
	worker := New(params.TestChainConfig, chain, chain2.Engine)

	if worker.GetCurrentState().GetBalance(crypto.PubkeyToAddress(testBankKey.PublicKey)).Cmp(testBankFunds) != 0 {
		t.Error("Worker state is not setup correctly")
	}
}

func TestCommitTransactions(t *testing.T) {
	// Setup a new blockchain with genesis block containing test token on test address
	var (
		database = ethdb.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 0,
		}
	)

	gspec.MustCommit(database)
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, chain2.Engine, vm.Config{}, nil)

	// Create a new worker
	worker := New(params.TestChainConfig, chain, chain2.Engine)

	// Generate a test tx
	baseNonce := worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(testBankKey.PublicKey))
	randAmount := rand.Float32()
	tx, _ := types.SignTx(types.NewTransaction(baseNonce, testBankAddress, uint32(0), big.NewInt(int64(denominations.One*randAmount)), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)

	// Commit the tx to the worker
	txs := make(map[common.Address]types.Transactions)
	txs[testBankAddress] = types.Transactions{tx}
	err := worker.CommitTransactions(
		txs, nil, testBankAddress,
	)
	if err != nil {
		t.Error(err)
	}

	if len(worker.GetCurrentReceipts()) == 0 {
		t.Error("No receipt is created for new transactions")
	}

	if len(worker.current.txs) != 1 {
		t.Error("Transaction is not committed")
	}
}
