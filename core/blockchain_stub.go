package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
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
	staking "github.com/harmony-one/harmony/staking/types"
)

// Smoke test.
var _ BlockChain = Stub{}

type Stub struct {
	Name string
}

func (a Stub) ValidateNewBlock(block *types.Block) error {
	panic("not implemented")
}

func (a Stub) SetHead(head uint64) error {
	panic("not implemented")
}

func (a Stub) ShardID() uint32 {
	panic("not implemented")
}

func (a Stub) CurrentBlock() *types.Block {
	panic("not implemented")
}

func (a Stub) Validator() Validator {
	panic("not implemented")
}

func (a Stub) Processor() Processor {
	panic("not implemented")
}

func (a Stub) State() (*state.DB, error) {
	panic("not implemented")
}

func (a Stub) StateAt(common.Hash) (*state.DB, error) {
	panic("not implemented")
}

func (a Stub) HasBlock(hash common.Hash, number uint64) bool {
	panic("not implemented")
}

func (a Stub) HasState(hash common.Hash) bool {
	panic("not implemented")
}

func (a Stub) HasBlockAndState(hash common.Hash, number uint64) bool {
	panic("not implemented")
}

func (a Stub) GetBlock(hash common.Hash, number uint64) *types.Block {
	panic("not implemented")
}

func (a Stub) GetBlockByHash(hash common.Hash) *types.Block {
	panic("not implemented")
}

func (a Stub) GetBlockByNumber(number uint64) *types.Block {
	panic("not implemented")
}

func (a Stub) GetReceiptsByHash(hash common.Hash) types.Receipts {
	panic("not implemented")
}

func (a Stub) Stop() {
}

func (a Stub) Rollback(chain []common.Hash) error {
	panic("not implemented")
}

func (a Stub) WriteBlockWithoutState(block *types.Block, td *big.Int) (err error) {
	panic("not implemented")
}

func (a Stub) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []staking.StakeMsg, paid reward.Reader, state *state.DB) (status WriteStatus, err error) {
	panic("not implemented")
}

func (a Stub) GetMaxGarbageCollectedBlockNumber() int64 {
	panic("not implemented")
}

func (a Stub) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	panic("not implemented")
}

func (a Stub) BadBlocks() []BadBlock {
	panic("not implemented")
}

func (a Stub) CurrentHeader() *block.Header {
	panic("not implemented")
}

func (a Stub) GetHeader(hash common.Hash, number uint64) *block.Header {
	panic("not implemented")
}

func (a Stub) GetHeaderByHash(hash common.Hash) *block.Header {
	panic("not implemented")
}

func (a Stub) GetCanonicalHash(number uint64) common.Hash {
	panic("not implemented")
}

func (a Stub) GetHeaderByNumber(number uint64) *block.Header {
	panic("not implemented")
}

func (a Stub) Config() *params.ChainConfig {
	panic("not implemented")
}

func (a Stub) Engine() engine.Engine {
	panic("not implemented")
}

func (a Stub) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	panic("not implemented")
}

func (a Stub) SubscribeTraceEvent(ch chan<- TraceEvent) event.Subscription {
	panic("not implemented")
}

func (a Stub) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	panic("not implemented")
}

func (a Stub) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	panic("not implemented")
}

func (a Stub) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	panic("not implemented")
}

func (a Stub) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	panic("not implemented")
}

func (a Stub) ReadShardState(epoch *big.Int) (*shard.State, error) {
	panic("not implemented")
}

func (a Stub) WriteShardStateBytes(db rawdb.DatabaseWriter, epoch *big.Int, shardState []byte) (*shard.State, error) {
	panic("not implemented")
}

func (a Stub) WriteHeadBlock(block *types.Block) error {
	panic("not implemented")
}

func (a Stub) ReadCommitSig(blockNum uint64) ([]byte, error) {
	panic("not implemented")
}

func (a Stub) WriteCommitSig(blockNum uint64, lastCommits []byte) error {
	panic("not implemented")
}

func (a Stub) GetVdfByNumber(number uint64) []byte {
	panic("not implemented")
}

func (a Stub) GetVrfByNumber(number uint64) []byte {
	panic("not implemented")
}

func (a Stub) ChainDb() ethdb.Database {
	panic("not implemented")
}

func (a Stub) GetEpochBlockNumber(epoch *big.Int) (*big.Int, error) {
	panic("not implemented")
}

func (a Stub) StoreEpochBlockNumber(epoch *big.Int, blockNum *big.Int) error {
	panic("not implemented")
}

func (a Stub) ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error) {
	panic("not implemented")
}

func (a Stub) WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error {
	panic("not implemented")
}

func (a Stub) ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error) {
	panic("not implemented")
}

func (a Stub) WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error {
	panic("not implemented")
}

func (a Stub) WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error {
	panic("not implemented")
}

