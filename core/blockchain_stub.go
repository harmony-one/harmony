package core

import (
	"math/big"

	"github.com/harmony-one/harmony/core/state/snapshot"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/tikv/redis_helper"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Smoke test.
var _ BlockChain = Stub{}

type Stub struct {
	Name string
}

func (a Stub) ValidateNewBlock(block *types.Block, beaconChain BlockChain) error {
	return errors.Errorf("method ValidateNewBlock not implemented for %s", a.Name)
}

func (a Stub) SetHead(head uint64) error {
	return errors.Errorf("method SetHead not implemented for %s", a.Name)
}

func (a Stub) ShardID() uint32 {
	return 0
}

func (a Stub) CurrentBlock() *types.Block {
	return nil
}

func (a Stub) CurrentFastBlock() *types.Block {
	return nil
}

func (a Stub) Validator() Validator {
	return nil
}

func (a Stub) Processor() Processor {
	return nil
}

func (a Stub) State() (*state.DB, error) {
	return nil, errors.Errorf("method State not implemented for %s", a.Name)
}

func (a Stub) StateAt(common.Hash) (*state.DB, error) {
	return nil, errors.Errorf("method StateAt not implemented for %s", a.Name)
}

func (a Stub) Snapshots() *snapshot.Tree {
	return nil
}

func (a Stub) TrieDB() *trie.Database {
	return nil
}

func (a Stub) TrieNode(hash common.Hash) ([]byte, error) {
	return []byte{}, errors.Errorf("method TrieNode not implemented for %s", a.Name)
}

func (a Stub) HasBlock(hash common.Hash, number uint64) bool {
	return false
}

func (a Stub) HasState(hash common.Hash) bool {
	return false
}

func (a Stub) HasBlockAndState(hash common.Hash, number uint64) bool {
	return false
}

func (a Stub) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}

func (a Stub) GetBlockByHash(hash common.Hash) *types.Block {
	return nil
}

func (a Stub) GetBlockByNumber(number uint64) *types.Block {
	return nil
}

func (a Stub) GetReceiptsByHash(hash common.Hash) types.Receipts {
	return nil
}

func (a Stub) ContractCode(hash common.Hash) ([]byte, error) {
	return []byte{}, nil
}

func (a Stub) ValidatorCode(hash common.Hash) ([]byte, error) {
	return []byte{}, nil
}

func (a Stub) Stop() {
}

func (a Stub) Rollback(chain []common.Hash) error {
	return errors.Errorf("method Rollback not implemented for %s", a.Name)
}

func (a Stub) WriteBlockWithoutState(block *types.Block) (err error) {
	return errors.Errorf("method WriteBlockWithoutState not implemented for %s", a.Name)
}

func (a Stub) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []staking.StakeMsg, paid reward.Reader, state *state.DB) (status WriteStatus, err error) {
	return 0, errors.Errorf("method WriteBlockWithState not implemented for %s", a.Name)
}

func (a Stub) GetMaxGarbageCollectedBlockNumber() int64 {
	return 0
}

func (a Stub) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	return 0, errors.Errorf("method InsertChain not implemented for %s", a.Name)
}

func (a Stub) InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error) {
	return 0, errors.Errorf("method InsertReceiptChain not implemented for %s", a.Name)
}

func (a Stub) BadBlocks() []BadBlock {
	return nil
}

func (a Stub) CurrentHeader() *block.Header {
	return nil
}

func (a Stub) GetHeader(hash common.Hash, number uint64) *block.Header {
	return nil
}

func (a Stub) GetHeaderByHash(hash common.Hash) *block.Header {
	return nil
}

func (a Stub) GetCanonicalHash(number uint64) common.Hash {
	return common.Hash{}
}

func (a Stub) GetHeaderByNumber(number uint64) *block.Header {
	return nil
}

func (a Stub) Config() *params.ChainConfig {
	return nil
}

func (a Stub) Engine() engine.Engine {
	return nil
}

func (a Stub) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return nil
}

func (a Stub) SubscribeTraceEvent(ch chan<- TraceEvent) event.Subscription {
	return nil
}

func (a Stub) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return nil
}

func (a Stub) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return nil
}

func (a Stub) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return nil
}

func (a Stub) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return nil
}

func (a Stub) ReadShardState(epoch *big.Int) (*shard.State, error) {
	return nil, errors.Errorf("method ReadShardState not implemented for %s", a.Name)
}

func (a Stub) WriteShardStateBytes(db rawdb.DatabaseWriter, epoch *big.Int, shardState []byte) (*shard.State, error) {
	return nil, errors.Errorf("method WriteShardStateBytes not implemented for %s", a.Name)
}

func (a Stub) WriteHeadBlock(block *types.Block) error {
	return errors.Errorf("method WriteHeadBlock not implemented for %s", a.Name)
}

