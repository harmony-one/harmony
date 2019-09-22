package engine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"
)

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
// Note this reader interface is still in process of being integrated with the BFT consensus.
type ChainReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *block.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *block.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *block.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) *block.Header

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) *types.Block

	// ReadShardState retrieves sharding state given the epoch number.
	// This api reads the shard state cached or saved on the chaindb.
	// Thus, only should be used to read the shard state of the current chain.
	ReadShardState(epoch *big.Int) (shard.State, error)
}

// Engine is an algorithm agnostic consensus engine.
// Note this engine interface is still in process of being integrated with the BFT consensus.
type Engine interface {
	// Author retrieves the Harmony address of the account that validated the given
	// block.
	Author(header *block.Header) (common.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainReader, header *block.Header, seal bool) error

	// Similiar to VerifyHeader, which is only for verifying the block headers of one's own chain, this verification
	// is used for verifying "incoming" block header against commit signature and bitmap sent from the other chain cross-shard via libp2p.
	// i.e. this header verification api is more flexible since the caller specifies which commit signature and bitmap to use
	// for verifying the block header, which is necessary for cross-shard block header verification. Example of such is cross-shard transaction.
	// (TODO) For now, when doing cross shard, we need recalcualte the shard state since we don't have shard state of other shards
	VerifyHeaderWithSignature(chain ChainReader, header *block.Header, commitSig []byte, commitBitmap []byte, reCalculate bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainReader, headers []*block.Header, seals []bool) (chan<- struct{}, <-chan error)

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain ChainReader, header *block.Header) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainReader, header *block.Header) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// and assembles the final block.
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	Finalize(chain ChainReader, header *block.Header, state *state.DB, txs []*types.Transaction,
		receipts []*types.Receipt, outcxs []*types.CXReceipt, incxs []*types.CXReceiptsProof) (*types.Block, error)

	// Seal generates a new sealing request for the given input block and pushes
	// the result into the given channel.
	//
	// Note, the method returns immediately and will send the result async. More
	// than one result may also be returned depending on the consensus algorithm.
	Seal(chain ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error

	// SealHash returns the hash of a block prior to it being sealed.
	SealHash(header *block.Header) common.Hash
}
