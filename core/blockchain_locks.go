package core

import (
	"io"
	"math/big"
	"time"

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
	"github.com/harmony-one/harmony/libs/locker"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	types2 "github.com/harmony-one/harmony/staking/types"
)

type BlockChainWithLocks struct {
	bc   *BlockChainWithoutLocks
	lock locker.RWLocker
}

func newBlockchainWithLocks(bc *BlockChainWithoutLocks) *BlockChainWithLocks {
	return &BlockChainWithLocks{
		bc:   bc,
		lock: locker.NewRwLocker(2 * time.Second),
	}
}

func (b *BlockChainWithLocks) CommitOffChainData(batch rawdb.DatabaseWriter, block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []types2.StakeMsg, payout reward.Reader, state *state.DB) (status WriteStatus, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.CommitOffChainData(batch, block, receipts, cxReceipts, stakeMsgs, payout, state)
}

func (b *BlockChainWithLocks) ValidateNewBlock(block *types.Block) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.ValidateNewBlock(block)
}

func (b *BlockChainWithLocks) SetHead(head uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.SetHead(head)
}

func (b *BlockChainWithLocks) ShardID() uint32 {
	return b.bc.ShardID()
}

func (b *BlockChainWithLocks) GasLimit() uint64 {
	return b.bc.GasLimit()
}

func (b *BlockChainWithLocks) CurrentBlock() *types.Block {
	return b.bc.CurrentBlock()
}

func (b *BlockChainWithLocks) CurrentFastBlock() *types.Block {
	return b.bc.CurrentFastBlock()
}

func (b *BlockChainWithLocks) SetProcessor(processor Processor) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.bc.SetProcessor(processor)
}

func (b *BlockChainWithLocks) SetValidator(validator Validator) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.bc.SetValidator(validator)
}

func (b *BlockChainWithLocks) Validator() Validator {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.Validator()
}

func (b *BlockChainWithLocks) Processor() Processor {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.Processor()
}

func (b *BlockChainWithLocks) State() (*state.DB, error) {
	return b.bc.State()
}

func (b *BlockChainWithLocks) StateAt(root common.Hash) (*state.DB, error) {
	return b.bc.StateAt(root)
}

func (b *BlockChainWithLocks) Reset() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.Reset()
}

func (b *BlockChainWithLocks) ResetWithGenesisBlock(genesis *types.Block) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.ResetWithGenesisBlock(genesis)
}

func (b *BlockChainWithLocks) Export(w io.Writer) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.Export(w)
}

func (b *BlockChainWithLocks) ExportN(w io.Writer, first uint64, last uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.ExportN(w, first, last)
}

func (b *BlockChainWithLocks) Genesis() *types.Block {
	return b.bc.Genesis()
}

func (b *BlockChainWithLocks) GetBody(hash common.Hash) *types.Body {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBody(hash)
}

func (b *BlockChainWithLocks) GetBodyRLP(hash common.Hash) rlp.RawValue {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBodyRLP(hash)
}

func (b *BlockChainWithLocks) HasBlock(hash common.Hash, number uint64) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.HasBlock(hash, number)
}

func (b *BlockChainWithLocks) HasState(hash common.Hash) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.HasState(hash)
}

func (b *BlockChainWithLocks) HasBlockAndState(hash common.Hash, number uint64) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.HasBlockAndState(hash, number)
}

func (b *BlockChainWithLocks) GetBlock(hash common.Hash, number uint64) *types.Block {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBlock(hash, number)
}

func (b *BlockChainWithLocks) GetBlockByHash(hash common.Hash) *types.Block {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBlockByHash(hash)
}

func (b *BlockChainWithLocks) GetBlockByNumber(number uint64) *types.Block {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBlockByNumber(number)
}

func (b *BlockChainWithLocks) GetReceiptsByHash(hash common.Hash) types.Receipts {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetReceiptsByHash(hash)
}

func (b *BlockChainWithLocks) GetBlocksFromHash(hash common.Hash, n int) (blocks []*types.Block) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBlocksFromHash(hash, n)
}

func (b *BlockChainWithLocks) GetUnclesInChain(block *types.Block, length int) []*block.Header {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetUnclesInChain(block, length)
}

func (b *BlockChainWithLocks) TrieNode(hash common.Hash) ([]byte, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.TrieNode(hash)
}

func (b *BlockChainWithLocks) Stop() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.bc.Stop()
}

func (b *BlockChainWithLocks) Rollback(chain []common.Hash) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.Rollback(chain)
}

