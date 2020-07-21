package hmy

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/bloombits"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internal_bls "github.com/harmony-one/harmony/crypto/bls"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
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
func (hmy *Harmony) GetBlockSigners(ctx context.Context, blockNr rpc.BlockNumber) (shard.SlotList, *internal_bls.Mask, error) {
	blk, err := hmy.BlockByNumber(ctx, blockNr)
	if err != nil {
		return nil, nil, err
	}
	blockWithSigners, err := hmy.BlockByNumber(ctx, blockNr+1)
	if err != nil {
		return nil, nil, err
	}
	committee, err := hmy.GetValidators(blk.Epoch())
	if err != nil {
		return nil, nil, err
	}
	pubKeys := make([]*bls_core.PublicKey, len(committee.Slots))
	for i, validator := range committee.Slots {
		if pubKeys[i], err = bls.BytesToBLSPublicKey(validator.BLSPublicKey[:]); err != nil {
			return nil, nil, err
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

// GetLatestChainHeaders ..
func (hmy *Harmony) GetLatestChainHeaders() *block.HeaderPair {
	return &block.HeaderPair{
		BeaconHeader: hmy.BeaconChain.CurrentHeader(),
		ShardHeader:  hmy.BlockChain.CurrentHeader(),
	}
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

// GetBlock ...
func (hmy *Harmony) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return hmy.BlockChain.GetBlockByHash(hash), nil
}

// GetCurrentBadBlocks ..
func (hmy *Harmony) GetCurrentBadBlocks() []core.BadBlock {
	return hmy.BlockChain.BadBlocks()
}

// GetBalance returns balance of an given address.
func (hmy *Harmony) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	s, _, err := hmy.StateAndHeaderByNumber(ctx, blockNr)
	if s == nil || err != nil {
		return nil, err
	}
	return s.GetBalance(address), s.Error()
}

// BlockByNumber ...
func (hmy *Harmony) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return hmy.BlockChain.CurrentBlock(), nil
	}
	return hmy.BlockChain.GetBlockByNumber(uint64(blockNr)), nil
}

// HeaderByNumber ...
func (hmy *Harmony) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*block.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, errors.New("not implemented")
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return hmy.BlockChain.CurrentBlock().Header(), nil
	}
	return hmy.BlockChain.GetHeaderByNumber(uint64(blockNr)), nil
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
func (hmy *Harmony) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.DB, *block.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		return nil, nil, errors.New("not implemented")
	}
	// Otherwise resolve the block number and return its state
	header, err := hmy.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := hmy.BlockChain.StateAt(header.Root())
	return stateDb, header, err
}

// GetLeaderAddress returns the one address of the leader
func (hmy *Harmony) GetLeaderAddress(a common.Address, e *big.Int) string {
	if hmy.IsStakingEpoch(e) {
		if leader, exists := hmy.leaderCache.Get(a); exists {
			bech32, _ := internal_common.AddressToBech32(leader.(common.Address))
			return bech32
		}
		committee, err := hmy.GetValidators(e)
		if err != nil {
			return ""
		}
		for _, v := range committee.Slots {
			addr := utils.GetAddressFromBLSPubKeyBytes(v.BLSPublicKey[:])
			hmy.leaderCache.Add(addr, v.EcdsaAddress)
			if addr == a {
				bech32, _ := internal_common.AddressToBech32(v.EcdsaAddress)
				return bech32
			}
		}
		return "" // Did not find matching address
	}
	bech32, _ := internal_common.AddressToBech32(a)
	return bech32
}

// Filter related APIs

// GetLogs ...
func (hmy *Harmony) GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error) {
	receipts := hmy.BlockChain.GetReceiptsByHash(blockHash)
	if receipts == nil {
		return nil, errors.New("Missing receipts")
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
