// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harmony-one/harmony/core/rawdb"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	// testTxPoolConfig is a transaction pool configuration without stateful disk sideeffects used during testing.
	testTxPoolConfig TxPoolConfig
	testBLSPubKey    = "30b2c38b1316da91e068ac3bd8751c0901ef6c02a1d58bc712104918302c6ed03d5894671d0c816dad2b4d303320f202"
	testBLSPrvKey    = "c6d7603520311f7a4e6aac0b26701fc433b75b38df504cd416ef2b900cd66205"
	gasPrice         = big.NewInt(100e9)
	gasLimit         = big.NewInt(int64(params.TxGasValidatorCreation))
	cost             = big.NewInt(1).Mul(gasPrice, gasLimit)
	dummyErrorSink   = types.NewTransactionErrorSink()
)

func init() {
	testTxPoolConfig = DefaultTxPoolConfig
	testTxPoolConfig.Journal = ""
}

type testBlockChain struct {
	statedb       *state.DB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *testBlockChain) SetGasLimit(value uint64) {
	atomic.StoreUint64(&bc.gasLimit, value)
}

func (bc *testBlockChain) CurrentBlock() *types.Block {
	return types.NewBlock(blockfactory.NewTestHeader().With().
		GasLimit(atomic.LoadUint64(&bc.gasLimit)).
		Header(), nil, nil, nil, nil, nil)
}

func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

func (bc *testBlockChain) StateAt(common.Hash) (*state.DB, error) {
	return bc.statedb, nil
}

func (bc *testBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

// TODO: more staking tests in tx pool & testing lib
func stakingCreateValidatorTransaction(key *ecdsa.PrivateKey) (*staking.StakingTransaction, error) {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		p := &bls_core.PublicKey{}
		p.DeserializeHexStr(testBLSPubKey)
		pub := bls.SerializedPublicKey{}
		pub.FromLibBLSPublicKey(p)
		messageBytes := []byte(staking.BLSVerificationStr)
		privateKey := &bls_core.SecretKey{}
		privateKey.DeserializeHexStr(testBLSPrvKey)
		msgHash := hash.Keccak256(messageBytes)
		signature := privateKey.SignHash(msgHash[:])
		var sig bls.SerializedSignature
		copy(sig[:], signature.Serialize())

		ra, _ := numeric.NewDecFromStr("0.7")
		maxRate, _ := numeric.NewDecFromStr("1")
		maxChangeRate, _ := numeric.NewDecFromStr("0.5")
		return staking.DirectiveCreateValidator, staking.CreateValidator{
			Description: staking.Description{
				Name:            "SuperHero",
				Identity:        "YouWouldNotKnow",
				Website:         "Secret Website",
				SecurityContact: "LicenseToKill",
				Details:         "blah blah blah",
			},
			CommissionRates: staking.CommissionRates{
				Rate:          ra,
				MaxRate:       maxRate,
				MaxChangeRate: maxChangeRate,
			},
			MinSelfDelegation:  tenKOnes,
			MaxTotalDelegation: twelveKOnes,
			ValidatorAddress:   crypto.PubkeyToAddress(key.PublicKey),
			SlotPubKeys:        []bls.SerializedPublicKey{pub},
			SlotKeySigs:        []bls.SerializedSignature{sig},
			Amount:             tenKOnes,
		}
	}
	gasPrice := big.NewInt(100e9)
	tx, _ := staking.NewStakingTransaction(0, 1e10, gasPrice, stakePayloadMaker)
	return staking.Sign(tx, staking.NewEIP155Signer(tx.ChainID()), key)
}

func transaction(shardID uint32, nonce uint64, gaslimit uint64, key *ecdsa.PrivateKey) types.PoolTransaction {
	return pricedTransaction(shardID, nonce, gaslimit, big.NewInt(100e9), key)
}

func pricedTransaction(shardID uint32, nonce uint64, gaslimit uint64, gasprice *big.Int, key *ecdsa.PrivateKey) types.PoolTransaction {
	signedTx, _ := types.SignTx(types.NewTransaction(nonce, common.Address{}, shardID, big.NewInt(100000000000), gaslimit, gasprice, nil), types.HomesteadSigner{}, key)
	return signedTx
}