func (b *BlockChainWithLocks) InsertReceiptChain(blockChain types.Blocks, receiptChain []types.Receipts) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.InsertReceiptChain(blockChain, receiptChain)
}

func (b *BlockChainWithLocks) WriteBlockWithoutState(block *types.Block, td *big.Int) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteBlockWithoutState(block, td)
}

func (b *BlockChainWithLocks) WriteBlockWithState(block *types.Block, receipts []*types.Receipt, cxReceipts []*types.CXReceipt, stakeMsgs []types2.StakeMsg, paid reward.Reader, state *state.DB) (status WriteStatus, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteBlockWithState(block, receipts, cxReceipts, stakeMsgs, paid, state)
}

func (b *BlockChainWithLocks) GetMaxGarbageCollectedBlockNumber() int64 {
	return b.bc.GetMaxGarbageCollectedBlockNumber()
}

func (b *BlockChainWithLocks) InsertChain(chain types.Blocks, verifyHeaders bool) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.InsertChain(chain, verifyHeaders)
}

func (b *BlockChainWithLocks) PostChainEvents(events []interface{}, logs []*types.Log) {
	b.bc.PostChainEvents(events, logs)
}

func (b *BlockChainWithLocks) BadBlocks() []BadBlock {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.BadBlocks()
}

func (b *BlockChainWithLocks) InsertHeaderChain(chain []*block.Header, checkFreq int) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.InsertHeaderChain(chain, checkFreq)
}

func (b *BlockChainWithLocks) CurrentHeader() *block.Header {
	return b.bc.CurrentHeader()
}

func (b *BlockChainWithLocks) GetTd(hash common.Hash, number uint64) *big.Int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetTd(hash, number)
}

func (b *BlockChainWithLocks) GetTdByHash(hash common.Hash) *big.Int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetTdByHash(hash)
}

func (b *BlockChainWithLocks) GetHeader(hash common.Hash, number uint64) *block.Header {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetHeader(hash, number)
}

func (b *BlockChainWithLocks) GetHeaderByHash(hash common.Hash) *block.Header {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetHeaderByHash(hash)
}

func (b *BlockChainWithLocks) HasHeader(hash common.Hash, number uint64) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.HasHeader(hash, number)
}

func (b *BlockChainWithLocks) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetBlockHashesFromHash(hash, max)
}

func (b *BlockChainWithLocks) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

func (b *BlockChainWithLocks) GetHeaderByNumber(number uint64) *block.Header {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetHeaderByNumber(number)
}

func (b *BlockChainWithLocks) Config() *params.ChainConfig {
	return b.bc.Config()
}

func (b *BlockChainWithLocks) Engine() engine.Engine {
	return b.bc.Engine()
}

func (b *BlockChainWithLocks) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return b.bc.SubscribeRemovedLogsEvent(ch)
}

func (b *BlockChainWithLocks) SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription {
	return b.bc.SubscribeChainEvent(ch)
}

func (b *BlockChainWithLocks) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return b.bc.SubscribeChainHeadEvent(ch)
}

func (b *BlockChainWithLocks) SubscribeChainSideEvent(ch chan<- ChainSideEvent) event.Subscription {
	return b.bc.SubscribeChainSideEvent(ch)
}

func (b *BlockChainWithLocks) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.bc.SubscribeLogsEvent(ch)
}

func (b *BlockChainWithLocks) ReadShardState(epoch *big.Int) (*shard.State, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadShardState(epoch)
}

func (b *BlockChainWithLocks) WriteShardStateBytes(db rawdb.DatabaseWriter, epoch *big.Int, shardState []byte) (*shard.State, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteShardStateBytes(db, epoch, shardState)
}

func (b *BlockChainWithLocks) ReadCommitSig(blockNum uint64) ([]byte, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadCommitSig(blockNum)
}

func (b *BlockChainWithLocks) WriteCommitSig(blockNum uint64, lastCommits []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteCommitSig(blockNum, lastCommits)
}

func (b *BlockChainWithLocks) GetVdfByNumber(number uint64) []byte {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetVdfByNumber(number)
}

func (b *BlockChainWithLocks) GetVrfByNumber(number uint64) []byte {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetVrfByNumber(number)
}

func (b *BlockChainWithLocks) ChainDb() ethdb.Database {
	return b.bc.ChainDb()
}

func (b *BlockChainWithLocks) GetEpochBlockNumber(epoch *big.Int) (*big.Int, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetEpochBlockNumber(epoch)
}

