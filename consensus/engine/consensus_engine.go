package engine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
)

// ChainReader defines a collection of methods needed to access the local
// blockchain during header and/or uncle verification.
// Note this reader interface is still in process of being integrated with the BFT consensus.
type ChainReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// TrieNode retrieves a blob of data associated with a trie node
	// either from ephemeral in-memory cache, or from persistent storage.
	TrieNode(hash common.Hash) ([]byte, error)

	// ContractCode retrieves a blob of data associated with a contract
	// hash either from ephemeral in-memory cache, or from persistent storage.
	//
	// If the code doesn't exist in the in-memory cache, check the storage with
	// new code scheme.
	ContractCode(hash common.Hash) ([]byte, error)

	// ValidatorCode retrieves a blob of data associated with a validator
	// hash either from ephemeral in-memory cache, or from persistent storage.
	//
	// If the code doesn't exist in the in-memory cache, check the storage with
	// new code scheme.
	ValidatorCode(hash common.Hash) ([]byte, error)

	// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
	GetReceiptsByHash(hash common.Hash) types.Receipts

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *block.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *block.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *block.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) *block.Header

	// ShardID returns shardID
	ShardID() uint32

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) *types.Block

	// ReadShardState retrieves sharding state given the epoch number.
	// This api reads the shard state cached or saved on the chaindb.
	// Thus, only should be used to read the shard state of the current chain.
	ReadShardState(epoch *big.Int) (*shard.State, error)

	// ReadValidatorList retrieves the list of all validators
	ReadValidatorList() ([]common.Address, error)

	// Methods needed for EPoS committee assignment calculation
	committee.StakingCandidatesReader
	// Methods for reading right epoch snapshot
	staking.ValidatorSnapshotReader

	//ReadBlockRewardAccumulator is the block-reward given for block number
	ReadBlockRewardAccumulator(uint64) (*big.Int, error)

	// ReadValidatorStats retrieves the running stats for a validator
	ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error)

	// SuperCommitteeForNextEpoch calculates the next epoch's supper committee
	// isVerify flag is to indicate which stage
	// to call this function: true (verification stage), false(propose stage)
	SuperCommitteeForNextEpoch(
		beacon ChainReader, header *block.Header, isVerify bool,
	) (*shard.State, error)

	// ReadCommitSig read the commit sig of a given block number
	ReadCommitSig(blockNum uint64) ([]byte, error)
}

// Engine is an algorithm agnostic consensus engine.
// Note this engine interface is still in process of being integrated with the BFT consensus.
type Engine interface {
	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainReader, header *block.Header, seal bool) error

	// Similiar to VerifyHeader, which is only for verifying the block headers of one's own chain, this verification
	// is used for verifying "incoming" block header against commit signature and bitmap sent from the other chain cross-shard via libp2p.
	// i.e. this header verification api is more flexible since the caller specifies which commit signature and bitmap to use
	// for verifying the block header, which is necessary for cross-shard block header verification. Example of such is cross-shard transaction.
	VerifyHeaderSignature(
		chain ChainReader, header *block.Header, commitSig bls.SerializedSignature, commitBitmap []byte,
	) error

	// VerifyCrossLink verify cross link
	VerifyCrossLink(ChainReader, types.CrossLink) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(
		chain ChainReader, headers []*block.Header, seals []bool,
	) (chan<- struct{}, <-chan error)

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain ChainReader, header *block.Header) error

	// VerifyShardState verifies the shard state during epoch transition is valid
	VerifyShardState(chain ChainReader, beacon ChainReader, header *block.Header) error

	// VerifyVRF verifies the vrf of the block
	VerifyVRF(chain ChainReader, header *block.Header) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// and assembles the final block.
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	// sigsReady signal indicates whether the commit sigs are populated in the header object.
	// Finalize() will block on sigsReady signal until the first value is send to the channel.
	Finalize(
		chain ChainReader,
		beacon ChainReader,
		header *block.Header,
		state *state.DB, txs []*types.Transaction,
		receipts []*types.Receipt, outcxs []*types.CXReceipt,
		incxs []*types.CXReceiptsProof, stks staking.StakingTransactions,
		doubleSigners slash.Records, sigsReady chan bool, viewID func() uint64,
	) (*types.Block, reward.Reader, error)
}
