package hmy

import (
	"context"
	"fmt"
	"math/big"

	v3 "github.com/harmony-one/harmony/block/v3"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internal_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/eth/rpc"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
	"github.com/pkg/errors"
)

// ChainConfig ...
func (hmy *Harmony) ChainConfig() *params.ChainConfig {
	return hmy.BlockChain.Config()
}

// GetShardState ...
func (hmy *Harmony) GetShardState() (*shard.State, error) {
	return hmy.BlockChain.ReadShardState(hmy.BlockChain.CurrentHeader().Epoch())
}

// GetBlockSigners ..
func (hmy *Harmony) GetBlockSigners(
	ctx context.Context, blockNum rpc.BlockNumber,
) (shard.SlotList, *internal_bls.Mask, error) {
	blk, err := hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, nil, err
	}
	blockWithSigners, err := hmy.BlockByNumber(ctx, blockNum+1)
	if err != nil {
		return nil, nil, err
	}
	if blockWithSigners == nil {
		return nil, nil, fmt.Errorf("block number %v not found", blockNum+1)
	}
	committee, err := hmy.GetValidators(blk.Epoch())
	if err != nil {
		return nil, nil, err
	}
	pubKeys := make([]internal_bls.PublicKeyWrapper, len(committee.Slots))
	for i, validator := range committee.Slots {
		key, err := bls.BytesToBLSPublicKey(validator.BLSPublicKey[:])
		if err != nil {
			return nil, nil, err
		}
		pubKeys[i] = internal_bls.PublicKeyWrapper{
			Bytes:  validator.BLSPublicKey,
			Object: key,
		}
	}
	mask, err := internal_bls.NewMask(pubKeys, nil)
	if err != nil {
		return nil, nil, err
	}
	err = mask.SetMask(blockWithSigners.Header().LastCommitBitmap())
	if err != nil {
		return nil, nil, err
	}
	return committee.Slots, mask, nil
}

// DetailedBlockSignerInfo contains all of the block singing information
type DetailedBlockSignerInfo struct {
	// Signers are all the signers for the block
	Signers shard.SlotList
	// Committee when the block was signed.
	Committee shard.SlotList
	BlockHash common.Hash
}

// GetDetailedBlockSignerInfo fetches the block signer information for any non-genesis block
func (hmy *Harmony) GetDetailedBlockSignerInfo(
	ctx context.Context, blk *types.Block,
) (*DetailedBlockSignerInfo, error) {
	parentBlk, err := hmy.BlockByNumber(ctx, rpc.BlockNumber(blk.NumberU64()-1))
	if err != nil {
		return nil, err
	}
	parentShardState, err := hmy.BlockChain.ReadShardState(parentBlk.Epoch())
	if err != nil {
		return nil, err
	}
	committee, signers, _, err := availability.BallotResult(
		parentBlk.Header(), blk.Header(), parentShardState, blk.ShardID(),
	)
	return &DetailedBlockSignerInfo{
		Signers:   signers,
		Committee: committee,
		BlockHash: blk.Hash(),
	}, nil
}

// PreStakingBlockRewards are the rewards for a block in the pre-staking era (epoch < staking epoch).
type PreStakingBlockRewards map[common.Address]*big.Int