func (a Stub) ReadCommitSig(blockNum uint64) ([]byte, error) {
	return nil, errors.Errorf("method ReadCommitSig not implemented for %s", a.Name)
}

func (a Stub) WriteCommitSig(blockNum uint64, lastCommits []byte) error {
	return errors.Errorf("method WriteCommitSig not implemented for %s", a.Name)
}

func (a Stub) GetVdfByNumber(number uint64) []byte {
	return nil
}

func (a Stub) GetVrfByNumber(number uint64) []byte {
	return nil
}

func (a Stub) ChainDb() ethdb.Database {
	return nil
}

func (a Stub) GetEpochBlockNumber(epoch *big.Int) (*big.Int, error) {
	return nil, errors.Errorf("method GetEpochBlockNumber not implemented for %s", a.Name)
}

func (a Stub) StoreEpochBlockNumber(epoch *big.Int, blockNum *big.Int) error {
	return errors.Errorf("method StoreEpochBlockNumber not implemented for %s", a.Name)
}

func (a Stub) ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error) {
	return nil, errors.Errorf("method ReadEpochVrfBlockNums not implemented for %s", a.Name)
}

func (a Stub) WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error {
	return errors.Errorf("method WriteEpochVrfBlockNums not implemented for %s", a.Name)
}

func (a Stub) ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error) {
	return nil, errors.Errorf("method ReadEpochVdfBlockNum not implemented for %s", a.Name)
}

func (a Stub) WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error {
	return errors.Errorf("method WriteEpochVdfBlockNum not implemented for %s", a.Name)
}

func (a Stub) WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error {
	return errors.Errorf("method WriteCrossLinks not implemented for %s", a.Name)
}

func (a Stub) DeleteCrossLinks(cls []types.CrossLink) error {
	return errors.Errorf("method DeleteCrossLinks not implemented for %s", a.Name)
}

func (a Stub) ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	return nil, errors.Errorf("method ReadCrossLink not implemented for %s", a.Name)
}

func (a Stub) LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error {
	return errors.Errorf("method LastContinuousCrossLink not implemented for %s", a.Name)
}

func (a Stub) ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error) {
	return nil, errors.Errorf("method ReadShardLastCrossLink not implemented for %s", a.Name)
}

func (a Stub) DeleteFromPendingSlashingCandidates(processed slash.Records) error {
	return errors.Errorf("method DeleteFromPendingSlashingCandidates not implemented for %s", a.Name)
}

func (a Stub) ReadPendingSlashingCandidates() slash.Records {
	return nil
}

func (a Stub) ReadPendingCrossLinks() ([]types.CrossLink, error) {
	return nil, errors.Errorf("method ReadPendingCrossLinks not implemented for %s", a.Name)
}

func (a Stub) CachePendingCrossLinks(crossLinks []types.CrossLink) error {
	return errors.Errorf("method CachePendingCrossLinks not implemented for %s", a.Name)
}

func (a Stub) SavePendingCrossLinks() error {
	return errors.Errorf("method SavePendingCrossLinks not implemented for %s", a.Name)
}

func (a Stub) AddPendingSlashingCandidates(candidates slash.Records) error {
	return errors.Errorf("method AddPendingSlashingCandidates not implemented for %s", a.Name)
}

func (a Stub) AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error) {
	return 0, errors.Errorf("method AddPendingCrossLinks not implemented for %s", a.Name)
}

func (a Stub) DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error) {
	return 0, errors.Errorf("method DeleteFromPendingCrossLinks not implemented for %s", a.Name)
}

func (a Stub) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	return false
}

func (a Stub) GetVMConfig() *vm.Config {
	return nil
}

func (a Stub) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error) {
	return nil, errors.Errorf("method ReadCXReceipts not implemented for %s", a.Name)
}

func (a Stub) CXMerkleProof(toShardID uint32, block *block.Header) (*types.CXMerkleProof, error) {
	return nil, errors.Errorf("method CXMerkleProof not implemented for %s", a.Name)
}

func (a Stub) WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error {
	return errors.Errorf("method WriteCXReceiptsProofSpent not implemented for %s", a.Name)
}

func (a Stub) IsSpent(cxp *types.CXReceiptsProof) bool {
	return false
}

func (a Stub) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	return common.Hash{}, 0, 0
}