func (b *BlockChainWithLocks) StoreEpochBlockNumber(epoch *big.Int, blockNum *big.Int) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.StoreEpochBlockNumber(epoch, blockNum)
}

func (b *BlockChainWithLocks) ReadEpochVrfBlockNums(epoch *big.Int) ([]uint64, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadEpochVrfBlockNums(epoch)
}

func (b *BlockChainWithLocks) WriteEpochVrfBlockNums(epoch *big.Int, vrfNumbers []uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteEpochVrfBlockNums(epoch, vrfNumbers)
}

func (b *BlockChainWithLocks) ReadEpochVdfBlockNum(epoch *big.Int) (*big.Int, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadEpochVdfBlockNum(epoch)
}

func (b *BlockChainWithLocks) WriteEpochVdfBlockNum(epoch *big.Int, blockNum *big.Int) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteEpochVdfBlockNum(epoch, blockNum)
}

func (b *BlockChainWithLocks) WriteCrossLinks(batch rawdb.DatabaseWriter, cls []types.CrossLink) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteCrossLinks(batch, cls)
}

func (b *BlockChainWithLocks) DeleteCrossLinks(cls []types.CrossLink) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.DeleteCrossLinks(cls)
}

func (b *BlockChainWithLocks) ReadCrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadCrossLink(shardID, blockNum)
}

func (b *BlockChainWithLocks) LastContinuousCrossLink(batch rawdb.DatabaseWriter, shardID uint32) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.LastContinuousCrossLink(batch, shardID)
}

func (b *BlockChainWithLocks) ReadShardLastCrossLink(shardID uint32) (*types.CrossLink, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadShardLastCrossLink(shardID)
}

func (b *BlockChainWithLocks) DeleteFromPendingSlashingCandidates(processed slash.Records) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.DeleteFromPendingSlashingCandidates(processed)
}

func (b *BlockChainWithLocks) ReadPendingSlashingCandidates() slash.Records {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadPendingSlashingCandidates()
}

func (b *BlockChainWithLocks) ReadPendingCrossLinks() ([]types.CrossLink, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadPendingCrossLinks()
}

func (b *BlockChainWithLocks) CachePendingCrossLinks(crossLinks []types.CrossLink) error {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.CachePendingCrossLinks(crossLinks)
}

func (b *BlockChainWithLocks) SavePendingCrossLinks() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.SavePendingCrossLinks()
}

func (b *BlockChainWithLocks) AddPendingSlashingCandidates(candidates slash.Records) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.AddPendingSlashingCandidates(candidates)
}

func (b *BlockChainWithLocks) AddPendingCrossLinks(pendingCLs []types.CrossLink) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.AddPendingCrossLinks(pendingCLs)
}

func (b *BlockChainWithLocks) DeleteFromPendingCrossLinks(crossLinks []types.CrossLink) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.DeleteFromPendingCrossLinks(crossLinks)
}

func (b *BlockChainWithLocks) IsSameLeaderAsPreviousBlock(block *types.Block) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.IsSameLeaderAsPreviousBlock(block)
}

func (b *BlockChainWithLocks) GetVMConfig() *vm.Config {
	return b.bc.GetVMConfig()
}

func (b *BlockChainWithLocks) ReadCXReceipts(shardID uint32, blockNum uint64, blockHash common.Hash) (types.CXReceipts, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadCXReceipts(shardID, blockNum, blockHash)
}

func (b *BlockChainWithLocks) CXMerkleProof(toShardID uint32, block *types.Block) (*types.CXMerkleProof, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.CXMerkleProof(toShardID, block)
}

func (b *BlockChainWithLocks) WriteCXReceiptsProofSpent(db rawdb.DatabaseWriter, cxps []*types.CXReceiptsProof) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteCXReceiptsProofSpent(db, cxps)
}

func (b *BlockChainWithLocks) IsSpent(cxp *types.CXReceiptsProof) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.IsSpent(cxp)
}

func (b *BlockChainWithLocks) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadTxLookupEntry(txID)
}

func (b *BlockChainWithLocks) ReadValidatorInformationAtRoot(addr common.Address, root common.Hash) (*types2.ValidatorWrapper, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorInformationAtRoot(addr, root)
}

func (b *BlockChainWithLocks) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*types2.ValidatorWrapper, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorInformationAtState(addr, state)
}

func (b *BlockChainWithLocks) ReadValidatorInformation(addr common.Address) (*types2.ValidatorWrapper, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorInformation(addr)
}