func createBlockChain() *BlockChainImpl {
	key, _ := crypto.GenerateKey()
	gspec := Genesis{
		Config:  params.TestChainConfig,
		Factory: blockfactory.ForTest,
		Alloc: GenesisAlloc{
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
	cacheConfig := &CacheConfig{SnapshotLimit: 0}
	blockchain, _ := NewBlockChain(database, nil, nil, cacheConfig, gspec.Config, engine, vm.Config{})
	return blockchain
}

func setupTxPool(chain blockChain) (*TxPool, *ecdsa.PrivateKey) {
	if chain == nil {
		statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		chain = &testBlockChain{statedb, 1e18, new(event.Feed)}
	}

	key, _ := crypto.GenerateKey()
	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, chain, dummyErrorSink)

	return pool, key
}

// validateTxPoolInternals checks various consistency invariants within the pool.
func validateTxPoolInternals(pool *TxPool) error {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// Ensure the total transaction set is consistent with pending + queued
	pending, queued := pool.stats()
	if total := pool.all.Count(); total != pending+queued {
		return fmt.Errorf("total transaction count %d != %d pending + %d queued", total, pending, queued)
	}
	if priced := pool.priced.items.Len() - pool.priced.stales; priced != pending+queued {
		return fmt.Errorf("total priced transaction count %d != %d pending + %d queued", priced, pending, queued)
	}
	// Ensure the next nonce to assign is the correct one
	for addr, txs := range pool.pending {
		// Find the last transaction
		var last uint64
		for nonce := range txs.txs.items {
			if last < nonce {
				last = nonce
			}
		}
		if nonce := pool.pendingState.GetNonce(addr); nonce != last+1 {
			return fmt.Errorf("pending nonce mismatch: have %v, want %v", nonce, last+1)
		}
	}
	return nil
}

func deriveSender(tx types.PoolTransaction) (common.Address, error) {
	return tx.SenderAddress()
}

type testChain struct {
	*testBlockChain
	address common.Address
	trigger *bool
}

// testChain.State() is used multiple times to reset the pending state.
// when simulate is true it will create a state that indicates
// that tx0 and tx1 are included in the chain.
func (c *testChain) State() (*state.DB, error) {
	// delay "state change" by one. The tx pool fetches the
	// state multiple times and by delaying it a bit we simulate
	// a state change between those fetches.
	stdb := c.statedb
	if *c.trigger {
		c.statedb, _ = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		// simulate that the new head block included tx0 and tx1
		c.statedb.SetNonce(c.address, 2)
		c.statedb.SetBalance(c.address, new(big.Int).SetUint64(denominations.One))
		*c.trigger = false
	}
	return stdb, nil
}

// This test simulates a scenario where a new block is imported during a
// state reset and tests whether the pending state is in sync with the
// block head event that initiated the resetState().
func TestStateChangeDuringTransactionPoolReset(t *testing.T) {
	t.Parallel()

	var (
		key, _     = crypto.GenerateKey()
		address    = crypto.PubkeyToAddress(key.PublicKey)
		statedb, _ = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		trigger    = false
	)

	// setup pool with 2 transaction in it
	statedb.SetBalance(address, new(big.Int).SetUint64(denominations.One))
	blockchain := &testChain{&testBlockChain{statedb, 1000000000, new(event.Feed)}, address, &trigger}

	tx0 := transaction(0, 0, 100000, key)
	tx1 := transaction(0, 1, 100000, key)

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	nonce := pool.State().GetNonce(address)
	if nonce != 0 {
		t.Fatalf("Invalid nonce, want 0, got %d", nonce)
	}

	pool.AddRemotes(types.PoolTransactions{tx0, tx1})

	nonce = pool.State().GetNonce(address)
	if nonce != 2 {
		t.Fatalf("Invalid nonce, want 2, got %d", nonce)
	}

	// trigger state change in the background
	trigger = true

	pool.lockedReset(nil, nil)

	_, err := pool.Pending()
	if err != nil {
		t.Fatalf("Could not fetch pending transactions: %v", err)
	}
	nonce = pool.State().GetNonce(address)
	if nonce != 2 {
		t.Fatalf("Invalid nonce, want 2, got %d", nonce)
	}
}

func TestInvalidTransactions(t *testing.T) {
	t.Parallel()

	pool, key := setupTxPool(nil)
	defer pool.Stop()

	tx := transaction(0, 0, 100, key)
	from, _ := deriveSender(tx)

	pool.currentState.AddBalance(from, big.NewInt(1))
	if err := pool.AddRemote(tx); err != ErrInsufficientFunds {
		t.Error("expected", ErrInsufficientFunds)
	}

	balance := new(big.Int).Add(tx.Value(), new(big.Int).Mul(new(big.Int).SetUint64(tx.GasLimit()), tx.GasPrice()))
	pool.currentState.AddBalance(from, balance)
	if err := pool.AddRemote(tx); err != ErrIntrinsicGas {
		t.Error("expected", ErrIntrinsicGas, "got", err)
	}

	pool.currentState.SetNonce(from, 1)
	pool.currentState.AddBalance(from, big.NewInt(0xffffffffffffff))
	tx = transaction(0, 0, 100000, key)
	if err := pool.AddRemote(tx); err != ErrNonceTooLow {
		t.Error("expected", ErrNonceTooLow)
	}

	tx = transaction(0, 1, 100000, key)
	pool.gasPrice = big.NewInt(300000000000)
	if err := pool.AddRemote(tx); err != ErrUnderpriced {
		t.Error("expected", ErrUnderpriced, "got", err)
	}
	if err := pool.AddLocal(tx); err != nil {
		t.Error("expected", nil, "got", err)
	}

	tx = transaction(1, 0, 100, key)
	if err := pool.AddRemote(tx); err != ErrInvalidShard {
		t.Error("expected", ErrInvalidShard, "got", err)
	}
}

func TestErrorSink(t *testing.T) {
	t.Parallel()

	pool, key := setupTxPool(createBlockChain())
	defer pool.Stop()

	testTxErrorSink := types.NewTransactionErrorSink()
	pool.txErrorSink = testTxErrorSink

	tx := transaction(0, 0, 100, key)
	from, _ := deriveSender(tx)

	stxKey, _ := crypto.GenerateKey()
	stx, err := stakingCreateValidatorTransaction(stxKey)
	if err != nil {
		t.Errorf("cannot create new staking transaction, %v\n", err)
	}
	fromStx, _ := stx.SenderAddress()

	pool.currentState.SetNonce(from, 1)
	pool.currentState.AddBalance(from, big.NewInt(0xffffffffffffff))
	tx = transaction(0, 0, 100000, key)
	if err := pool.AddRemote(tx); err != ErrNonceTooLow {
		t.Error("expected", ErrNonceTooLow)
	}
	if !testTxErrorSink.Contains(tx.Hash().String()) {
		t.Error("expected errored transaction in tx pool")
	}

	pool.currentState.SetNonce(from, 0)
	tx = transaction(0, 0, 100000, key)
	if err := pool.AddRemote(tx); err != nil {
		t.Error("expected successful transaction got", err)
	}
	if testTxErrorSink.Contains(tx.Hash().String()) {
		t.Error("expected successful transaction to not be in error sink")
	}

	pool.currentState.SetNonce(from, 2)
	tx = transaction(0, 2, 100000, key)
	pool.currentState.SetBalance(from, big.NewInt(0x0))
	pool.currentState.SetBalance(fromStx, big.NewInt(0x0))
	if err := pool.AddRemote(tx); err != ErrInsufficientFunds {
		t.Error("expected", ErrInsufficientFunds)
	}
	if err := pool.AddRemote(stx); err != ErrInsufficientFunds {
		t.Error("expected", ErrInsufficientFunds)
	}
	if !testTxErrorSink.Contains(tx.Hash().String()) {
		t.Error("expected errored transaction in tx pool")
	}
	if !testTxErrorSink.Contains(stx.Hash().String()) {
		t.Error("expected errored transaction in tx pool")
	}

	pool.currentState.SetBalance(from, twelveKOnes)
	pool.currentState.SetBalance(fromStx, twelveKOnes)
	if err := pool.AddRemote(tx); err != nil {
		t.Error("expected successful transaction got", err)
	}
	if err := pool.AddRemote(stx); err != nil {
		t.Error("expected successful transaction got", err)
	}
	if testTxErrorSink.Contains(tx.Hash().String()) {
		t.Error("expected successful transaction to not be in error sink")
	}
	if testTxErrorSink.Contains(stx.Hash().String()) {
		t.Error("expected successful transaction to not be in error sink")
	}
}

func TestCreateValidatorTransaction(t *testing.T) {
	t.Parallel()

	pool, _ := setupTxPool(createBlockChain())
	defer pool.Stop()

	fromKey, _ := crypto.GenerateKey()
	stx, err := stakingCreateValidatorTransaction(fromKey)
	if err != nil {
		t.Errorf("cannot create new staking transaction, %v\n", err)
	}
	senderAddr, _ := stx.SenderAddress()
	pool.currentState.AddBalance(senderAddr, hundredKOnes)
	// Add additional create validator tx cost
	pool.currentState.AddBalance(senderAddr, cost)

	if err = pool.AddRemote(stx); err != nil {
		t.Error(err.Error())
	}

	if pool.pending[senderAddr] == nil || pool.pending[senderAddr].Len() != 1 {
		t.Error("Expected 1 pending transaction")
	}
}

func TestMixedTransactions(t *testing.T) {
	t.Parallel()

	pool, _ := setupTxPool(createBlockChain())
	defer pool.Stop()

	fromKey, _ := crypto.GenerateKey()
	stx, err := stakingCreateValidatorTransaction(fromKey)
	if err != nil {
		t.Errorf("cannot create new staking transaction, %v\n", err)
	}
	stxAddr, _ := stx.SenderAddress()
	pool.currentState.AddBalance(stxAddr, hundredKOnes)
	// Add additional create validator tx cost
	pool.currentState.AddBalance(stxAddr, cost)

	goodFromKey, _ := crypto.GenerateKey()
	tx := transaction(0, 0, 25000, goodFromKey)
	txAddr, _ := deriveSender(tx)
	pool.currentState.AddBalance(txAddr, big.NewInt(5_010_000e9)) // 50100000000000 original value

	errs := pool.AddRemotes(types.PoolTransactions{stx, tx})
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}

	if pool.pending[stxAddr] == nil || pool.pending[stxAddr].Len() != 1 {
		t.Error("Expected 1 pending transaction")
	}
}