// GetPreStakingBlockRewards for the given block number.
// Calculated rewards are done exactly like chain.AccumulateRewardsAndCountSigs.
func (hmy *Harmony) GetPreStakingBlockRewards(
	ctx context.Context, blk *types.Block,
) (PreStakingBlockRewards, error) {
	if hmy.IsStakingEpoch(blk.Epoch()) {
		return nil, fmt.Errorf("block %v is in staking era", blk.Number())
	}

	if cachedReward, ok := hmy.preStakingBlockRewardsCache.Get(blk.Hash()); ok {
		return cachedReward.(PreStakingBlockRewards), nil
	}
	rewards := PreStakingBlockRewards{}

	sigInfo, err := hmy.GetDetailedBlockSignerInfo(ctx, blk)
	if err != nil {
		return nil, err
	}
	last := big.NewInt(0)
	count := big.NewInt(int64(len(sigInfo.Signers)))
	for i, slot := range sigInfo.Signers {
		rewardsForThisAddr, ok := rewards[slot.EcdsaAddress]
		if !ok {
			rewardsForThisAddr = big.NewInt(0)
		}
		cur := big.NewInt(0)
		cur.Mul(stakingReward.PreStakedBlocks, big.NewInt(int64(i+1))).Div(cur, count)
		reward := big.NewInt(0).Sub(cur, last)
		rewards[slot.EcdsaAddress] = new(big.Int).Add(reward, rewardsForThisAddr)
		last = cur
	}

	// Report tx fees of the coinbase (== leader)
	receipts, err := hmy.GetReceipts(ctx, blk.Hash())
	if err != nil {
		return nil, err
	}
	txFees := big.NewInt(0)
	for _, tx := range blk.Transactions() {
		txnHash := tx.HashByType()
		dbTx, _, _, receiptIndex := rawdb.ReadTransaction(hmy.ChainDb(), txnHash)
		if dbTx == nil {
			return nil, fmt.Errorf("could not find receipt for tx: %v", txnHash.String())
		}
		if len(receipts) <= int(receiptIndex) {
			return nil, fmt.Errorf("invalid receipt indext %v (>= num receipts: %v) for tx: %v",
				receiptIndex, len(receipts), txnHash.String())
		}
		txFee := new(big.Int).Mul(tx.GasPrice(), big.NewInt(int64(receipts[receiptIndex].GasUsed)))
		txFees = new(big.Int).Add(txFee, txFees)
	}

	if amt, ok := rewards[blk.Header().Coinbase()]; ok {
		rewards[blk.Header().Coinbase()] = new(big.Int).Add(amt, txFees)
	} else {
		rewards[blk.Header().Coinbase()] = txFees
	}

	hmy.preStakingBlockRewardsCache.Add(blk.Hash(), rewards)
	return rewards, nil
}

// GetLatestChainHeaders ..
func (hmy *Harmony) GetLatestChainHeaders() *block.HeaderPair {
	pair := &block.HeaderPair{
		BeaconHeader: &block.Header{Header: v3.NewHeader()},
		ShardHeader:  &block.Header{Header: v3.NewHeader()},
	}

	if hmy.BeaconChain != nil {
		pair.BeaconHeader = hmy.BeaconChain.CurrentHeader()
	}

	if hmy.BlockChain != nil {
		pair.ShardHeader = hmy.BlockChain.CurrentHeader()
	}

	return pair
}

// GetLastCrossLinks ..
func (hmy *Harmony) GetLastCrossLinks() ([]*types.CrossLink, error) {
	crossLinks := []*types.CrossLink{}
	for i := uint32(1); i < shard.Schedule.InstanceForEpoch(hmy.CurrentBlock().Epoch()).NumShards(); i++ {
		link, err := hmy.BlockChain.ReadShardLastCrossLink(i)
		if err != nil {
			return nil, err
		}
		crossLinks = append(crossLinks, link)
	}

	return crossLinks, nil
}

// CurrentBlock ...
func (hmy *Harmony) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(hmy.BlockChain.CurrentHeader())
}

// CurrentHeader returns the current header from the local chain.
func (hmy *Harmony) CurrentHeader() *block.Header {
	return hmy.BlockChain.CurrentHeader()
}

// GetBlock returns block by hash.
func (hmy *Harmony) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return hmy.BlockChain.GetBlockByHash(hash), nil
}

// GetHeader returns header by hash.
func (hmy *Harmony) GetHeader(ctx context.Context, hash common.Hash) (*block.Header, error) {
	return hmy.BlockChain.GetHeaderByHash(hash), nil
}

// GetCurrentBadBlocks ..
func (hmy *Harmony) GetCurrentBadBlocks() []core.BadBlock {
	return hmy.BlockChain.BadBlocks()
}

func (hmy *Harmony) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return hmy.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := hmy.BlockChain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && hmy.BlockChain.GetCanonicalHash(header.Number().Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := hmy.BlockChain.GetBlock(hash, header.Number().Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

// GetBalance returns balance of an given address.
func (hmy *Harmony) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*big.Int, error) {
	s, _, err := hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if s == nil || err != nil {
		return nil, err
	}
	return s.GetBalance(address), s.Error()
}

// BlockByNumber ...
func (hmy *Harmony) BlockByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNum == rpc.LatestBlockNumber {
		return hmy.BlockChain.CurrentBlock(), nil
	}
	return hmy.BlockChain.GetBlockByNumber(uint64(blockNum)), nil
}