func (b *BlockChainWithLocks) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*types2.ValidatorSnapshot, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorSnapshotAtEpoch(epoch, addr)
}

func (b *BlockChainWithLocks) ReadValidatorSnapshot(addr common.Address) (*types2.ValidatorSnapshot, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorSnapshot(addr)
}

func (b *BlockChainWithLocks) WriteValidatorSnapshot(batch rawdb.DatabaseWriter, snapshot *types2.ValidatorSnapshot) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteValidatorSnapshot(batch, snapshot)
}

func (b *BlockChainWithLocks) ReadValidatorStats(addr common.Address) (*types2.ValidatorStats, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorStats(addr)
}

func (b *BlockChainWithLocks) UpdateValidatorVotingPower(batch rawdb.DatabaseWriter, block *types.Block, newEpochSuperCommittee, currentEpochSuperCommittee *shard.State, state *state.DB) (map[common.Address]*types2.ValidatorStats, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.UpdateValidatorVotingPower(batch, block, newEpochSuperCommittee, currentEpochSuperCommittee, state)
}

func (b *BlockChainWithLocks) ComputeAndUpdateAPR(block *types.Block, now *big.Int, wrapper *types2.ValidatorWrapper, stats *types2.ValidatorStats) error {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ComputeAndUpdateAPR(block, now, wrapper, stats)
}

func (b *BlockChainWithLocks) UpdateValidatorSnapshots(batch rawdb.DatabaseWriter, epoch *big.Int, state *state.DB, newValidators []common.Address) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.UpdateValidatorSnapshots(batch, epoch, state, newValidators)
}

func (b *BlockChainWithLocks) ReadValidatorList() ([]common.Address, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadValidatorList()
}

func (b *BlockChainWithLocks) WriteValidatorList(db rawdb.DatabaseWriter, addrs []common.Address) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteValidatorList(db, addrs)
}

func (b *BlockChainWithLocks) ReadDelegationsByDelegator(delegator common.Address) (m types2.DelegationIndexes, err error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadDelegationsByDelegator(delegator)
}

func (b *BlockChainWithLocks) ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (m types2.DelegationIndexes, err error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadDelegationsByDelegatorAt(delegator, blockNum)
}

func (b *BlockChainWithLocks) UpdateStakingMetaData(batch rawdb.DatabaseWriter, block *types.Block, stakeMsgs []types2.StakeMsg, state *state.DB, epoch, newEpoch *big.Int) (newValidators []common.Address, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.UpdateStakingMetaData(batch, block, stakeMsgs, state, epoch, newEpoch)
}

func (b *BlockChainWithLocks) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ReadBlockRewardAccumulator(number)
}

func (b *BlockChainWithLocks) WriteBlockRewardAccumulator(batch rawdb.DatabaseWriter, reward *big.Int, number uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.WriteBlockRewardAccumulator(batch, reward, number)
}

func (b *BlockChainWithLocks) UpdateBlockRewardAccumulator(batch rawdb.DatabaseWriter, diff *big.Int, number uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.bc.UpdateBlockRewardAccumulator(batch, diff, number)
}

func (b *BlockChainWithLocks) ValidatorCandidates() []common.Address {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.ValidatorCandidates()
}

func (b *BlockChainWithLocks) DelegatorsInformation(addr common.Address) []*types2.Delegation {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.DelegatorsInformation(addr)
}

func (b *BlockChainWithLocks) GetECDSAFromCoinbase(header *block.Header) (common.Address, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.GetECDSAFromCoinbase(header)
}

func (b *BlockChainWithLocks) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.bc.SuperCommitteeForNextEpoch(beacon, header, isVerify)
}

func (b *BlockChainWithLocks) EnablePruneBeaconChainFeature() {
	b.bc.EnablePruneBeaconChainFeature()
}

func (b *BlockChainWithLocks) IsEnablePruneBeaconChainFeature() bool {
	return b.bc.IsEnablePruneBeaconChainFeature()
}

func (b *BlockChainWithLocks) writeDelegationsByDelegator(
	batch rawdb.DatabaseWriter,
	delegator common.Address,
	indices []staking.DelegationIndex,
) error {
	return b.bc.writeDelegationsByDelegator(batch, delegator, indices)
}

func (b *BlockChainWithLocks) prepareStakingMetaData(
	block *types.Block, stakeMsgs []staking.StakeMsg, state *state.DB,
) ([]common.Address,
	map[common.Address]staking.DelegationIndexes,
	error,
) {
	return b.bc.prepareStakingMetaData(block, stakeMsgs, state)
}
