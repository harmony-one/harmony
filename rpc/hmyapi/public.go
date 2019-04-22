package hmyapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rpc"
)

// PublicBlockChainAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b *core.BlockChain
}

// NewPublicBlockChainAPI creates a new Ethereum blockchain API.
func NewPublicBlockChainAPI(b *core.BlockChain) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (*RPCBlock, error) {
	block := s.b.GetBlockByNumber(uint64(blockNr))

	if block == nil {
		return nil, nil
	}

	return RPCMarshalBlock(block, false, false)
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (*RPCBlock, error) {
	block := s.b.GetBlockByHash(blockHash)
	if block == nil {
		return nil, nil
	}
	return RPCMarshalBlock(block, false, false)
}

// newRPCTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockHash(b *types.Block, hash common.Hash) *RPCTransaction {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64) *RPCTransaction {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.Hash(), b.NumberU64(), index)
}

// PublicHarmonyAPI provides an API to access Harmony related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicHarmonyAPI struct {
	b *core.BlockChain
}

// TODO(ricl): update the following two functions.
// ProtocolVersion returns the current Harmony protocol version this node supports
func (s *PublicHarmonyAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(1)
	// return hexutil.Uint(s.b.ProtocolVersion())
}

// Syncing returns false in case the node is currently not syncing with the network. It can be up to date or has not
// yet received the latest block headers from its pears. In case it is synchronizing:
// - startingBlock: block number this node started to synchronise from
// - currentBlock:  block number this node is currently importing
// - highestBlock:  block number of the highest block header this node has received from peers
// - pulledStates:  number of state entries processed until now
// - knownStates:   number of known state entries that still need to be pulled
func (s *PublicHarmonyAPI) Syncing() (interface{}, error) {
	return false, nil
	// progress := s.b.Downloader().Progress()

	// // Return not syncing if the synchronisation already completed
	// if progress.CurrentBlock >= progress.HighestBlock {
	// 	return false, nil
	// }
	// // Otherwise gather the block sync stats
	// return map[string]interface{}{
	// 	"startingBlock": hexutil.Uint64(progress.StartingBlock),
	// 	"currentBlock":  hexutil.Uint64(progress.CurrentBlock),
	// 	"highestBlock":  hexutil.Uint64(progress.HighestBlock),
	// 	"pulledStates":  hexutil.Uint64(progress.PulledStates),
	// 	"knownStates":   hexutil.Uint64(progress.KnownStates),
	// }, nil
}

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	net            *p2p.Server
	networkVersion uint64
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net *p2p.Server, networkVersion uint64) *PublicNetAPI {
	return &PublicNetAPI{net, networkVersion}
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
	return hexutil.Uint(s.net.PeerCount())
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         *core.BlockChain
	txPool    *core.TxPool
	nonceLock *AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b *core.BlockChain, nonceLock *AddrLocker, txPool *core.TxPool) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, txPool, nonceLock}
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	block := s.b.GetBlockByNumber(uint64(blockNr))
	n := hexutil.Uint(block.Transactions().Len())
	return &n
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	block := s.b.GetBlockByHash(blockHash)
	n := hexutil.Uint(block.Transactions().Len())
	return &n
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) *RPCTransaction {
	block := s.b.GetBlockByNumber(uint64(blockNr))
	return newRPCTransactionFromBlockIndex(block, uint64(index))
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) *RPCTransaction {
	block := s.b.GetBlockByHash(blockHash)
	return newRPCTransactionFromBlockIndex(block, uint64(index))
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *RPCTransaction {
	// Try to return an already finalized transaction
	if tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash); tx != nil {
		return newRPCTransaction(tx, blockHash, blockNumber, index)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.txPool.Get(hash); tx != nil {
		return newRPCPendingTransaction(tx)
	}
	// Transaction unknown, return as such
	return nil
}
