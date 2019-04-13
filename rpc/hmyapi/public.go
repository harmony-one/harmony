package hmyapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
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

// // GetBalance returns the amount of wei for the given address in the state of the
// // given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// // block numbers are also allowed.
// func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
// 	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
// 	if state == nil || err != nil {
// 		return nil, err
// 	}
// 	return (*hexutil.Big)(state.GetBalance(address)), state.Error()
// }

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr uint64, fullTx bool) (*RPCBlock, error) {
	block := s.b.GetBlockByNumber(blockNr)

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
