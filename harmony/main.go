package main

import (
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/node/worker"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(1000000000000000000)

	testUserKey, _  = crypto.GenerateKey()
	testUserAddress = crypto.PubkeyToAddress(testUserKey.PublicKey)

	chainConfig = params.TestChainConfig

	// Test transactions
	pendingTxs []*types.Transaction
	newTxs     []*types.Transaction
)

func init() {
	tx1, _ := types.SignTx(types.NewTransaction(0, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	tx2, _ := types.SignTx(types.NewTransaction(1, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	tx3, _ := types.SignTx(types.NewTransaction(2, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	pendingTxs = append(pendingTxs, tx1)
	pendingTxs = append(pendingTxs, tx2)
	pendingTxs = append(pendingTxs, tx3)
	tx4, _ := types.SignTx(types.NewTransaction(1, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	newTxs = append(newTxs, tx4)
}

type testWorkerBackend struct {
	db     db.Database
	txPool *core.TxPool
	chain  *core.BlockChain
}

func main() {
	var (
		database = db.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)

	genesis := gspec.MustCommit(database)
	_ = genesis
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, consensus.NewFaker(), vm.Config{}, nil)

	txpool := core.NewTxPool(core.DefaultTxPoolConfig, chainConfig, chain)

	backend := &testWorkerBackend{
		db:     database,
		chain:  chain,
		txPool: txpool,
	}
	backend.txPool.AddLocals(pendingTxs)

	//// Generate a small n-block chain and an uncle block for it
	n := 3
	if n > 0 {
		blocks, _ := core.GenerateChain(chainConfig, genesis, consensus.NewFaker(), database, n, func(i int, gen *core.BlockGen) {
			gen.SetCoinbase(testBankAddress)
			gen.SetShardID(types.EncodeShardID(10))
			gen.AddTx(pendingTxs[i])
		})
		if _, err := chain.InsertChain(blocks); err != nil {
			log.Fatal(err)
		}
	}

	txs := make([]*types.Transaction, 100)
	worker := worker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(testBankKey.PublicKey))
	nonce := worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(testBankKey.PublicKey))
	for i := range txs {
		randomUserKey, _ := crypto.GenerateKey()
		randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i), randomUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
		txs[i] = tx
	}

	worker.CommitTransactions(txs)
}