func TestBlacklistedTransactions(t *testing.T) {
	// DO NOT parallelize, test will add accounts to tx pool config.

	// Create the pool
	pool, _ := setupTxPool(nil)
	defer pool.Stop()

	// Create testing keys
	bannedFromKey, _ := crypto.GenerateKey()
	goodFromKey, _ := crypto.GenerateKey()

	// Create testing transactions
	badTx := transaction(0, 0, 25000, bannedFromKey)
	goodTx := transaction(0, 0, 25000, goodFromKey)
	bannedFromAcc, _ := deriveSender(badTx)
	bannedToAcc := *badTx.To()
	goodFromAcc, _ := deriveSender(goodTx)

	// Fund from accounts
	pool.currentState.AddBalance(bannedFromAcc, big.NewInt(15030000000000000))
	pool.currentState.AddBalance(goodFromAcc, big.NewInt(15030000000000000))

	DefaultTxPoolConfig.Blacklist[bannedToAcc] = struct{}{}
	err := pool.AddRemotes(types.PoolTransactions{badTx})
	if err[0] != ErrBlacklistTo {
		t.Error("expected", ErrBlacklistTo, "got", err[0])
	}

	delete(DefaultTxPoolConfig.Blacklist, bannedToAcc)
	DefaultTxPoolConfig.Blacklist[bannedFromAcc] = struct{}{}
	err = pool.AddRemotes(types.PoolTransactions{badTx})
	if err[0] != ErrBlacklistFrom {
		t.Error("expected", ErrBlacklistFrom, "got", err[0])
	}

	// to acc is same for bad and good tx, so keep off blacklist for valid tx check
	err = pool.AddRemotes(types.PoolTransactions{goodTx})
	if err[0] != nil {
		t.Error("expected", nil, "got", err[0])
	}

	// cleanup blacklist config for other tests
	DefaultTxPoolConfig.Blacklist = map[common.Address]struct{}{}
}

func TestTransactionQueue(t *testing.T) {
	t.Parallel()

	pool, key := setupTxPool(nil)
	defer pool.Stop()

	tx := transaction(0, 0, 100, key)
	from, _ := deriveSender(tx)
	pool.currentState.AddBalance(from, big.NewInt(100_000e9))
	pool.lockedReset(nil, nil)
	pool.enqueueTx(tx)

	pool.promoteExecutables([]common.Address{from})
	if len(pool.pending) != 1 {
		t.Error("expected valid txs to be 1 is", len(pool.pending))
	}

	tx = transaction(0, 1, 100, key)
	from, _ = deriveSender(tx)
	pool.currentState.SetNonce(from, 2)
	pool.enqueueTx(tx)
	pool.promoteExecutables([]common.Address{from})
	if _, ok := pool.pending[from].txs.items[tx.Nonce()]; ok {
		t.Error("expected transaction to be in tx pool")
	}

	if len(pool.queue) > 0 {
		t.Error("expected transaction queue to be empty. is", len(pool.queue))
	}

	pool, key = setupTxPool(nil)
	defer pool.Stop()

	tx1 := transaction(0, 0, 100, key)
	tx2 := transaction(0, 10, 100, key)
	tx3 := transaction(0, 11, 100, key)
	from, _ = deriveSender(tx1)
	pool.currentState.AddBalance(from, big.NewInt(30000000000000))
	pool.lockedReset(nil, nil)

	pool.enqueueTx(tx1)
	pool.enqueueTx(tx2)
	pool.enqueueTx(tx3)

	pool.promoteExecutables([]common.Address{from})

	if len(pool.pending) != 1 {
		t.Error("expected tx pool to be 1, got", len(pool.pending))
	}
	if pool.queue[from].Len() != 2 {
		t.Error("expected len(queue) == 2, got", pool.queue[from].Len())
	}
}

func TestTransactionNegativeValue(t *testing.T) {
	t.Parallel()

	pool, key := setupTxPool(nil)
	defer pool.Stop()

	tx, _ := types.SignTx(
		types.NewTransaction(0, common.Address{}, 0, big.NewInt(-1), 100, big.NewInt(1), nil),
		types.HomesteadSigner{}, key)
	from, _ := deriveSender(tx)
	pool.currentState.AddBalance(from, big.NewInt(1))
	if err := pool.AddRemote(tx); err != ErrNegativeValue {
		t.Error("expected", ErrNegativeValue, "got", err)
	}
}

func TestTransactionChainFork(t *testing.T) {
	t.Skip("This test doesn't work with race detector")
	t.Parallel()

	pool, key := setupTxPool(nil)
	defer pool.Stop()

	addr := crypto.PubkeyToAddress(key.PublicKey)
	resetState := func() {
		statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		statedb.AddBalance(addr, big.NewInt(9000000000000000000))

		pool.chain = &testBlockChain{statedb, 1000000, new(event.Feed)}
		pool.lockedReset(nil, nil)
	}
	resetState()

	tx := transaction(0, 0, 100000, key)
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
	pool.removeTx(tx.Hash(), true)

	// reset the pool's internal state
	resetState()
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
}