func (a Stub) ReadValidatorInformationAtRoot(addr common.Address, root common.Hash) (*staking.ValidatorWrapper, error) {
	return nil, errors.Errorf("method ReadValidatorInformationAtRoot not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	return nil, errors.Errorf("method ReadValidatorInformationAtState not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	return nil, errors.Errorf("method ReadValidatorInformation not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, errors.Errorf("method ReadValidatorSnapshotAtEpoch not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, errors.Errorf("method ReadValidatorSnapshot not implemented for %s", a.Name)
}

func (a Stub) WriteValidatorSnapshot(batch rawdb.DatabaseWriter, snapshot *staking.ValidatorSnapshot) error {
	return errors.Errorf("method WriteValidatorSnapshot not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return nil, errors.Errorf("method ReadValidatorStats not implemented for %s", a.Name)
}

func (a Stub) UpdateValidatorVotingPower(batch rawdb.DatabaseWriter, block *types.Block, newEpochSuperCommittee, currentEpochSuperCommittee *shard.State, state *state.DB) (map[common.Address]*staking.ValidatorStats, error) {
	return nil, errors.Errorf("method UpdateValidatorVotingPower not implemented for %s", a.Name)
}

func (a Stub) ComputeAndUpdateAPR(block *types.Block, now *big.Int, wrapper *staking.ValidatorWrapper, stats *staking.ValidatorStats) error {
	return errors.Errorf("method ComputeAndUpdateAPR not implemented for %s", a.Name)
}

func (a Stub) UpdateValidatorSnapshots(batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address) error {
	return errors.Errorf("method UpdateValidatorSnapshots not implemented for %s", a.Name)
}

func (a Stub) ReadValidatorList() ([]common.Address, error) {
	return nil, errors.Errorf("method ReadValidatorList not implemented for %s", a.Name)
}

func (a Stub) WriteValidatorList(db rawdb.DatabaseWriter, addrs []common.Address) error {
	return errors.Errorf("method WriteValidatorList not implemented for %s", a.Name)
}

func (a Stub) ReadDelegationsByDelegator(delegator common.Address) (m staking.DelegationIndexes, err error) {
	return nil, errors.Errorf("method ReadDelegationsByDelegator not implemented for %s", a.Name)
}

func (a Stub) ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (m staking.DelegationIndexes, err error) {
	return nil, errors.Errorf("method ReadDelegationsByDelegatorAt not implemented for %s", a.Name)
}

func (a Stub) UpdateStakingMetaData(batch rawdb.DatabaseWriter, block *types.Block, stakeMsgs []staking.StakeMsg, state *state.DB, epoch, newEpoch *big.Int) (newValidators []common.Address, err error) {
	return nil, errors.Errorf("method UpdateStakingMetaData not implemented for %s", a.Name)
}

func (a Stub) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	return nil, errors.Errorf("method ReadBlockRewardAccumulator not implemented for %s", a.Name)
}

func (a Stub) WriteBlockRewardAccumulator(batch rawdb.DatabaseWriter, reward *big.Int, number uint64) error {
	return errors.Errorf("method WriteBlockRewardAccumulator not implemented for %s", a.Name)
}

func (a Stub) UpdateBlockRewardAccumulator(batch rawdb.DatabaseWriter, diff *big.Int, number uint64) error {
	return errors.Errorf("method UpdateBlockRewardAccumulator not implemented for %s", a.Name)
}

func (a Stub) ValidatorCandidates() []common.Address {
	return nil
}

func (a Stub) DelegatorsInformation(addr common.Address) []*staking.Delegation {
	return nil
}

func (a Stub) GetECDSAFromCoinbase(header *block.Header) (common.Address, error) {
	return common.Address{}, errors.Errorf("method GetECDSAFromCoinbase not implemented for %s", a.Name)
}

func (a Stub) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	return nil, errors.Errorf("method SuperCommitteeForNextEpoch not implemented for %s", a.Name)
}

func (a Stub) EnablePruneBeaconChainFeature() {
}

func (a Stub) IsEnablePruneBeaconChainFeature() bool {
	return false
}

func (a Stub) CommitOffChainData(batch rawdb.DatabaseWriter, block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []staking.StakeMsg, payout reward.Reader, state *state.DB) (status WriteStatus, err error) {
	return 0, errors.Errorf("method CommitOffChainData not implemented for %s", a.Name)
}

func (a Stub) GetLeaderPubKeyFromCoinbase(h *block.Header) (*bls.PublicKeyWrapper, error) {
	return nil, errors.Errorf("method GetLeaderPubKeyFromCoinbase not implemented for %s", a.Name)
}

func (a Stub) IsTikvWriterMaster() bool {
	return false
}

func (a Stub) RedisPreempt() *redis_helper.RedisPreempt {
	return nil
}

func (a Stub) SyncFromTiKVWriter(newBlkNum uint64, logs []*types.Log) error {
	return errors.Errorf("method SyncFromTiKVWriter not implemented for %s", a.Name)
}

func (a Stub) InitTiKV(conf *harmonyconfig.TiKVConfig) {
	return
}

func (a Stub) LeaderRotationMeta() LeaderRotationMeta {
	return LeaderRotationMeta{}
}

func (a Stub) CommitPreimages() error {
	return errors.Errorf("method CommitPreimages not implemented for %s", a.Name)
}

func (a Stub) GetStateCache() state.Database {
	return nil
}

func (a Stub) GetSnapshotTrie() *snapshot.Tree {
	return nil
}