func (a Stub) DeleteCrossLinks(cls []types.CrossLink) error {
	panic("not implemented")
}

func (a Stub) ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	panic("not implemented")
}

func (a Stub) LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error {
	panic("not implemented")
}

func (a Stub) ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error) {
	panic("not implemented")
}

func (a Stub) DeleteFromPendingSlashingCandidates(processed slash.Records) error {
	panic("not implemented")
}

func (a Stub) ReadPendingSlashingCandidates() slash.Records {
	panic("not implemented")
}

func (a Stub) ReadPendingCrossLinks() ([]types.CrossLink, error) {
	panic("not implemented")
}

func (a Stub) CachePendingCrossLinks(crossLinks []types.CrossLink) error {
	panic("not implemented")
}

func (a Stub) SavePendingCrossLinks() error {
	panic("not implemented")
}

func (a Stub) AddPendingSlashingCandidates(candidates slash.Records) error {
	panic("not implemented")
}

func (a Stub) AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error) {
	panic("not implemented")
}

func (a Stub) DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error) {
	panic("not implemented")
}

func (a Stub) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	panic("not implemented")
}

func (a Stub) GetVMConfig() *vm.Config {
	panic("not implemented")
}

func (a Stub) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error) {
	panic("not implemented")
}

func (a Stub) CXMerkleProof(toShardID uint32, block *types.Block) (*types.CXMerkleProof, error) {
	panic("not implemented")
}

func (a Stub) WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error {
	panic("not implemented")
}

func (a Stub) IsSpent(cxp *types.CXReceiptsProof) bool {
	panic("not implemented")
}

func (a Stub) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	panic("not implemented")
}

func (a Stub) ReadValidatorInformationAtRoot(addr common.Address, root common.Hash) (*staking.ValidatorWrapper, error) {
	panic("not implemented")
}

func (a Stub) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	panic("not implemented")
}

func (a Stub) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	panic("not implemented")
}

func (a Stub) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*staking.ValidatorSnapshot, error) {
	panic("not implemented")
}

func (a Stub) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	panic("not implemented")
}

func (a Stub) WriteValidatorSnapshot(batch rawdb.DatabaseWriter, snapshot *staking.ValidatorSnapshot) error {
	panic("not implemented")
}

func (a Stub) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	panic("not implemented")
}

func (a Stub) UpdateValidatorVotingPower(batch rawdb.DatabaseWriter, block *types.Block, newEpochSuperCommittee, currentEpochSuperCommittee *shard.State, state *state.DB) (map[common.Address]*staking.ValidatorStats, error) {
	panic("not implemented")
}

func (a Stub) ComputeAndUpdateAPR(block *types.Block, now *big.Int, wrapper *staking.ValidatorWrapper, stats *staking.ValidatorStats) error {
	panic("not implemented")
}

func (a Stub) UpdateValidatorSnapshots(batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address) error {
	panic("not implemented")
}

func (a Stub) ReadValidatorList() ([]common.Address, error) {
	panic("not implemented")
}

func (a Stub) WriteValidatorList(db rawdb.DatabaseWriter, addrs []common.Address) error {
	panic("not implemented")
}

func (a Stub) ReadDelegationsByDelegator(delegator common.Address) (m staking.DelegationIndexes, err error) {
	panic("not implemented")
}

func (a Stub) ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (m staking.DelegationIndexes, err error) {
	panic("not implemented")
}

func (a Stub) UpdateStakingMetaData(batch rawdb.DatabaseWriter, block *types.Block, stakeMsgs []staking.StakeMsg, state *state.DB, epoch, newEpoch *big.Int) (newValidators []common.Address, err error) {
	panic("not implemented")
}

func (a Stub) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	panic("not implemented")
}

func (a Stub) WriteBlockRewardAccumulator(batch rawdb.DatabaseWriter, reward *big.Int, number uint64) error {
	panic("not implemented")
}

func (a Stub) UpdateBlockRewardAccumulator(batch rawdb.DatabaseWriter, diff *big.Int, number uint64) error {
	panic("not implemented")
}

func (a Stub) ValidatorCandidates() []common.Address {
	panic("not implemented")
}

func (a Stub) DelegatorsInformation(addr common.Address) []*staking.Delegation {
	panic("not implemented")
}

func (a Stub) GetECDSAFromCoinbase(header *block.Header) (common.Address, error) {
	panic("not implemented")
}

func (a Stub) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	panic("not implemented")
}

func (a Stub) EnablePruneBeaconChainFeature() {
}

func (a Stub) IsEnablePruneBeaconChainFeature() bool {
	panic("not implemented")
}

func (a Stub) CommitOffChainData(batch rawdb.DatabaseWriter, block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []staking.StakeMsg, payout reward.Reader, state *state.DB) (status WriteStatus, err error) {
	panic("not implemented")
}