// HeaderByNumber ...
func (hmy *Harmony) HeaderByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*block.Header, error) {
	// Pending block is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNum == rpc.LatestBlockNumber {
		return hmy.BlockChain.CurrentBlock().Header(), nil
	}
	return hmy.BlockChain.GetHeaderByNumber(uint64(blockNum)), nil
}

// HeaderByHash ...
func (hmy *Harmony) HeaderByHash(ctx context.Context, blockHash common.Hash) (*block.Header, error) {
	header := hmy.BlockChain.GetHeaderByHash(blockHash)
	if header == nil {
		return nil, errors.New("Header is not found")
	}
	return header, nil
}

// StateAndHeaderByNumber ...
func (hmy *Harmony) StateAndHeaderByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*state.DB, *block.Header, error) {
	// Pending state is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, nil, errors.New("not implemented")
	}
	// Otherwise resolve the block number and return its state
	header, err := hmy.HeaderByNumber(ctx, blockNum)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := hmy.BlockChain.StateAt(header.Root())
	return stateDb, header, err
}

func (hmy *Harmony) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.DB, *block.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return hmy.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := hmy.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && hmy.BlockChain.GetCanonicalHash(header.Number().Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := hmy.BlockChain.StateAt(header.Root())
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

// GetLeaderAddress returns the one address of the leader, given the coinbaseAddr.
// Note that the coinbaseAddr is overloaded with the BLS pub key hash in staking era.
func (hmy *Harmony) GetLeaderAddress(coinbaseAddr common.Address, epoch *big.Int) string {
	if hmy.IsStakingEpoch(epoch) {
		if leader, exists := hmy.leaderCache.Get(coinbaseAddr); exists {
			bech32, _ := internal_common.AddressToBech32(leader.(common.Address))
			return bech32
		}
		committee, err := hmy.GetValidators(epoch)
		if err != nil {
			return ""
		}
		for _, val := range committee.Slots {
			addr := utils.GetAddressFromBLSPubKeyBytes(val.BLSPublicKey[:])
			hmy.leaderCache.Add(addr, val.EcdsaAddress)
			if addr == coinbaseAddr {
				bech32, _ := internal_common.AddressToBech32(val.EcdsaAddress)
				return bech32
			}
		}
		return "" // Did not find matching address
	}
	bech32, _ := internal_common.AddressToBech32(coinbaseAddr)
	return bech32
}

// Filter related APIs

// GetLogs ...
func (hmy *Harmony) GetLogs(ctx context.Context, blockHash common.Hash, isEth bool) ([][]*types.Log, error) {
	receipts := hmy.BlockChain.GetReceiptsByHash(blockHash)
	if receipts == nil {
		return nil, errors.New("Missing receipts")
	}
	if isEth {
		block := hmy.BlockChain.GetBlockByHash(blockHash)
		if block == nil {
			return nil, errors.New("Missing block data")
		}
		txns := block.Transactions()
		for i := range receipts {
			if i < len(txns) {
				ethHash := txns[i].ConvertToEth().Hash()
				receipts[i].TxHash = ethHash
				for j := range receipts[i].Logs {
					// Override log txHash with receipt's
					receipts[i].Logs[j].TxHash = ethHash
				}
			}
		}
	}

	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

// ServiceFilter ...
func (hmy *Harmony) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	// TODO(dm): implement
}

// SubscribeNewTxsEvent subscribes new tx event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return hmy.TxPool.SubscribeNewTxsEvent(ch)
}

// SubscribeChainEvent subscribes chain event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return hmy.BlockChain.SubscribeChainEvent(ch)
}

// SubscribeChainHeadEvent subcribes chain head event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return hmy.BlockChain.SubscribeChainHeadEvent(ch)
}

// SubscribeChainSideEvent subcribes chain side event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return hmy.BlockChain.SubscribeChainSideEvent(ch)
}

// SubscribeRemovedLogsEvent subcribes removed logs event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return hmy.BlockChain.SubscribeRemovedLogsEvent(ch)
}

// SubscribeLogsEvent subcribes log event.
// TODO: this is not implemented or verified yet for harmony.
func (hmy *Harmony) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return hmy.BlockChain.SubscribeLogsEvent(ch)
}
