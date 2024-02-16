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

package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core"

	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// BlockGen creates blocks for testing.
// See GenerateChain for a detailed explanation.
type BlockGen struct {
	i        int
	parent   *block.Header
	chain    []*types.Block
	factory  blockfactory.Factory
	header   *block.Header
	statedb  *state.DB
	gasPool  *core.GasPool
	txs      []*types.Transaction
	receipts []*types.Receipt
	uncles   []*block.Header
	config   *params.ChainConfig
	engine   consensus_engine.Engine
}

// SetCoinbase sets the coinbase of the generated block.
// It can be called at most once.
func (b *BlockGen) SetCoinbase(addr common.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("coinbase must be set before adding transactions")
		}
		panic("coinbase can only be set once")
	}
	b.header.SetCoinbase(addr)
	b.gasPool = new(core.GasPool).AddGas(b.header.GasLimit())
}

// SetExtra sets the extra data field of the generated block.
func (b *BlockGen) SetExtra(data []byte) {
	b.header.SetExtra(data)
}

// SetShardID sets the shardID field of the generated block.
func (b *BlockGen) SetShardID(shardID uint32) {
	b.header.SetShardID(shardID)
}

// AddTx adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTx panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. Notably, contract code relying on the BLOCKHASH instruction
// will panic during execution.
func (b *BlockGen) AddTx(tx *types.Transaction) {
	b.AddTxWithChain(nil, tx)
}

// AddTxWithChain adds a transaction to the generated block. If no coinbase has
// been set, the block's coinbase is set to the zero address.
//
// AddTxWithChain panics if the transaction cannot be executed. In addition to
// the protocol-imposed limitations (gas limit, etc.), there are some
// further limitations on the content of transactions that can be
// added. If contract code relies on the BLOCKHASH instruction,
// the block in chain will be returned.
func (b *BlockGen) AddTxWithChain(bc core.BlockChain, tx *types.Transaction) {
	if b.gasPool == nil {
		b.SetCoinbase(common.Address{})
	}
	b.statedb.Prepare(tx.Hash(), common.Hash{}, len(b.txs))
	coinbase := b.header.Coinbase()
	gasUsed := b.header.GasUsed()
	receipt, _, _, _, err := core.ApplyTransaction(bc, &coinbase, b.gasPool, b.statedb, b.header, tx, &gasUsed, vm.Config{})
	b.header.SetGasUsed(gasUsed)
	b.header.SetCoinbase(coinbase)
	if err != nil {
		panic(err)
	}
	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}

// Number returns the block number of the block being generated.
func (b *BlockGen) Number() *big.Int {
	return b.header.Number()
}

// AddUncheckedReceipt forcefully adds a receipts to the block without a
// backing transaction.
//
// AddUncheckedReceipt will cause consensus failures when used during real
// chain processing. This is best used in conjunction with raw block insertion.
func (b *BlockGen) AddUncheckedReceipt(receipt *types.Receipt) {
	b.receipts = append(b.receipts, receipt)
}

// TxNonce returns the next valid transaction nonce for the
// account at addr. It panics if the account does not exist.
func (b *BlockGen) TxNonce(addr common.Address) uint64 {
	if !b.statedb.Exist(addr) {
		panic("account does not exist")
	}
	return b.statedb.GetNonce(addr)
}

// AddUncle adds an uncle header to the generated block.
func (b *BlockGen) AddUncle(h *block.Header) {
	b.uncles = append(b.uncles, h)
}

// GenerateChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions and uncles added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their coinbase will be the zero address.
//
// Blocks created by GenerateChain do not contain valid proof of work
// values. Inserting them into BlockChain requires use of FakePow or
// a similar non-validating proof of work implementation.
func GenerateChain(
	config *params.ChainConfig, parent *block.Header,
	engine consensus_engine.Engine, db ethdb.Database,
	n int,
	gen func(int, *BlockGen),
) ([]*types.Block, []types.Receipts) {
	if config == nil {
		config = params.TestChainConfig
	}
	factory := blockfactory.NewFactory(config)
	blocks, receipts := make(types.Blocks, n), make([]types.Receipts, n)
	chainreader := &fakeChainReader{config: config}
	genblock := func(i int, parent *block.Header, statedb *state.DB) (*types.Block, types.Receipts) {
		b := &BlockGen{
			i:       i,
			chain:   blocks,
			parent:  parent,
			statedb: statedb,
			config:  config,
			factory: factory,
			engine:  engine,
		}
		b.header = makeHeader(chainreader, parent, statedb, factory)

		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}

		if b.engine != nil {
			// Finalize and seal the block
			block, _, err := b.engine.Finalize(
				chainreader, nil, b.header, statedb, b.txs, b.receipts, nil, nil, nil, nil, nil, func() uint64 { return 0 },
			)
			if err != nil {
				panic(err)
			}

			// Write state changes to db
			root, err := statedb.Commit(config.IsS3(b.header.Epoch()))
			if err != nil {
				panic(fmt.Sprintf("state write error: %v", err))
			}
			if err := statedb.Database().TrieDB().Commit(root, false); err != nil {
				panic(fmt.Sprintf("trie write error: %v", err))
			}
			return block, b.receipts
		}
		return nil, nil
	}
	for i := 0; i < n; i++ {
		statedb, err := state.New(parent.Root(), state.NewDatabase(db), nil)
		if err != nil {
			panic(err)
		}
		block, receipt := genblock(i, parent, statedb)
		blocks[i] = block
		receipts[i] = receipt
		parent = block.Header()
	}
	return blocks, receipts
}