func TestTransactionDoubleNonce(t *testing.T) {
	t.Parallel()

	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	statedb.AddBalance(addr, big.NewInt(1000000000000000000))
	pool, _ := setupTxPool(&testBlockChain{statedb, 1000000, new(event.Feed)})
	defer pool.Stop()
	pool.lockedReset(nil, nil)

	signer := types.HomesteadSigner{}
	tx1, _ := types.SignTx(
		types.NewTransaction(0, common.Address{}, 0, big.NewInt(100), 100000, big.NewInt(100e9), nil),
		signer, key)
	tx2, _ := types.SignTx(
		types.NewTransaction(0, common.Address{}, 0, big.NewInt(100), 1000000, big.NewInt(101e9), nil), // related to price bump 1%
		signer, key)
	tx3, _ := types.SignTx(
		types.NewTransaction(0, common.Address{}, 0, big.NewInt(100), 1000000, big.NewInt(100e9), nil),
		signer, key)

	// Add the first two transaction, ensure higher priced stays only
	if replace, err := pool.add(tx1, false); err != nil || replace {
		t.Errorf("first transaction insert failed (%v) or reported replacement (%v)", err, replace)
	}
	if replace, err := pool.add(tx2, false); err != nil || !replace {
		t.Errorf("second transaction insert failed (%v) or not reported replacement (%v)", err, replace)
	}
	pool.promoteExecutables([]common.Address{addr})
	if pool.pending[addr].Len() != 1 {
		t.Error("expected 1 pending transactions, got", pool.pending[addr].Len())
	}
	if tx := pool.pending[addr].txs.items[0]; tx.Hash() != (*tx2).Hash() {
		t.Errorf("transaction mismatch: have %x, want %x", tx.Hash(), (*tx2).Hash())
	}
	// Add the third transaction and ensure it's not saved (smaller price)
	pool.add(tx3, false)
	pool.promoteExecutables([]common.Address{addr})
	if pool.pending[addr].Len() != 1 {
		t.Error("expected 1 pending transactions, got", pool.pending[addr].Len())
	}
	if tx := pool.pending[addr].txs.items[0]; tx.Hash() != (*tx2).Hash() {
		t.Errorf("transaction mismatch: have %x, want %x", tx.Hash(), (*tx2).Hash())
	}
	// Ensure the total transaction count is correct
	if pool.all.Count() != 1 {
		t.Error("expected 1 total transactions, got", pool.all.Count())
	}
}

func TestTransactionMissingNonce(t *testing.T) {
	t.Parallel()

	pool, key := setupTxPool(nil)
	defer pool.Stop()

	addr := crypto.PubkeyToAddress(key.PublicKey)
	pool.currentState.AddBalance(addr, big.NewInt(10010000e9))
	tx := transaction(0, 1, 100000, key)
	if _, err := pool.add(tx, false); err != nil {
		t.Error("didn't expect error", err)
	}
	if len(pool.pending) != 0 {
		t.Error("expected 0 pending transactions, got", len(pool.pending))
	}
	if pool.queue[addr].Len() != 1 {
		t.Error("expected 1 queued transaction, got", pool.queue[addr].Len())
	}
	if pool.all.Count() != 1 {
		t.Error("expected 1 total transactions, got", pool.all.Count())
	}
}

func TestTransactionNonceRecovery(t *testing.T) {
	t.Parallel()

	const n = 10
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	addr := crypto.PubkeyToAddress(key.PublicKey)
	pool.currentState.SetNonce(addr, n)
	pool.currentState.AddBalance(addr, big.NewInt(10010000e9))
	pool.lockedReset(nil, nil)

	tx := transaction(0, n, 100000, key)
	if err := pool.AddRemote(tx); err != nil {
		t.Error(err)
	}
	// simulate some weird re-order of transactions and missing nonce(s)
	pool.currentState.SetNonce(addr, n-1)
	pool.lockedReset(nil, nil)
	if fn := pool.pendingState.GetNonce(addr); fn != n-1 {
		t.Errorf("expected nonce to be %d, got %d", n-1, fn)
	}
}

// Tests that if an account runs out of funds, any pending and queued transactions
// are dropped.
func TestTransactionDropping(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(100_000_000_000_000))

	// Add some pending and some queued transactions
	var (
		tx0  = transaction(0, 0, 100, key)
		tx1  = transaction(0, 1, 200, key)
		tx2  = transaction(0, 2, 300, key)
		tx10 = transaction(0, 10, 100, key)
		tx11 = transaction(0, 11, 200, key)
		tx12 = transaction(0, 12, 300, key)
	)
	pool.promoteTx(account, tx0)
	pool.promoteTx(account, tx1)
	pool.promoteTx(account, tx2)
	pool.enqueueTx(tx10)
	pool.enqueueTx(tx11)
	pool.enqueueTx(tx12)

	// Check that pre and post validations leave the pool as is
	if pool.pending[account].Len() != 3 {
		t.Errorf("pending transaction mismatch: have %d, want %d", pool.pending[account].Len(), 3)
	}
	if pool.queue[account].Len() != 3 {
		t.Errorf("queued transaction mismatch: have %d, want %d", pool.queue[account].Len(), 3)
	}
	if pool.all.Count() != 6 {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), 6)
	}
	pool.lockedReset(nil, nil)
	if pool.pending[account].Len() != 3 {
		t.Errorf("pending transaction mismatch: have %d, want %d", pool.pending[account].Len(), 3)
	}
	if pool.queue[account].Len() != 3 {
		t.Errorf("queued transaction mismatch: have %d, want %d", pool.queue[account].Len(), 3)
	}
	if pool.all.Count() != 6 {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), 6)
	}
	// Reduce the balance of the account, and check that invalidated transactions are dropped
	pool.currentState.AddBalance(account, big.NewInt(-75_000e9))
	pool.lockedReset(nil, nil)

	if _, ok := pool.pending[account].txs.items[tx0.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx0)
	}
	if _, ok := pool.pending[account].txs.items[tx1.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx1)
	}
	if _, ok := pool.pending[account].txs.items[tx2.Nonce()]; ok {
		t.Errorf("out-of-fund pending transaction present: %v", tx2)
	}
	if _, ok := pool.queue[account].txs.items[tx10.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx10)
	}
	if _, ok := pool.queue[account].txs.items[tx11.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx11)
	}
	if _, ok := pool.queue[account].txs.items[tx12.Nonce()]; ok {
		t.Errorf("out-of-fund queued transaction present: %v", tx12)
	}
	if pool.all.Count() != 4 {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), 4)
	}
	// Reduce the block gas limit, check that invalidated transactions are dropped
	pool.chain.(*testBlockChain).SetGasLimit(100)
	pool.lockedReset(nil, nil)

	if _, ok := pool.pending[account].txs.items[tx0.Nonce()]; !ok {
		t.Errorf("funded pending transaction missing: %v", tx0)
	}
	if _, ok := pool.pending[account].txs.items[tx1.Nonce()]; ok {
		t.Errorf("over-gased pending transaction present: %v", tx1)
	}
	if _, ok := pool.queue[account].txs.items[tx10.Nonce()]; !ok {
		t.Errorf("funded queued transaction missing: %v", tx10)
	}
	if _, ok := pool.queue[account].txs.items[tx11.Nonce()]; ok {
		t.Errorf("over-gased queued transaction present: %v", tx11)
	}
	if pool.all.Count() != 2 {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), 2)
	}
}

