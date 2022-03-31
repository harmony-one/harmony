package core

import (
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	types2 "github.com/harmony-one/harmony/staking/types"
)

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
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
	ValidateNewBlock(block *types.Block) error
	SetHead(head uint64) error
	ShardID() uint32
	GasLimit() uint64
	CurrentBlock() *types.Block
	CurrentFastBlock() *types.Block
	SetProcessor(processor Processor)
	SetValidator(validator Validator)
	Validator() Validator
	Processor() Processor
	State() (*state.DB, error)
	StateAt(root common.Hash) (*state.DB, error)
	Reset() error
	ResetWithGenesisBlock(genesis *types.Block) error
	Export(w io.Writer) error
	ExportN(w io.Writer, first uint64, last uint64) error
	Genesis() *types.Block
	GetBody(hash common.Hash) *types.Body
	GetBodyRLP(hash common.Hash) rlp.RawValue
	HasBlock(hash common.Hash, number uint64) bool
	HasState(hash common.Hash) bool
	HasBlockAndState(hash common.Hash, number uint64) bool
	GetBlock(hash common.Hash, number uint64) *types.Block
	GetBlockByHash(hash common.Hash) *types.Block
	GetBlockByNumber(number uint64) *types.Block
	GetReceiptsByHash(hash common.Hash) types.Receipts
	GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.Block)
	GetUnclesInChain(b *types.Block, length int) []*block.Header
	TrieNode(hash common.Hash) ([]byte, error)
	Stop()
	Rollback(chain []common.Hash) error
	InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error)
	WriteBlockWithoutState(block *types.Block, td *big.Int) (err error)
	WriteBlockWithState(
		block *types.Block, receipts []*types.Receipt,
		cxReceipts []*types.CXReceipt,
		stakeMsgs []types2.StakeMsg,
		paid reward.Reader,
		state *state.DB,
	) (status WriteStatus, err error)
	GetMaxGarbageCollectedBlockNumber() int64
	InsertChain(chain types.Blocks, verifyHeaders bool) (int, error)
	PostChainEvents(events []interface{}, logs []*types.Log)
	BadBlocks() []BadBlock
	InsertHeaderChain(chain []*block.Header, checkFreq int) (int, error)
	CurrentHeader() *block.Header
	GetTd(hash common.Hash, number uint64) *big.Int
	GetTdByHash(hash common.Hash) *big.Int
	GetHeader(hash common.Hash, number uint64) *block.Header
	GetHeaderByHash(hash common.Hash) *block.Header
	HasHeader(hash common.Hash, number uint64) bool
	GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash
	GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64)
	GetHeaderByNumber(number uint64) *block.Header
	Config() *params.ChainConfig
	Engine() engine.Engine
	SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription
	SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
	ReadShardState(epoch *big.Int) (*shard.State, error)
	WriteShardStateBytes(db rawdb.DatabaseWriter,
		epoch *big.Int, shardState []byte,
	) (*shard.State, error)
	ReadCommitSig(blockNum uint64) ([]byte, error)
	WriteCommitSig(blockNum uint64, lastCommits []byte) error
	GetVdfByNumber(number uint64) []byte
	GetVrfByNumber(number uint64) []byte
	ChainDb() ethdb.Database
	GetEpochBlockNumber(epoch *big.Int) (*big.Int, error)
	StoreEpochBlockNumber(
		epoch *big.Int, blockNum *big.Int,
	) error
	ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error)
	WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error
	ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error)
	WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error
	WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error
	DeleteCrossLinks(cls []types.CrossLink) error
	ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error)
	LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error
	ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error)
	DeleteFromPendingSlashingCandidates(
		processed slash.Records,
	) error
	ReadPendingSlashingCandidates() slash.Records
	ReadPendingCrossLinks() ([]types.CrossLink, error)
	CachePendingCrossLinks(crossLinks []types.CrossLink) error
	SavePendingCrossLinks() error
	AddPendingSlashingCandidates(
		candidates slash.Records,
	) error
	AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error)
	DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error)
	IsSameLeaderAsPreviousBlock(block *types.Block) bool
	GetVMConfig() *vm.Config
	ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error)
	CXMerkleProof(toShardID uint32, block *types.Block) (*types.CXMerkleProof, error)
	WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error
	IsSpent(cxp *types.CXReceiptsProof) bool
	ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64)
	ReadValidatorInformationAtRoot(
		addr common.Address, root common.Hash,
	) (*types2.ValidatorWrapper, error)
	ReadValidatorInformationAtState(
		addr common.Address, state *state.DB,
	) (*types2.ValidatorWrapper, error)
	ReadValidatorInformation(
		addr common.Address,
	) (*types2.ValidatorWrapper, error)
	ReadValidatorSnapshotAtEpoch(
		epoch *big.Int,
		addr common.Address,
	) (*types2.ValidatorSnapshot, error)
	ReadValidatorSnapshot(
		addr common.Address,
	) (*types2.ValidatorSnapshot, error)
	WriteValidatorSnapshot(
		batch rawdb.DatabaseWriter, snapshot *types2.ValidatorSnapshot,
	) error
	ReadValidatorStats(
		addr common.Address,
	) (*types2.ValidatorStats, error)
	UpdateValidatorVotingPower(
		batch rawdb.DatabaseWriter,
		block *types.Block,
		newEpochSuperCommittee, currentEpochSuperCommittee *shard.State,
		state *state.DB,
	) (map[common.Address]*types2.ValidatorStats, error)
	ComputeAndUpdateAPR(
		block *types.Block, now *big.Int,
		wrapper *types2.ValidatorWrapper, stats *types2.ValidatorStats,
	) error
	UpdateValidatorSnapshots(
		batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address,
	) error
	ReadValidatorList() ([]common.Address, error)
	WriteValidatorList(
		db rawdb.DatabaseWriter, addrs []common.Address,
	) error
	ReadDelegationsByDelegator(
		delegator common.Address,
	) (m types2.DelegationIndexes, err error)
	ReadDelegationsByDelegatorAt(
		delegator common.Address, blockNum *big.Int,
	) (m types2.DelegationIndexes, err error)
	UpdateStakingMetaData(
		batch rawdb.DatabaseWriter, block *types.Block,
		stakeMsgs []types2.StakeMsg,
		state *state.DB, epoch, newEpoch *big.Int,
	) (newValidators []common.Address, err error)
	ReadBlockRewardAccumulator(number uint64) (*big.Int, error)
	WriteBlockRewardAccumulator(
		batch rawdb.DatabaseWriter, reward *big.Int, number uint64,
	) error
	UpdateBlockRewardAccumulator(
		batch rawdb.DatabaseWriter, diff *big.Int, number uint64,
	) error
	ValidatorCandidates() []common.Address
	DelegatorsInformation(addr common.Address) []*types2.Delegation
	GetECDSAFromCoinbase(header *block.Header) (common.Address, error)
	SuperCommitteeForNextEpoch(
		beacon engine.ChainReader,
		header *block.Header,
		isVerify bool,
	) (*shard.State, error)
	EnablePruneBeaconChainFeature()
	IsEnablePruneBeaconChainFeature() bool
	CommitOffChainData(
		batch rawdb.DatabaseWriter,
		block *types.Block,
		receipts []*types.Receipt,
		cxReceipts []*types.CXReceipt,
		stakeMsgs []types2.StakeMsg,
		payout reward.Reader,
		state *state.DB,
	) (status WriteStatus, err error)
}