func makeHeader(chain consensus_engine.ChainReader, parent *block.Header, state *state.DB, factory blockfactory.Factory) *block.Header {
	var time *big.Int
	if parent.Time() == nil {
		time = big.NewInt(10)
	} else {
		time = new(big.Int).Add(parent.Time(), big.NewInt(10)) // block time is fixed at 10 seconds
	}

	return factory.NewHeader(parent.Epoch()).With().
		Root(state.IntermediateRoot(chain.Config().IsS3(parent.Epoch()))).
		ParentHash(parent.Hash()).
		Coinbase(parent.Coinbase()).
		GasLimit(core.CalcGasLimit(parent, parent.GasLimit(), parent.GasLimit())).
		Number(new(big.Int).Add(parent.Number(), common.Big1)).
		Time(time).
		Header()
}

type fakeChainReader struct {
	config *params.ChainConfig
}

// Config returns the chain configuration.
func (cr *fakeChainReader) Config() *params.ChainConfig {
	return cr.config
}

func (cr *fakeChainReader) CurrentHeader() *block.Header                            { return nil }
func (cr *fakeChainReader) ShardID() uint32                                         { return 0 }
func (cr *fakeChainReader) GetHeaderByNumber(number uint64) *block.Header           { return nil }
func (cr *fakeChainReader) GetHeaderByHash(hash common.Hash) *block.Header          { return nil }
func (cr *fakeChainReader) GetHeader(hash common.Hash, number uint64) *block.Header { return nil }
func (cr *fakeChainReader) GetBlock(hash common.Hash, number uint64) *types.Block   { return nil }
func (cr *fakeChainReader) GetReceiptsByHash(hash common.Hash) types.Receipts       { return nil }
func (cr *fakeChainReader) ContractCode(hash common.Hash) ([]byte, error)           { return []byte{}, nil }
func (cr *fakeChainReader) ValidatorCode(hash common.Hash) ([]byte, error)          { return []byte{}, nil }
func (cr *fakeChainReader) ReadShardState(epoch *big.Int) (*shard.State, error)     { return nil, nil }
func (cr *fakeChainReader) TrieDB() *trie.Database                                  { return nil }
func (cr *fakeChainReader) TrieNode(hash common.Hash) ([]byte, error)               { return []byte{}, nil }
func (cr *fakeChainReader) ReadValidatorList() ([]common.Address, error)            { return nil, nil }
func (cr *fakeChainReader) ValidatorCandidates() []common.Address                   { return nil }
func (cr *fakeChainReader) SuperCommitteeForNextEpoch(
	beacon consensus_engine.ChainReader, header *block.Header, isVerify bool,
) (*shard.State, error) {
	return nil, nil
}
func (cr *fakeChainReader) ReadValidatorInformation(
	addr common.Address,
) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (cr *fakeChainReader) ReadValidatorInformationAtState(
	addr common.Address, state *state.DB,
) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (cr *fakeChainReader) StateAt(root common.Hash) (*state.DB, error) {
	return nil, nil
}
func (cr *fakeChainReader) Snapshots() *snapshot.Tree {
	return nil
}
func (cr *fakeChainReader) ReadValidatorSnapshot(
	addr common.Address,
) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}
func (cr *fakeChainReader) ReadValidatorSnapshotAtEpoch(
	epoch *big.Int, addr common.Address,
) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}

func (cr *fakeChainReader) ReadBlockRewardAccumulator(
	uint64,
) (*big.Int, error) {
	return nil, nil
}

func (cr *fakeChainReader) CurrentBlock() *types.Block {
	return nil
}

func (cr *fakeChainReader) ValidatorStakingWithDelegation(
	addr common.Address,
) *big.Int {
	return nil
}

func (cr *fakeChainReader) ReadValidatorStats(
	addr common.Address,
) (*staking.ValidatorStats, error) {
	return nil, nil
}

func (cr *fakeChainReader) ReadCommitSig(blockNum uint64) ([]byte, error) { return nil, nil }