// Tests that if a transaction is dropped from the current pending pool (e.g. out
// of fund), all consecutive (still valid, but not executable) transactions are
// postponed back into the future queue to prevent broadcasting them.
func TestTransactionPostponing(t *testing.T) {
	t.Parallel()

	// Create the pool to test the postponing with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create two test accounts to produce different gap profiles with
	keys := make([]*ecdsa.PrivateKey, 2)
	accs := make([]common.Address, len(keys))

	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		accs[i] = crypto.PubkeyToAddress(keys[i].PublicKey)

		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(5000100000000000))
	}
	// Add a batch consecutive pending transactions for validation
	txs := types.PoolTransactions{}
	for i, key := range keys {

		for j := 0; j < 100; j++ {
			var tx types.PoolTransaction
			if (i+j)%2 == 0 {
				tx = transaction(0, uint64(j), 25000, key)
			} else {
				tx = transaction(0, uint64(j), 50000, key)
			}
			txs = append(txs, tx)
		}
	}
	for i, err := range pool.AddRemotes(txs) {
		if err != nil {
			t.Fatalf("tx %d: failed to add transactions: %v", i, err)
		}
	}
	// Check that pre and post validations leave the pool as is
	if pending := pool.pending[accs[0]].Len() + pool.pending[accs[1]].Len(); pending != len(txs) {
		t.Errorf("pending transaction mismatch: have %d, want %d", pending, len(txs))
	}
	if len(pool.queue) != 0 {
		t.Errorf("queued accounts mismatch: have %d, want %d", len(pool.queue), 0)
	}
	if pool.all.Count() != len(txs) {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), len(txs))
	}
	pool.lockedReset(nil, nil)
	if pending := pool.pending[accs[0]].Len() + pool.pending[accs[1]].Len(); pending != len(txs) {
		t.Errorf("pending transaction mismatch: have %d, want %d", pending, len(txs))
	}
	if len(pool.queue) != 0 {
		t.Errorf("queued accounts mismatch: have %d, want %d", len(pool.queue), 0)
	}
	if pool.all.Count() != len(txs) {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), len(txs))
	}
	// Reduce the balance of the account, and check that transactions are reorganised
	for _, addr := range accs {
		pool.currentState.AddBalance(addr, big.NewInt(-30))
	}
	pool.lockedReset(nil, nil)

	// The first account's first transaction remains valid, check that subsequent
	// ones are either filtered out, or queued up for later.
	if _, ok := pool.pending[accs[0]].txs.items[txs[0].Nonce()]; !ok {
		t.Errorf("tx %d: valid and funded transaction missing from pending pool: %v", 0, txs[0])
	}
	if pool.queue[accs[0]] != nil {
		if _, ok := pool.queue[accs[0]].txs.items[txs[0].Nonce()]; ok {
			t.Errorf("tx %d: valid and funded transaction present in future queue: %v", 0, txs[0])
		}
	}
	for i, tx := range txs[1:100] {
		if i%2 == 1 {
			if _, ok := pool.pending[accs[0]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: valid but future transaction present in pending pool: %v", i+1, tx)
			}
			if _, ok := pool.queue[accs[0]].txs.items[tx.Nonce()]; !ok {
				t.Errorf("tx %d: valid but future transaction missing from future queue: %v", i+1, tx)
			}
		} else {
			if _, ok := pool.pending[accs[0]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: out-of-fund transaction present in pending pool: %v", i+1, tx)
			}
			if pool.queue[accs[0]] != nil {
				if _, ok := pool.queue[accs[0]].txs.items[tx.Nonce()]; ok {
					t.Errorf("tx %d: out-of-fund transaction present in future queue: %v", i+1, tx)
				}
			}
		}
	}
	// The second account's first transaction got invalid, check that all transactions
	// are either filtered out, or queued up for later.
	if pool.pending[accs[1]] != nil {
		t.Errorf("invalidated account still has pending transactions")
	}
	for i, tx := range txs[100:] {
		if i%2 == 1 {
			if _, ok := pool.queue[accs[1]].txs.items[tx.Nonce()]; !ok {
				t.Errorf("tx %d: valid but future transaction missing from future queue: %v", 100+i, tx)
			}
		} else {
			if _, ok := pool.queue[accs[1]].txs.items[tx.Nonce()]; ok {
				t.Errorf("tx %d: out-of-fund transaction present in future queue: %v", 100+i, tx)
			}
		}
	}
	if pool.all.Count() != len(txs)/2 {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), len(txs)/2)
	}
}

