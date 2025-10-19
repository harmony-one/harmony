package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/tikv/redis_helper"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	types2 "github.com/harmony-one/harmony/staking/types"
)

// Options contains configuration values to change blockchain behaviour.
type Options struct {
	// Subset of blockchain suitable for storing last epoch blocks i.e. blocks with shard state.
	EpochChain bool
}

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the blockchain happens according to the set of rules
// defined by the two stage validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the validator. Failing results in aborting of
// the import.
//
// The BlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type BlockChain interface {
	// ValidateNewBlock validates new block.
	ValidateNewBlock(block *types.Block, beaconChain BlockChain) error
	// ShardID returns the shard Id of the blockchain.
	ShardID() uint32
	// CurrentBlock retrieves the current head block of the canonical chain. The
	// block is retrieved from the blockchain's internal cache.
	CurrentBlock() *types.Block
	// CurrentFastBlock retrieves the current fast-sync head block of the canonical
	// block is retrieved from the blockchain's internal cache.
	CurrentFastBlock() *types.Block
	// Validator returns the current validator.
	Validator() Validator
	// Processor returns the current processor.
	Processor() Processor
	// State returns a new mutable state based on the current HEAD block.
	State() (*state.DB, error)
	// StateAt returns a new mutable state based on a particular point in time.
	StateAt(root common.Hash) (*state.DB, error)
	// Snapshots returns the blockchain snapshot tree.
	Snapshots() *snapshot.Tree
	// TrieDB returns trie database
	TrieDB() *trie.Database
	// HasBlock checks if a block is fully present in the database or not.
	HasBlock(hash common.Hash, number uint64) bool
	// HasState checks if state trie is fully present in the database or not.
	HasState(hash common.Hash) bool
	// HasBlockAndState checks if a block and associated state trie is fully present
	// in the database or not, caching it if present.
	HasBlockAndState(hash common.Hash, number uint64) bool
	// GetBlock retrieves a block from the database by hash and number,
	// caching it if found.
	GetBlock(hash common.Hash, number uint64) *types.Block
	// GetBlockByHash retrieves a block from the database by hash, caching it if found.
	GetBlockByHash(hash common.Hash) *types.Block
	// GetBlockByNumber retrieves a block from the database by number, caching it
	// (associated with its hash) if found.
	GetBlockByNumber(number uint64) *types.Block
	// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
	GetReceiptsByHash(hash common.Hash) types.Receipts
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
	// Stop stops the blockchain service. If any imports are currently in progress
	// it will abort them using the procInterrupt.
	Stop()
	// Rollback is designed to remove a chain of links from the database that aren't
	// certain enough to be valid.
	Rollback(chain []common.Hash) error
	// writeHeadBlock writes a new head block
	WriteHeadBlock(block *types.Block) error
	// WriteBlockWithoutState writes only the block and its metadata to the database,
	// but does not write any state. This is used to construct competing side forks
	// up to the point where they exceed the canonical total difficulty.
	WriteBlockWithoutState(block *types.Block) (err error)
	// WriteBlockWithState writes the block and all associated state to the database.
	WriteBlockWithState(
		block *types.Block, receipts []*types.Receipt,
		cxReceipts []*types.CXReceipt,
		stakeMsgs []types2.StakeMsg,
		paid reward.Reader,
		state *state.DB,
	) (status WriteStatus, err error)
	// GetMaxGarbageCollectedBlockNumber ..
	GetMaxGarbageCollectedBlockNumber() int64
	// InsertChain attempts to insert the given batch of blocks in to the canonical
	// chain or, otherwise, create a fork. If an error is returned it will return
	// the index number of the failing block as well an error describing what went
	// wrong.
	//
	// After insertion is done, all accumulated events will be fired.
	InsertChain(chain types.Blocks, verifyHeaders bool) (int, error)
	// InsertReceiptChain attempts to complete an already existing header chain with
	// transaction and receipt data.
	InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error)
	// LeaderRotationMeta returns the number of continuous blocks by the leader.
	LeaderRotationMeta() LeaderRotationMeta
	// BadBlocks returns a list of the last 'bad blocks' that
	// the client has seen on the network.
	BadBlocks() []BadBlock
	// CurrentHeader retrieves the current head header of the canonical chain. The
	// header is retrieved from the HeaderChain's internal cache.
	CurrentHeader() *block.Header
	// GetHeader retrieves a block header from the database by hash and number,
	// caching it if found.
	GetHeader(hash common.Hash, number uint64) *block.Header
	// GetHeaderByHash retrieves a block header from the database by hash, caching it if
	// found.
	GetHeaderByHash(hash common.Hash) *block.Header
	// GetCanonicalHash returns the canonical hash for a given block number.
	GetCanonicalHash(number uint64) common.Hash
	// GetHeaderByNumber retrieves a block header from the database by number,
	// caching it (associated with its hash) if found.
	GetHeaderByNumber(number uint64) *block.Header
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig
	// Engine retrieves the blockchain's consensus engine.
	Engine() engine.Engine
	// SubscribeRemovedLogsEvent registers a subscription of RemovedLogsEvent.
	SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription
	// SubscribeTraceEvent registers a subscription of ChainEvent.
	SubscribeTraceEvent(ch chan<- TraceEvent) event.Subscription
	// SubscribeChainEvent registers a subscription of ChainEvent.
	SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription
	// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
	// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
	SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription
	// SubscribeLogsEvent registers a subscription of []*types.Log.
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
	// ReadShardState retrieves sharding state given the epoch number.
	ReadShardState(epoch *big.Int) (*shard.State, error)
	// WriteShardStateBytes saves the given sharding state under the given epoch number.
	WriteShardStateBytes(db rawdb.DatabaseWriter,
		epoch *big.Int, shardState []byte,
	) (*shard.State, error)
	// ReadCommitSig retrieves the commit signature on a block.
	ReadCommitSig(blockNum uint64) ([]byte, error)
	// WriteCommitSig saves the commits signatures signed on a block.
	WriteCommitSig(blockNum uint64, lastCommits []byte) error
	// GetVdfByNumber retrieves the rand seed given the block number, return 0 if not exist.
	GetVdfByNumber(number uint64) []byte
	// GetVrfByNumber retrieves the randomness preimage given the block number, return 0 if not exist
	GetVrfByNumber(number uint64) []byte
	// ChainDb returns the database.
	ChainDb() ethdb.Database
	// ReadEpochVrfBlockNums retrieves block numbers with valid VRF for the specified epoch.
	ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error)
	// WriteCrossLinks saves the hashes of crosslinks by shardID and blockNum combination key.
	WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error
	// DeleteCrossLinks removes the hashes of crosslinks by shardID and blockNum combination key.
	DeleteCrossLinks(cls []types.CrossLink) error
	// ReadCrossLink retrieves crosslink given shardID and blockNum.
	ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error)
	// LastContinuousCrossLink saves the last crosslink of a shard
	// This function will update the latest crosslink in the sense that
	// any previous block's crosslink is received up to this point
	// there is no missing hole between genesis to this crosslink of given shardID.
	LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error
	// ReadShardLastCrossLink retrieves the last crosslink of a shard.
	ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error)
	// DeleteFromPendingSlashingCandidates ..
	DeleteFromPendingSlashingCandidates(
		processed slash.Records,
	) error
	// ReadPendingSlashingCandidates retrieves pending slashing candidates.
	ReadPendingSlashingCandidates() slash.Records
	// ReadPendingCrossLinks retrieves pending crosslinks.
	ReadPendingCrossLinks() ([]types.CrossLink, error)
	// CachePendingCrossLinks caches the pending crosslinks in memory.
	CachePendingCrossLinks(crossLinks []types.CrossLink) error
	// SavePendingCrossLinks saves the pending crosslinks in db.
	SavePendingCrossLinks() error
	// AddPendingSlashingCandidates appends pending slashing candidates.
	AddPendingSlashingCandidates(
		candidates slash.Records,
	) error
	// AddPendingCrossLinks appends pending crosslinks.
	AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error)
	// DeleteFromPendingCrossLinks delete pending crosslinks that already committed (i.e. passed in the params).
	DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error)
	// IsSameLeaderAsPreviousBlock retrieves a block from the database by number, caching it.
	IsSameLeaderAsPreviousBlock(block *types.Block) bool
	// GetVMConfig returns the blockchain VM config.
	GetVMConfig() *vm.Config
	// ReadCXReceipts retrieves the cross shard transaction receipts of a given shard.
	ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error)
	// CXMerkleProof calculates the cross shard transaction merkle proof of a given destination shard.
	CXMerkleProof(toShardID uint32, block *block.Header) (*types.CXMerkleProof, error)
	// WriteCXReceiptsProofSpent mark the CXReceiptsProof list with given unspent status
	WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error
	// IsSpent checks whether a CXReceiptsProof is spent.
	IsSpent(cxp *types.CXReceiptsProof) bool
	// ReadTxLookupEntry returns where the given transaction resides in the chain,
	// as a (block hash, block number, index in transaction list) triple.
	// returns 0, 0 if not found.
	ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64)
	// ReadValidatorInformationAtRoot reads staking
	// information of given validatorWrapper at a specific state root.
	ReadValidatorInformationAtRoot(
		addr common.Address, root common.Hash,
	) (*types2.ValidatorWrapper, error)
	// ReadValidatorInformationAtState reads staking
	// information of given validatorWrapper at a specific state root.
	ReadValidatorInformationAtState(
		addr common.Address, state *state.DB,
	) (*types2.ValidatorWrapper, error)
	// ReadValidatorInformation reads staking information of given validator address.
	ReadValidatorInformation(
		addr common.Address,
	) (*types2.ValidatorWrapper, error)
	// ReadValidatorSnapshotAtEpoch reads the snapshot
	// staking validator information of given validator address.
	ReadValidatorSnapshotAtEpoch(
		epoch *big.Int,
		addr common.Address,
	) (*types2.ValidatorSnapshot, error)
	// ReadValidatorSnapshot reads the snapshot staking information of given validator address.
	ReadValidatorSnapshot(
		addr common.Address,
	) (*types2.ValidatorSnapshot, error)
	// WriteValidatorSnapshot writes the snapshot of provided validator.
	WriteValidatorSnapshot(
		batch rawdb.DatabaseWriter, snapshot *types2.ValidatorSnapshot,
	) error
	// ReadValidatorStats reads the stats of a validator.
	ReadValidatorStats(
		addr common.Address,
	) (*types2.ValidatorStats, error)
	// ComputeAndUpdateAPR ...
	ComputeAndUpdateAPR(
		block *types.Block, now *big.Int,
		wrapper *types2.ValidatorWrapper, stats *types2.ValidatorStats,
	) error
	// UpdateValidatorSnapshots updates the content snapshot of all validators
	// Note: this should only be called within the blockchain insert process.
	UpdateValidatorSnapshots(
		batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address,
	) error
	// ReadValidatorList reads the addresses of current all validators.
	ReadValidatorList() ([]common.Address, error)
	// WriteValidatorList writes the list of validator addresses to database
	// Note: this should only be called within the blockchain insert process.
	WriteValidatorList(
		db rawdb.DatabaseWriter, addrs []common.Address,
	) error
	// ReadDelegationsByDelegator reads the addresses of validators delegated by a delegator.
	ReadDelegationsByDelegator(
		delegator common.Address,
	) (m types2.DelegationIndexes, err error)
	// ReadDelegationsByDelegatorAt reads the addresses of validators delegated by a delegator at a given block.
	ReadDelegationsByDelegatorAt(
		delegator common.Address, blockNum *big.Int,
	) (m types2.DelegationIndexes, err error)
	// UpdateStakingMetaData updates the metadata of validators and delegations,
	// including the full validator list and delegation indexes.
	// Note: this should only be called within the blockchain insert process.
	UpdateStakingMetaData(
		batch rawdb.DatabaseWriter, block *types.Block,
		stakeMsgs []types2.StakeMsg,
		state *state.DB, epoch, newEpoch *big.Int,
	) (newValidators []common.Address, err error)
	// ReadBlockRewardAccumulator must only be called on beaconchain
	// Note that block rewards are only for staking era.
	ReadBlockRewardAccumulator(number uint64) (*big.Int, error)
	// WriteBlockRewardAccumulator directly writes the BlockRewardAccumulator value
	// Note: this should only be called once during staking launch.
	WriteBlockRewardAccumulator(
		batch rawdb.DatabaseWriter, reward *big.Int, number uint64,
	) error
	// UpdateBlockRewardAccumulator ..
	// Note: this should only be called within the blockchain insert process.
	UpdateBlockRewardAccumulator(
		batch rawdb.DatabaseWriter, diff *big.Int, number uint64,
	) error
	// ValidatorCandidates returns the up to date validator candidates for next epoch.
	ValidatorCandidates() []common.Address
	// DelegatorsInformation returns up to date information of delegators of a given validator address.
	DelegatorsInformation(addr common.Address) []*types2.Delegation
	// GetECDSAFromCoinbase retrieve corresponding ecdsa address from Coinbase Address.
	GetECDSAFromCoinbase(header *block.Header) (common.Address, error)
	// SuperCommitteeForNextEpoch ...
	// isVerify=true means validators use it to verify
	// isVerify=false means leader is to propose.
	SuperCommitteeForNextEpoch(
		beacon engine.ChainReader,
		header *block.Header,
		isVerify bool,
	) (*shard.State, error)
	// EnablePruneBeaconChainFeature enabled prune BeaconChain feature.
	EnablePruneBeaconChainFeature()
	// IsEnablePruneBeaconChainFeature returns is enable prune BeaconChain feature.
	IsEnablePruneBeaconChainFeature() bool
	// CommitOffChainData write off chain data of a block onto db writer.
	CommitOffChainData(
		batch rawdb.DatabaseWriter,
		block *types.Block,
		receipts []*types.Receipt,
		cxReceipts []*types.CXReceipt,
		stakeMsgs []types2.StakeMsg,
		payout reward.Reader,
		state *state.DB,
	) (status WriteStatus, err error)

	GetLeaderPubKeyFromCoinbase(h *block.Header) (*bls.PublicKeyWrapper, error)
	CommitPreimages() error
	GetStateCache() state.Database
	GetSnapshotTrie() *snapshot.Tree

	// ========== Only For Tikv Start ==========

	// return true if is tikv writer master
	IsTikvWriterMaster() bool
	// RedisPreempt used for tikv mode, get the redis preempt instance
	RedisPreempt() *redis_helper.RedisPreempt
	// SyncFromTiKVWriter used for tikv mode, all reader or follower writer used to sync block from master writer
	SyncFromTiKVWriter(newBlkNum uint64, logs []*types.Log) error
	// InitTiKV used for tikv mode, init the tikv mode
	InitTiKV(conf *harmonyconfig.TiKVConfig)

	// ========== Only For Tikv End ==========
}
