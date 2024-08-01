package hmy_boot

import (
	"context"
	"math/big"

	"github.com/harmony-one/harmony/eth/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/pkg/errors"
)

// ChainConfig ...
func (hmyboot *BootService) ChainConfig() *params.ChainConfig {
	return hmyboot.BlockChain.Config()
}

// GetShardState ...
func (hmyboot *BootService) GetShardState() (*shard.State, error) {
	return hmyboot.BlockChain.ReadShardState(hmyboot.BlockChain.CurrentHeader().Epoch())
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
func (hmyboot *BootService) GetDetailedBlockSignerInfo(
	ctx context.Context, blk *types.Block,
) (*DetailedBlockSignerInfo, error) {
	parentBlk, err := hmyboot.BlockByNumber(ctx, rpc.BlockNumber(blk.NumberU64()-1))
	if err != nil {
		return nil, err
	}
	parentShardState, err := hmyboot.BlockChain.ReadShardState(parentBlk.Epoch())
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

// GetLastCrossLinks ..
func (hmyboot *BootService) GetLastCrossLinks() ([]*types.CrossLink, error) {
	crossLinks := []*types.CrossLink{}
	for i := uint32(1); i < shard.Schedule.InstanceForEpoch(hmyboot.CurrentBlock().Epoch()).NumShards(); i++ {
		link, err := hmyboot.BlockChain.ReadShardLastCrossLink(i)
		if err != nil {
			return nil, err
		}
		crossLinks = append(crossLinks, link)
	}

	return crossLinks, nil
}

// CurrentBlock ...
func (hmyboot *BootService) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(hmyboot.BlockChain.CurrentHeader())
}

// CurrentHeader returns the current header from the local chain.
func (hmyboot *BootService) CurrentHeader() *block.Header {
	return hmyboot.BlockChain.CurrentHeader()
}

// GetBlock returns block by hash.
func (hmyboot *BootService) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return hmyboot.BlockChain.GetBlockByHash(hash), nil
}

// GetHeader returns header by hash.
func (hmyboot *BootService) GetHeader(ctx context.Context, hash common.Hash) (*block.Header, error) {
	return hmyboot.BlockChain.GetHeaderByHash(hash), nil
}

// GetCurrentBadBlocks ..
func (hmyboot *BootService) GetCurrentBadBlocks() []core.BadBlock {
	return hmyboot.BlockChain.BadBlocks()
}

func (hmyboot *BootService) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return hmyboot.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := hmyboot.BlockChain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && hmyboot.BlockChain.GetCanonicalHash(header.Number().Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := hmyboot.BlockChain.GetBlock(hash, header.Number().Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

// GetBalance returns balance of an given address.
func (hmyboot *BootService) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*big.Int, error) {
	s, _, err := hmyboot.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if s == nil || err != nil {
		return nil, err
	}
	return s.GetBalance(address), s.Error()
}

// BlockByNumber ...
func (hmyboot *BootService) BlockByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNum == rpc.LatestBlockNumber {
		return hmyboot.BlockChain.CurrentBlock(), nil
	}
	return hmyboot.BlockChain.GetBlockByNumber(uint64(blockNum)), nil
}

// HeaderByNumber ...
func (hmyboot *BootService) HeaderByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*block.Header, error) {
	// Pending block is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNum == rpc.LatestBlockNumber {
		return hmyboot.BlockChain.CurrentBlock().Header(), nil
	}
	return hmyboot.BlockChain.GetHeaderByNumber(uint64(blockNum)), nil
}

// HeaderByHash ...
func (hmyboot *BootService) HeaderByHash(ctx context.Context, blockHash common.Hash) (*block.Header, error) {
	header := hmyboot.BlockChain.GetHeaderByHash(blockHash)
	if header == nil {
		return nil, errors.New("Header is not found")
	}
	return header, nil
}

// StateAndHeaderByNumber ...
func (hmyboot *BootService) StateAndHeaderByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*state.DB, *block.Header, error) {
	// Pending state is only known by the miner
	if blockNum == rpc.PendingBlockNumber {
		return nil, nil, errors.New("not implemented")
	}
	// Otherwise resolve the block number and return its state
	header, err := hmyboot.HeaderByNumber(ctx, blockNum)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := hmyboot.BlockChain.StateAt(header.Root())
	return stateDb, header, err
}

func (hmyboot *BootService) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.DB, *block.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return hmyboot.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := hmyboot.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && hmyboot.BlockChain.GetCanonicalHash(header.Number().Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := hmyboot.BlockChain.StateAt(header.Root())
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

// Filter related APIs

// GetLogs ...
func (hmyboot *BootService) GetLogs(ctx context.Context, blockHash common.Hash, isEth bool) ([][]*types.Log, error) {
	receipts := hmyboot.BlockChain.GetReceiptsByHash(blockHash)
	if receipts == nil {
		return nil, errors.New("Missing receipts")
	}
	if isEth {
		block := hmyboot.BlockChain.GetBlockByHash(blockHash)
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
func (hmyboot *BootService) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	// TODO(dm): implement
}

// SubscribeNewTxsEvent subscribes new tx event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return hmyboot.TxPool.SubscribeNewTxsEvent(ch)
}

// SubscribeChainEvent subscribes chain event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return hmyboot.BlockChain.SubscribeChainEvent(ch)
}

// SubscribeChainHeadEvent subcribes chain head event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return hmyboot.BlockChain.SubscribeChainHeadEvent(ch)
}

// SubscribeChainSideEvent subcribes chain side event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return hmyboot.BlockChain.SubscribeChainSideEvent(ch)
}

// SubscribeRemovedLogsEvent subcribes removed logs event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return hmyboot.BlockChain.SubscribeRemovedLogsEvent(ch)
}

// SubscribeLogsEvent subcribes log event.
// TODO: this is not implemented or verified yet for harmony.
func (hmyboot *BootService) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return hmyboot.BlockChain.SubscribeLogsEvent(ch)
}