// Tests that if the transaction count belonging to a single account goes above
// some threshold, the higher transactions are dropped to prevent DOS attacks.
func TestTransactionQueueAccountLimiting(t *testing.T) {
	t.Parallel()

	// Create a test account and fund it
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(9000000000000000000))

	// Keep queuing up transactions and make sure all above a limit are dropped
	for i := uint64(1); i <= testTxPoolConfig.AccountQueue+5; i++ {
		if err := pool.AddRemote(transaction(0, i, 100000, key)); err != nil {
			t.Fatalf("tx %d: failed to add transaction: %v", i, err)
		}
		if len(pool.pending) != 0 {
			t.Errorf("tx %d: pending pool size mismatch: have %d, want %d", i, len(pool.pending), 0)
		}
		if i <= testTxPoolConfig.AccountQueue {
			if pool.queue[account].Len() != int(i) {
				t.Errorf("tx %d: queue size mismatch: have %d, want %d", i, pool.queue[account].Len(), i)
			}
		} else {
			if pool.queue[account].Len() != int(testTxPoolConfig.AccountQueue) {
				t.Errorf("tx %d: queue limit mismatch: have %d, want %d", i, pool.queue[account].Len(), testTxPoolConfig.AccountQueue)
			}
		}
	}
	if pool.all.Count() != int(testTxPoolConfig.AccountQueue) {
		t.Errorf("total transaction mismatch: have %d, want %d", pool.all.Count(), testTxPoolConfig.AccountQueue)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some threshold, the higher transactions are dropped to prevent DOS attacks.
//
// This logic should not hold for local transactions, unless the local tracking
// mechanism is disabled.
func TestTransactionQueueGlobalLimiting(t *testing.T) {
	testTransactionQueueGlobalLimiting(t, false)
}
func TestTransactionQueueGlobalLimitingNoLocals(t *testing.T) {
	testTransactionQueueGlobalLimiting(t, true)
}

func testTransactionQueueGlobalLimiting(t *testing.T, nolocals bool) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.NoLocals = nolocals
	config.GlobalQueue = config.AccountQueue*3 - 1 // reduce the queue limits to shorten test time (-1 to make it non divisible)

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create a number of test accounts and fund them (last one will be the local)
	keys := make([]*ecdsa.PrivateKey, 5)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(30000000000000000))
	}
	local := keys[len(keys)-1]

	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := make(types.PoolTransactions, 0, 3*config.GlobalQueue)
	for len(txs) < cap(txs) {
		key := keys[rand.Intn(len(keys)-1)] // skip adding transactions with the local account
		addr := crypto.PubkeyToAddress(key.PublicKey)

		txs = append(txs, transaction(0, nonces[addr]+1, 100000, key))
		nonces[addr]++
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	queued := 0
	for addr, list := range pool.queue {
		if list.Len() > int(config.AccountQueue) {
			t.Errorf("addr %x: queued accounts overflown allowance: %d > %d", addr, list.Len(), config.AccountQueue)
		}
		queued += list.Len()
	}
	if queued > int(config.GlobalQueue) {
		t.Fatalf("total transactions overflow allowance: %d > %d", queued, config.GlobalQueue)
	}
	// Generate a batch of transactions from the local account and import them
	txs = txs[:0]
	for i := uint64(0); i < 3*config.GlobalQueue; i++ {
		txs = append(txs, transaction(0, i+1, 100000, local))
	}
	pool.AddLocals(txs)

	// If locals are disabled, the previous eviction algorithm should apply here too
	if nolocals {
		queued := 0
		for addr, list := range pool.queue {
			if list.Len() > int(config.AccountQueue) {
				t.Errorf("addr %x: queued accounts overflown allowance: %d > %d", addr, list.Len(), config.AccountQueue)
			}
			queued += list.Len()
		}
		if queued > int(config.GlobalQueue) {
			t.Fatalf("total transactions overflow allowance: %d > %d", queued, config.GlobalQueue)
		}
	} else {
		// Local exemptions are enabled, make sure the local account owned the queue
		if len(pool.queue) != 1 {
			t.Errorf("multiple accounts in queue: have %v, want %v", len(pool.queue), 1)
		}
		// Also ensure no local transactions are ever dropped, even if above global limits
		if queued := pool.queue[crypto.PubkeyToAddress(local.PublicKey)].Len(); uint64(queued) != 3*config.GlobalQueue {
			t.Fatalf("local account queued transaction count mismatch: have %v, want %v", queued, 3*config.GlobalQueue)
		}
	}
}

// Tests that if an account remains idle for a prolonged amount of time, any
// non-executable transactions queued up are dropped to prevent wasting resources
// on shuffling them around.
//
// This logic should not hold for local transactions, unless the local tracking
// mechanism is disabled.
func TestTransactionQueueTimeLimiting(t *testing.T) { testTransactionQueueTimeLimiting(t, false) }
func TestTransactionQueueTimeLimitingNoLocals(t *testing.T) {
	testTransactionQueueTimeLimiting(t, true)
}

func testTransactionQueueTimeLimiting(t *testing.T, nolocals bool) {
	// Reduce the eviction interval to a testable amount
	defer func(old time.Duration) { evictionInterval = old }(evictionInterval)
	evictionInterval = time.Second

	// Create the pool to test the non-expiration enforcement
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.Lifetime = time.Second
	config.NoLocals = nolocals

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create two test accounts to ensure remotes expire but locals do not
	local, _ := crypto.GenerateKey()
	remote, _ := crypto.GenerateKey()

	pool.currentState.AddBalance(crypto.PubkeyToAddress(local.PublicKey), big.NewInt(9000000000000000000))
	pool.currentState.AddBalance(crypto.PubkeyToAddress(remote.PublicKey), big.NewInt(9000000000000000000))

	// Add the two transactions and ensure they both are queued up
	if err := pool.AddLocal(pricedTransaction(0, 1, 100000, big.NewInt(100000000000), local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransaction(0, 1, 100000, big.NewInt(100000000000), remote)); err != nil {
		t.Fatalf("failed to add remote transaction: %v", err)
	}
	pending, queued := pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Wait a bit for eviction to run and clean up any leftovers, and ensure only the local remains
	time.Sleep(4 * config.Lifetime)

	pending, queued = pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if nolocals {
		if queued != 0 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
		}
	} else {
		if queued != 1 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that the transaction limits are enforced the same way irrelevant whether
// the transactions are added one by one or in batches.
func TestTransactionQueueLimitingEquivalency(t *testing.T) { testTransactionLimitingEquivalency(t, 1) }
func TestTransactionPendingLimitingEquivalency(t *testing.T) {
	testTransactionLimitingEquivalency(t, 0)
}

func testTransactionLimitingEquivalency(t *testing.T, origin uint64) {
	t.Parallel()

	// Add a batch of transactions to a pool one by one
	pool1, key1 := setupTxPool(nil)
	defer pool1.Stop()

	account1, _ := deriveSender(transaction(0, 0, 0, key1))
	pool1.currentState.AddBalance(account1, big.NewInt(9000000000000000000))

	for i := uint64(0); i < testTxPoolConfig.AccountQueue+5; i++ {
		if err := pool1.AddRemote(transaction(0, origin+i, 100000, key1)); err != nil {
			t.Fatalf("tx %d: failed to add transaction: %v", i, err)
		}
	}
	// Add a batch of transactions to a pool in one big batch
	pool2, key2 := setupTxPool(nil)
	defer pool2.Stop()

	account2, _ := deriveSender(transaction(0, 0, 0, key2))
	pool2.currentState.AddBalance(account2, big.NewInt(9000000000000000000))

	txs := types.PoolTransactions{}
	for i := uint64(0); i < testTxPoolConfig.AccountQueue+5; i++ {
		txs = append(txs, transaction(0, origin+i, 100000, key2))
	}
	pool2.AddRemotes(txs)

	// Ensure the batch optimization honors the same pool mechanics
	if len(pool1.pending) != len(pool2.pending) {
		t.Errorf("pending transaction count mismatch: one-by-one algo: %d, batch algo: %d", len(pool1.pending), len(pool2.pending))
	}
	if len(pool1.queue) != len(pool2.queue) {
		t.Errorf("queued transaction count mismatch: one-by-one algo: %d, batch algo: %d", len(pool1.queue), len(pool2.queue))
	}
	if pool1.all.Count() != pool2.all.Count() {
		t.Errorf("total transaction count mismatch: one-by-one algo %d, batch algo %d", pool1.all.Count(), pool2.all.Count())
	}
	if err := validateTxPoolInternals(pool1); err != nil {
		t.Errorf("pool 1 internal state corrupted: %v", err)
	}
	if err := validateTxPoolInternals(pool2); err != nil {
		t.Errorf("pool 2 internal state corrupted: %v", err)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some hard threshold, the higher transactions are dropped to prevent DOS
// attacks.
func TestTransactionPendingGlobalLimiting(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = config.AccountSlots * 10

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]*ecdsa.PrivateKey, 5)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := types.PoolTransactions{}
	for _, key := range keys {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		for j := 0; j < int(config.GlobalSlots)/len(keys)*2; j++ {
			txs = append(txs, transaction(0, nonces[addr], 100000, key))
			nonces[addr]++
		}
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	if pending > int(config.GlobalSlots) {
		t.Fatalf("total pending transactions overflow allowance: %d > %d", pending, config.GlobalSlots)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that if transactions start being capped, transactions are also removed from 'all'
func TestTransactionCapClearsFromAll(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.AccountSlots = 2
	config.AccountQueue = 2
	config.GlobalSlots = 8

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	pool.currentState.AddBalance(addr, big.NewInt(1000000))

	txs := types.PoolTransactions{}
	for j := 0; j < int(config.GlobalSlots)*2; j++ {
		txs = append(txs, transaction(0, uint64(j), 100000, key))
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that if the transaction count belonging to multiple accounts go above
// some hard threshold, if they are under the minimum guaranteed slot count then
// the transactions are still kept.
func TestTransactionPendingMinimumAllowance(t *testing.T) {
	t.Parallel()

	// Create the pool to test the limit enforcement with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.GlobalSlots = 0

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]*ecdsa.PrivateKey, 5)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(1000000))
	}
	// Generate and queue a batch of transactions
	nonces := make(map[common.Address]uint64)

	txs := types.PoolTransactions{}
	for _, key := range keys {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		for j := 0; j < int(config.AccountSlots)*2; j++ {
			txs = append(txs, transaction(0, nonces[addr], 100000, key))
			nonces[addr]++
		}
	}
	// Import the batch and verify that limits have been enforced
	pool.AddRemotes(txs)

	for addr, list := range pool.pending {
		if list.Len() != int(config.AccountSlots) {
			t.Errorf("addr %x: total pending transactions mismatch: have %d, want %d", addr, list.Len(), config.AccountSlots)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
}

// Tests that setting the transaction pool gas price to a higher value does not
// remove local transactions.
func TestTransactionPoolRepricingKeepsLocals(t *testing.T) {
	t.Parallel()

	// Create the pool to test the pricing enforcement with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create a number of test accounts and fund them
	keys := make([]*ecdsa.PrivateKey, 3)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(1000*1000000*1000000000))
	}
	// Create transaction (both pending and queued) with a linearly growing gasprice
	for i := uint64(0); i < 500; i++ {
		// Add pending
		pTx := pricedTransaction(0, i, 100000, big.NewInt(int64(30000000000+i*1000000000)), keys[2])
		if err := pool.AddLocal(pTx); err != nil {
			t.Fatal(err)
		}
		// Add queued
		qTx := pricedTransaction(0, i+501, 100000, big.NewInt(int64(30000000000+i*1000000000)), keys[2])
		if err := pool.AddLocal(qTx); err != nil {
			t.Fatal(err)
		}
	}
	pending, queued := pool.Stats()
	expPending, expQueued := 500, 500
	validate := func() {
		pending, queued = pool.Stats()
		if pending != expPending {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, expPending)
		}
		if queued != expQueued {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, expQueued)
		}

		if err := validateTxPoolInternals(pool); err != nil {
			t.Fatalf("pool internal state corrupted: %v", err)
		}
	}
	validate()

	// Reprice the pool and check that nothing is dropped
	pool.SetGasPrice(big.NewInt(2000000000))
	validate()

	pool.SetGasPrice(big.NewInt(2000000000))
	pool.SetGasPrice(big.NewInt(4000000000))
	pool.SetGasPrice(big.NewInt(8000000000))
	pool.SetGasPrice(big.NewInt(100000000000))
	validate()
}

// Tests that local transactions are journaled to disk, but remote transactions
// get discarded between restarts.
func TestTransactionJournaling(t *testing.T)         { testTransactionJournaling(t, false) }
func TestTransactionJournalingNoLocals(t *testing.T) { testTransactionJournaling(t, true) }

func testTransactionJournaling(t *testing.T, nolocals bool) {
	t.Parallel()

	// Create a temporary file for the journal
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("failed to create temporary journal: %v", err)
	}
	journal := file.Name()
	defer os.Remove(journal)

	// Clean up the temporary file, we only need the path for now
	file.Close()
	os.Remove(journal)

	// Create the original pool to inject transaction into the journal
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	config := testTxPoolConfig
	config.NoLocals = nolocals
	config.Journal = journal
	config.Rejournal = time.Second

	pool := NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)

	// Create two test accounts to ensure remotes expire but locals do not
	local, _ := crypto.GenerateKey()
	remote, _ := crypto.GenerateKey()

	pool.currentState.AddBalance(crypto.PubkeyToAddress(local.PublicKey), big.NewInt(9_000_000_000e9))
	pool.currentState.AddBalance(crypto.PubkeyToAddress(remote.PublicKey), big.NewInt(9_000_000_000e9))

	// Add three local and a remote transactions and ensure they are queued up
	if err := pool.AddLocal(pricedTransaction(0, 0, 100000, big.NewInt(100e9), local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddLocal(pricedTransaction(0, 1, 100000, big.NewInt(100e9), local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddLocal(pricedTransaction(0, 2, 100000, big.NewInt(100e9), local)); err != nil {
		t.Fatalf("failed to add local transaction: %v", err)
	}
	if err := pool.AddRemote(pricedTransaction(0, 0, 100000, big.NewInt(100e9), remote)); err != nil {
		t.Fatalf("failed to add remote transaction: %v", err)
	}
	pending, queued := pool.Stats()
	if pending != 4 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 4)
	}
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Terminate the old pool, bump the local nonce, create a new pool and ensure relevant transaction survive
	pool.Stop()
	statedb.SetNonce(crypto.PubkeyToAddress(local.PublicKey), 1)
	blockchain = &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool = NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)

	pending, queued = pool.Stats()
	if queued != 0 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
	}
	if nolocals {
		if pending != 0 {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
		}
	} else {
		if pending != 2 {
			t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Bump the nonce temporarily and ensure the newly invalidated transaction is removed
	statedb.SetNonce(crypto.PubkeyToAddress(local.PublicKey), 2)
	pool.lockedReset(nil, nil)
	time.Sleep(2 * config.Rejournal)
	pool.Stop()

	statedb.SetNonce(crypto.PubkeyToAddress(local.PublicKey), 1)
	blockchain = &testBlockChain{statedb, 1000000, new(event.Feed)}
	pool = NewTxPool(config, params.TestChainConfig, blockchain, dummyErrorSink)

	pending, queued = pool.Stats()
	if pending != 0 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 0)
	}
	if nolocals {
		if queued != 0 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 0)
		}
	} else {
		if queued != 1 {
			t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 1)
		}
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	pool.Stop()
}

// TestTransactionStatusCheck tests that the pool can correctly retrieve the
// pending status of individual transactions.
func TestTransactionStatusCheck(t *testing.T) {
	t.Parallel()

	// Create the pool to test the status retrievals with
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	blockchain := &testBlockChain{statedb, 1000000, new(event.Feed)}

	pool := NewTxPool(testTxPoolConfig, params.TestChainConfig, blockchain, dummyErrorSink)
	defer pool.Stop()

	// Create the test accounts to check various transaction statuses with
	keys := make([]*ecdsa.PrivateKey, 3)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
		pool.currentState.AddBalance(crypto.PubkeyToAddress(keys[i].PublicKey), big.NewInt(9000000000000000000))
	}
	// Generate and queue a batch of transactions, both pending and queued
	txs := types.PoolTransactions{}

	txs = append(txs, pricedTransaction(0, 0, 100000, big.NewInt(100e9), keys[0])) // Pending only
	txs = append(txs, pricedTransaction(0, 0, 100000, big.NewInt(100e9), keys[1])) // Pending and queued
	txs = append(txs, pricedTransaction(0, 2, 100000, big.NewInt(100e9), keys[1]))
	txs = append(txs, pricedTransaction(0, 2, 100000, big.NewInt(100e9), keys[2])) // Queued only

	// Import the transaction and ensure they are correctly added
	pool.AddRemotes(txs)

	pending, queued := pool.Stats()
	if pending != 2 {
		t.Fatalf("pending transactions mismatched: have %d, want %d", pending, 2)
	}
	if queued != 2 {
		t.Fatalf("queued transactions mismatched: have %d, want %d", queued, 2)
	}
	if err := validateTxPoolInternals(pool); err != nil {
		t.Fatalf("pool internal state corrupted: %v", err)
	}
	// Retrieve the status of each transaction and validate them
	hashes := make([]common.Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.Hash()
	}
	hashes = append(hashes, common.Hash{})

	statuses := pool.Status(hashes)
	expect := []TxStatus{TxStatusPending, TxStatusPending, TxStatusQueued, TxStatusQueued, TxStatusUnknown}

	for i := 0; i < len(statuses); i++ {
		if statuses[i] != expect[i] {
			t.Errorf("transaction %d: status mismatch: have %v, want %v", i, statuses[i], expect[i])
		}
	}
}

// Benchmarks the speed of validating the contents of the pending queue of the
// transaction pool.
func BenchmarkPendingDemotion100(b *testing.B)   { benchmarkPendingDemotion(b, 100) }
func BenchmarkPendingDemotion1000(b *testing.B)  { benchmarkPendingDemotion(b, 1000) }
func BenchmarkPendingDemotion10000(b *testing.B) { benchmarkPendingDemotion(b, 10000) }

func benchmarkPendingDemotion(b *testing.B, size int) {
	// Add a batch of transactions to a pool one by one
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	for i := 0; i < size; i++ {
		tx := transaction(0, uint64(i), 100000, key)
		pool.promoteTx(account, tx)
	}
	// Benchmark the speed of pool validation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.demoteUnexecutables(0)
	}
}

// Benchmarks the speed of scheduling the contents of the future queue of the
// transaction pool.
func BenchmarkFuturePromotion100(b *testing.B)   { benchmarkFuturePromotion(b, 100) }
func BenchmarkFuturePromotion1000(b *testing.B)  { benchmarkFuturePromotion(b, 1000) }
func BenchmarkFuturePromotion10000(b *testing.B) { benchmarkFuturePromotion(b, 10000) }

func benchmarkFuturePromotion(b *testing.B, size int) {
	// Add a batch of transactions to a pool one by one
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	for i := 0; i < size; i++ {
		tx := transaction(0, uint64(1+i), 100000, key)
		pool.enqueueTx(tx)
	}
	// Benchmark the speed of pool validation
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.promoteExecutables(nil)
	}
}

// Benchmarks the speed of iterative transaction insertion.
func BenchmarkPoolInsert(b *testing.B) {
	// Generate a batch of transactions to enqueue into the pool
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	txs := make(types.PoolTransactions, b.N)
	for i := 0; i < b.N; i++ {
		txs[i] = transaction(0, uint64(i), 100000, key)
	}
	// Benchmark importing the transactions into the queue
	b.ResetTimer()
	for _, tx := range txs {
		pool.AddRemote(tx)
	}
}

// Benchmarks the speed of batched transaction insertion.
func BenchmarkPoolBatchInsert100(b *testing.B)   { benchmarkPoolBatchInsert(b, 100) }
func BenchmarkPoolBatchInsert1000(b *testing.B)  { benchmarkPoolBatchInsert(b, 1000) }
func BenchmarkPoolBatchInsert10000(b *testing.B) { benchmarkPoolBatchInsert(b, 10000) }

func benchmarkPoolBatchInsert(b *testing.B, size int) {
	// Generate a batch of transactions to enqueue into the pool
	pool, key := setupTxPool(nil)
	defer pool.Stop()

	account, _ := deriveSender(transaction(0, 0, 0, key))
	pool.currentState.AddBalance(account, big.NewInt(1000000))

	batches := make([]types.PoolTransactions, b.N)
	for i := 0; i < b.N; i++ {
		batches[i] = make(types.PoolTransactions, size)
		for j := 0; j < size; j++ {
			batches[i][j] = transaction(0, uint64(size*i+j), 100000, key)
		}
	}
	// Benchmark importing the transactions into the queue
	b.ResetTimer()
	for _, batch := range batches {
		pool.AddRemotes(batch)
	}
}
