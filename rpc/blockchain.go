package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/harmony-one/harmony/core/types"
	internal_bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/chain"
	internal_common "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	eth "github.com/harmony-one/harmony/rpc/eth"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/shard"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
)

// PublicBlockchainService provides an API to access the Harmony blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockchainService struct {
	hmy             *hmy.Harmony
	version         Version
	limiter         *rate.Limiter
	rpcBlockFactory rpc_common.BlockFactory
	helper          *bcServiceHelper
	// TEMP SOLUTION to rpc node spamming issue
	limiterGetStakingNetworkInfo    *rate.Limiter
	limiterGetSuperCommittees       *rate.Limiter
	limiterGetCurrentUtilityMetrics *rate.Limiter
}

const (
	DefaultRateLimiterWaitTimeout = 5 * time.Second
	rpcGetBlocksLimit             = 1024
)

// NewPublicBlockchainAPI creates a new API for the RPC interface
func NewPublicBlockchainAPI(hmy *hmy.Harmony, version Version, limiterEnable bool, limit int) rpc.API {
	var limiter *rate.Limiter
	if limiterEnable {
		limiter = rate.NewLimiter(rate.Limit(limit), limit)
		name := reflect.TypeOf(limiter).Elem().Name()
		rpcRateLimitCounterVec.With(prometheus.Labels{
			"limiter_name": name,
		}).Add(float64(0))
	}

	s := &PublicBlockchainService{
		hmy:                             hmy,
		version:                         version,
		limiter:                         limiter,
		limiterGetStakingNetworkInfo:    rate.NewLimiter(5, 10),
		limiterGetSuperCommittees:       rate.NewLimiter(5, 10),
		limiterGetCurrentUtilityMetrics: rate.NewLimiter(5, 10),
	}
	s.helper = s.newHelper()

	switch version {
	case V1:
		s.rpcBlockFactory = v1.NewBlockFactory(s.helper)
	case V2:
		s.rpcBlockFactory = v2.NewBlockFactory(s.helper)
	case Eth:
		s.rpcBlockFactory = eth.NewBlockFactory(s.helper)
	default:
		// This shall not happen for legitimate code.
	}

	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   s,
		Public:    true,
	}
}

// ChainId returns the chain id of the chain - required by MetaMask
func (s *PublicBlockchainService) ChainId(ctx context.Context) (interface{}, error) {
	// Format return base on version
	switch s.version {
	case V1:
		return hexutil.Uint64(s.hmy.ChainID), nil
	case V2:
		return s.hmy.ChainID, nil
	case Eth:
		ethChainID := nodeconfig.GetDefaultConfig().GetNetworkType().ChainConfig().EthCompatibleChainID
		return hexutil.Uint64(ethChainID.Uint64()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// Accounts returns the collection of accounts this node manages
// While this JSON-RPC method is supported, it will not return any accounts.
// Similar to e.g. Infura "unlocking" accounts isn't supported.
// Instead, users should send already signed raw transactions using hmy_sendRawTransaction or eth_sendRawTransaction
func (s *PublicBlockchainService) Accounts() []common.Address {
	return []common.Address{}
}

// getBalanceByBlockNumber returns balance by block number at given eth blockNum without checks
func (s *PublicBlockchainService) getBalanceByBlockNumber(
	ctx context.Context, address string, blockNum rpc.BlockNumber,
) (*big.Int, error) {
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	balance, err := s.hmy.GetBalance(ctx, addr, rpc.BlockNumberOrHashWithNumber(blockNum))
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockchainService) BlockNumber(ctx context.Context) (interface{}, error) {
	// Fetch latest header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}

	// Format return base on version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint64(header.Number().Uint64()), nil
	case V2:
		return header.Number().Uint64(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

func (s *PublicBlockchainService) wait(limiter *rate.Limiter, ctx context.Context) error {
	if limiter != nil {
		deadlineCtx, cancel := context.WithTimeout(ctx, DefaultRateLimiterWaitTimeout)
		defer cancel()
		if !limiter.Allow() {
			name := reflect.TypeOf(limiter).Elem().Name()
			rpcRateLimitCounterVec.With(prometheus.Labels{
				"limiter_name": name,
			}).Inc()
		}

		return limiter.Wait(deadlineCtx)
	}
	return nil
}

// GetBlockByNumber returns the requested block. When blockNum is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
// When withSigners in BlocksArgs is true it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByNumber(
	ctx context.Context, blockNumber BlockNumber, opts interface{},
) (response interface{}, err error) {
	timer := DoMetricRPCRequest(GetBlockByNumber)
	defer DoRPCRequestDuration(GetBlockByNumber, timer)

	err = s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByNumber, RateLimitedNumber)
		return nil, err
	}

	// Process arguments based on version
	blockArgs, err := s.getBlockOptions(opts)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByNumber, FailedNumber)
		return nil, err
	}

	if blockNumber.EthBlockNumber() == rpc.PendingBlockNumber {
		return nil, errors.New("pending block number not implemented")
	}
	var blockNum uint64
	if blockNumber.EthBlockNumber() == rpc.LatestBlockNumber {
		blockNum = s.hmy.BlockChain.CurrentHeader().Number().Uint64()
	} else {
		blockNum = uint64(blockNumber.EthBlockNumber().Int64())
	}

	blk := s.hmy.BlockChain.GetBlockByNumber(blockNum)
	// Some Ethereum tools (such as Truffle) rely on being able to query for future blocks without the chain returning errors.
	// These tools implement retry mechanisms that will query & retry for a given block until it has been finalized.
	// Throwing an error like "requested block number greater than current block number" breaks this retry functionality.
	// Disable isBlockGreaterThanLatest checks for Ethereum RPC:s, but keep them in place for legacy hmy_ RPC:s for now to ensure backwards compatibility
	if blk == nil {
		DoMetricRPCQueryInfo(GetBlockByNumber, FailedNumber)
		if s.version == Eth {
			return nil, nil
		}
		return nil, ErrRequestedBlockTooHigh
	}
	// Format the response according to version
	rpcBlock, err := s.rpcBlockFactory.NewBlock(blk, blockArgs)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByNumber, FailedNumber)
		return nil, err
	}
	return rpcBlock, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByHash(
	ctx context.Context, blockHash common.Hash, opts interface{},
) (response interface{}, err error) {
	timer := DoMetricRPCRequest(GetBlockByHash)
	defer DoRPCRequestDuration(GetBlockByHash, timer)

	err = s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByHash, RateLimitedNumber)
		return nil, err
	}

	// Process arguments based on version
	blockArgs, err := s.getBlockOptions(opts)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByHash, FailedNumber)
		return nil, err
	}

	// Fetch the block
	blk, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil || blk == nil {
		DoMetricRPCQueryInfo(GetBlockByHash, FailedNumber)
		return nil, err
	}

	// Format the response according to version
	rpcBlock, err := s.rpcBlockFactory.NewBlock(blk, blockArgs)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByNumber, FailedNumber)
		return nil, err
	}
	return rpcBlock, err
}

// GetBlockByNumberNew is an alias for GetBlockByNumber using rpc_common.BlockArgs
func (s *PublicBlockchainService) GetBlockByNumberNew(
	ctx context.Context, blockNum BlockNumber, blockArgs *rpc_common.BlockArgs,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockByNumberNew)
	defer DoRPCRequestDuration(GetBlockByNumberNew, timer)

	res, err := s.GetBlockByNumber(ctx, blockNum, blockArgs)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByNumberNew, FailedNumber)
	}
	return res, err
}

// GetBlockByHashNew is an alias for GetBlocksByHash using rpc_common.BlockArgs
func (s *PublicBlockchainService) GetBlockByHashNew(
	ctx context.Context, blockHash common.Hash, blockArgs *rpc_common.BlockArgs,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockByHashNew)
	defer DoRPCRequestDuration(GetBlockByHashNew, timer)

	res, err := s.GetBlockByHash(ctx, blockHash, blockArgs)
	if err != nil {
		DoMetricRPCQueryInfo(GetBlockByHashNew, FailedNumber)
	}
	return res, err
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockchainService) GetBlocks(
	ctx context.Context, blockNumberStart BlockNumber,
	blockNumberEnd BlockNumber, blockArgs *rpc_common.BlockArgs,
) ([]interface{}, error) {
	timer := DoMetricRPCRequest(GetBlocks)
	defer DoRPCRequestDuration(GetBlocks, timer)

	blockStart := blockNumberStart.Int64()
	blockEnd := blockNumberEnd.Int64()
	if blockNumberEnd.EthBlockNumber() == rpc.LatestBlockNumber {
		blockEnd = s.hmy.BlockChain.CurrentHeader().Number().Int64()
	}
	if blockEnd >= blockStart && blockEnd-blockStart > rpcGetBlocksLimit {
		return nil, fmt.Errorf("GetBlocks query must be smaller than size %v", rpcGetBlocksLimit)
	}

	// Fetch blocks within given range
	result := make([]interface{}, 0)
	for i := blockStart; i <= blockEnd; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		blockNum := BlockNumber(i)
		if blockNum.Int64() > s.hmy.CurrentBlock().Number().Int64() {
			break
		}
		// rpcBlock is already formatted according to version
		rpcBlock, err := s.GetBlockByNumber(ctx, blockNum, blockArgs)
		if err != nil {
			DoMetricRPCQueryInfo(GetBlockByNumber, FailedNumber)
			utils.Logger().Warn().Err(err).Msg("RPC Get Blocks Error")
			return nil, err
		}
		if rpcBlock != nil {
			result = append(result, rpcBlock)
		}
	}
	return result, nil
}

// IsLastBlock checks if block is last epoch block.
func (s *PublicBlockchainService) IsLastBlock(ctx context.Context, blockNum uint64) (bool, error) {
	timer := DoMetricRPCRequest(IsLastBlock)
	defer DoRPCRequestDuration(IsLastBlock, timer)

	if !isBeaconShard(s.hmy) {
		return false, ErrNotBeaconShard
	}
	return shard.Schedule.IsLastBlock(blockNum), nil
}

// EpochLastBlock returns epoch last block.
func (s *PublicBlockchainService) EpochLastBlock(ctx context.Context, epoch uint64) (uint64, error) {
	timer := DoMetricRPCRequest(EpochLastBlock)
	defer DoRPCRequestDuration(EpochLastBlock, timer)

	if !isBeaconShard(s.hmy) {
		return 0, ErrNotBeaconShard
	}
	return shard.Schedule.EpochLastBlock(epoch), nil
}

// GetBlockSigners returns signers for a particular block.
func (s *PublicBlockchainService) GetBlockSigners(
	ctx context.Context, blockNumber BlockNumber,
) ([]string, error) {
	timer := DoMetricRPCRequest(GetBlockSigners)
	defer DoRPCRequestDuration(GetBlockSigners, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("cannot get signer keys for pending blocks")
	}
	// Ensure correct block
	if blockNum.Int64() == 0 || blockNum.Int64() >= s.hmy.CurrentBlock().Number().Int64() {
		return []string{}, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}
	var bn uint64
	if blockNum == rpc.LatestBlockNumber {
		bn = s.hmy.CurrentBlock().NumberU64()
	} else {
		bn = uint64(blockNum.Int64())
	}
	blk := s.hmy.BlockChain.GetBlockByNumber(bn)
	if blk == nil {
		return nil, errors.New("unknown block")
	}
	// Fetch signers
	return s.helper.GetSigners(blk)
}

// GetBlockSignerKeys returns bls public keys that signed the block.
func (s *PublicBlockchainService) GetBlockSignerKeys(
	ctx context.Context, blockNumber BlockNumber,
) ([]string, error) {
	timer := DoMetricRPCRequest(GetBlockSignerKeys)
	defer DoRPCRequestDuration(GetBlockSignerKeys, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
	if blockNum == rpc.PendingBlockNumber {
		return nil, errors.New("cannot get signer keys for pending blocks")
	}
	// Ensure correct block
	if blockNum.Int64() == 0 || blockNum.Int64() >= s.hmy.CurrentBlock().Number().Int64() {
		return []string{}, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}
	var bn uint64
	if blockNum == rpc.LatestBlockNumber {
		bn = s.hmy.CurrentBlock().NumberU64()
	} else {
		bn = uint64(blockNum.Int64())
	}
	// Fetch signers
	return s.helper.GetBLSSigners(bn)
}

// GetBlockReceipts returns all transaction receipts for a particular block.
func (s *PublicBlockchainService) GetBlockReceipts(
	ctx context.Context, blockHash common.Hash,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetBlockReceipts)
	defer DoRPCRequestDuration(GetBlockReceipts, timer)

	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	receipts, err := s.hmy.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	rmap := make(map[common.Hash]*types.Receipt, len(receipts))
	for _, r := range receipts {
		rmap[r.TxHash] = r
	}

	txns := make([]types.CoreTransaction, 0,
		block.Transactions().Len()+block.StakingTransactions().Len())
	for _, tx := range block.Transactions() {
		txns = append(txns, tx)
	}
	for _, tx := range block.StakingTransactions() {
		txns = append(txns, tx)
	}

	if len(txns) != len(rmap) {
		return nil, fmt.Errorf(
			"transactions (%d) and receipts (%d) count mismatch",
			len(txns), len(rmap))
	}

	rpcr := make([]StructuredResponse, 0, len(txns))

	for i, tx := range txns {
		index := uint64(i)

		r, err := interface{}(nil), error(nil)
		switch s.version {
		case V1:
			r, err = v1.NewReceipt(tx, blockHash, block.NumberU64(), index, rmap[tx.Hash()])
		case V2:
			r, err = v2.NewReceipt(tx, blockHash, block.NumberU64(), index, rmap[tx.Hash()])
		case Eth:
			if tx, ok := tx.(*types.Transaction); ok {
				r, err = eth.NewReceipt(tx.ConvertToEth(), blockHash, block.NumberU64(), index, rmap[tx.Hash()])
			}
		default:
			return nil, ErrUnknownRPCVersion
		}
		if err != nil {
			return nil, err
		}

		sr, err := NewStructuredResponse(r)
		if err != nil {
			return nil, err
		}
		rpcr = append(rpcr, sr)
	}

	return rpcr, nil
}

// IsBlockSigner returns true if validator with address signed blockNum block.
func (s *PublicBlockchainService) IsBlockSigner(
	ctx context.Context, blockNumber BlockNumber, address string,
) (bool, error) {
	timer := DoMetricRPCRequest(IsBlockSigner)
	defer DoRPCRequestDuration(IsBlockSigner, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure correct block
	if blockNum.Int64() == 0 {
		return false, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return false, ErrRequestedBlockTooHigh
	}
	var bn uint64
	if blockNum == rpc.PendingBlockNumber {
		return false, errors.New("no signing data for pending block number")
	} else if blockNum == rpc.LatestBlockNumber {
		bn = s.hmy.BlockChain.CurrentBlock().NumberU64()
	} else {
		bn = uint64(blockNum.Int64())
	}

	// Fetch signers
	return s.helper.IsSigner(address, bn)
}

// GetSignedBlocks returns how many blocks a particular validator signed for
// last blocksPeriod (1 epoch's worth of blocks).
func (s *PublicBlockchainService) GetSignedBlocks(
	ctx context.Context, address string,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetSignedBlocks)
	defer DoRPCRequestDuration(GetSignedBlocks, timer)

	// Fetch the number of signed blocks within default period
	curEpoch := s.hmy.CurrentBlock().Epoch()
	var totalSigned uint64
	if !s.hmy.ChainConfig().IsStaking(curEpoch) {
		// calculate signed before staking epoch
		totalSigned := uint64(0)
		lastBlock := uint64(0)
		blockHeight := s.hmy.CurrentBlock().Number().Uint64()
		instance := shard.Schedule.InstanceForEpoch(s.hmy.CurrentBlock().Epoch())
		if blockHeight >= instance.BlocksPerEpoch() {
			lastBlock = blockHeight - instance.BlocksPerEpoch() + 1
		}
		for i := lastBlock; i <= blockHeight; i++ {
			signed, err := s.IsBlockSigner(ctx, BlockNumber(i), address)
			if err == nil && signed {
				totalSigned++
			}
		}
	} else {
		ethAddr, err := internal_common.Bech32ToAddress(address)
		if err != nil {
			return nil, err
		}
		curVal, err := s.hmy.BlockChain.ReadValidatorInformation(ethAddr)
		if err != nil {
			return nil, err
		}
		prevVal, err := s.hmy.BlockChain.ReadValidatorSnapshot(ethAddr)
		if err != nil {
			return nil, err
		}
		signedInEpoch := new(big.Int).Sub(curVal.Counters.NumBlocksSigned, prevVal.Validator.Counters.NumBlocksSigned)
		if signedInEpoch.Cmp(common.Big0) < 0 {
			return nil, errors.New("negative signed in epoch")
		}
		totalSigned = signedInEpoch.Uint64()
	}

	// Format the response according to the version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint64(totalSigned), nil
	case V2:
		return totalSigned, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetEpoch returns current epoch.
func (s *PublicBlockchainService) GetEpoch(ctx context.Context) (interface{}, error) {
	timer := DoMetricRPCRequest(GetEpoch)
	defer DoRPCRequestDuration(GetEpoch, timer)

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return "", err
	}
	epoch := header.Epoch().Uint64()

	// Format the response according to the version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint64(epoch), nil
	case V2:
		return epoch, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetLeader returns current shard leader.
func (s *PublicBlockchainService) GetLeader(ctx context.Context) (string, error) {
	timer := DoMetricRPCRequest(GetLeader)
	defer DoRPCRequestDuration(GetLeader, timer)

	// Fetch Header
	blk := s.hmy.BlockChain.CurrentBlock()
	// Response output is the same for all versions
	leader := s.helper.GetLeader(blk)
	return leader, nil
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockchainService) GetShardingStructure(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetShardingStructure)
	defer DoRPCRequestDuration(GetShardingStructure, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetShardingStructure, RateLimitedNumber)
		return nil, err
	}

	// Get header and number of shards.
	epoch := s.hmy.CurrentBlock().Epoch()
	numShard := shard.Schedule.InstanceForEpoch(epoch).NumShards()

	// Return sharding structure for each case (response output is the same for all versions)
	return shard.Schedule.GetShardingStructure(int(numShard), int(s.hmy.ShardID)), nil
}

// GetShardID returns shard ID of the requested node.
func (s *PublicBlockchainService) GetShardID(ctx context.Context) (int, error) {
	// Response output is the same for all versions
	return int(s.hmy.ShardID), nil
}

// GetBalanceByBlockNumber returns balance by block number
func (s *PublicBlockchainService) GetBalanceByBlockNumber(
	ctx context.Context, address string, blockNumber BlockNumber,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBalanceByBlockNumber)
	defer DoRPCRequestDuration(GetBalanceByBlockNumber, timer)

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch balance
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetBalanceByBlockNumber, FailedNumber)
		return nil, ErrRequestedBlockTooHigh
	}
	balance, err := s.getBalanceByBlockNumber(ctx, address, blockNum)
	if err != nil {
		DoMetricRPCQueryInfo(GetBalanceByBlockNumber, FailedNumber)
		return nil, err
	}

	// Format return base on version
	switch s.version {
	case V1, Eth:
		return (*hexutil.Big)(balance), nil
	case V2:
		return balance, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// LatestHeader returns the latest header information
func (s *PublicBlockchainService) LatestHeader(ctx context.Context) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(LatestHeader)
	defer DoRPCRequestDuration(LatestHeader, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(LatestHeader, RateLimitedNumber)
		return nil, err
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		DoMetricRPCQueryInfo(LatestHeader, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return NewStructuredResponse(NewHeaderInformation(header, leader))
}

// GetLatestChainHeaders ..
func (s *PublicBlockchainService) GetLatestChainHeaders(
	ctx context.Context,
) (StructuredResponse, error) {
	// Response output is the same for all versions
	timer := DoMetricRPCRequest(GetLatestChainHeaders)
	defer DoRPCRequestDuration(GetLatestChainHeaders, timer)
	return NewStructuredResponse(s.hmy.GetLatestChainHeaders())
}

// GetLastCrossLinks ..
func (s *PublicBlockchainService) GetLastCrossLinks(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetLastCrossLinks)
	defer DoRPCRequestDuration(GetLastCrossLinks, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetLastCrossLinks, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetLastCrossLinks, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch crosslinks
	crossLinks, err := s.hmy.GetLastCrossLinks()
	if err != nil {
		DoMetricRPCQueryInfo(GetLastCrossLinks, FailedNumber)
		return nil, err
	}

	// Format response, all output is the same for all versions
	responseSlice := []StructuredResponse{}
	for _, el := range crossLinks {
		response, err := NewStructuredResponse(el)
		if err != nil {
			DoMetricRPCQueryInfo(GetLastCrossLinks, FailedNumber)
			return nil, err
		}
		responseSlice = append(responseSlice, response)
	}
	return responseSlice, nil
}

// GetHeaderByNumber returns block header at given number
func (s *PublicBlockchainService) GetHeaderByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetHeaderByNumber)
	defer DoRPCRequestDuration(GetHeaderByNumber, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetHeaderByNumber, RateLimitedNumber)
		return nil, err
	}

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure valid block number
	if s.version != Eth && isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetHeaderByNumber, FailedNumber)
		return nil, ErrRequestedBlockTooHigh
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, blockNum)
	if header != nil && err == nil {
		// Response output is the same for all versions
		leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
		return NewStructuredResponse(NewHeaderInformation(header, leader))
	}
	return nil, err
}

// Result structs for GetProof
type AccountResult struct {
	Address      common.Address  `json:"address"`
	AccountProof []string        `json:"accountProof"`
	Balance      *hexutil.Big    `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        hexutil.Uint64  `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	StorageProof []StorageResult `json:"storageProof"`
}

type StorageResult struct {
	Key   string       `json:"key"`
	Value *hexutil.Big `json:"value"`
	Proof []string     `json:"proof"`
}

// GetHeaderByNumberRLPHex returns block header at given number by `hex(rlp(header))`
func (s *PublicBlockchainService) GetProof(
	ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpc.BlockNumberOrHash) (ret *AccountResult, err error) {
	timer := DoMetricRPCRequest(GetProof)
	defer DoRPCRequestDuration(GetProof, timer)

	defer func() {
		if ret == nil || err != nil {
			DoMetricRPCQueryInfo(GetProof, FailedNumber)
		}
	}()

	err = s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetProof, RateLimitedNumber)
		return
	}

	state, _, err := s.hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		return nil, err
	}

	storageTrie, errTr := state.StorageTrie(address)
	if errTr != nil {
		return
	}
	storageHash := types.EmptyRootHash
	codeHash := state.GetCodeHash(address)
	storageProof := make([]StorageResult, len(storageKeys))

	// if we have a storageTrie, (which means the account exists), we can update the storagehash
	if storageTrie != nil {
		storageHash = storageTrie.Hash()
	} else {
		// no storageTrie means the account does not exist, so the codeHash is the hash of an empty bytearray.
		codeHash = crypto.Keccak256Hash(nil)
	}

	// create the proof for the storageKeys
	for i, key := range storageKeys {
		if storageTrie != nil {
			proof, storageError := state.GetStorageProof(address, common.HexToHash(key))
			if storageError != nil {
				err = storageError
				return
			}
			storageProof[i] = StorageResult{key, (*hexutil.Big)(state.GetState(address, common.HexToHash(key)).Big()), toHexSlice(proof)}
		} else {
			storageProof[i] = StorageResult{key, &hexutil.Big{}, []string{}}
		}
	}

	// create the accountProof
	accountProof, err := state.GetProof(address)
	if err != nil {
		return
	}

	ret, err = &AccountResult{
		Address:      address,
		AccountProof: toHexSlice(accountProof),
		Balance:      (*hexutil.Big)(state.GetBalance(address)),
		CodeHash:     codeHash,
		Nonce:        hexutil.Uint64(state.GetNonce(address)),
		StorageHash:  storageHash,
		StorageProof: storageProof,
	}, state.Error()
	return
}

// toHexSlice creates a slice of hex-strings based on []byte.
func toHexSlice(b [][]byte) []string {
	r := make([]string, len(b))
	for i := range b {
		r[i] = hexutil.Encode(b[i])
	}
	return r
}

// GetHeaderByNumberRLPHex returns block header at given number by `hex(rlp(header))`
func (s *PublicBlockchainService) GetHeaderByNumberRLPHex(
	ctx context.Context, blockNumber BlockNumber,
) (string, error) {
	timer := DoMetricRPCRequest(GetHeaderByNumberRLPHex)
	defer DoRPCRequestDuration(GetHeaderByNumberRLPHex, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetHeaderByNumberRLPHex, RateLimitedNumber)
		return "", err
	}

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure valid block number
	if s.version != Eth && isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetHeaderByNumberRLPHex, FailedNumber)
		return "", ErrRequestedBlockTooHigh
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, blockNum)
	if header != nil && err == nil {
		// Response output is the same for all versions
		val, _ := rlp.EncodeToBytes(header)
		return hex.EncodeToString(val), nil
	}
	return "", err
}

// GetCurrentUtilityMetrics ..
func (s *PublicBlockchainService) GetCurrentUtilityMetrics(
	ctx context.Context,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetCurrentUtilityMetrics)
	defer DoRPCRequestDuration(GetCurrentUtilityMetrics, timer)

	err := s.wait(s.limiterGetCurrentUtilityMetrics, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetCurrentUtilityMetrics, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetCurrentUtilityMetrics, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch metrics
	metrics, err := s.hmy.GetCurrentUtilityMetrics()
	if err != nil {
		DoMetricRPCQueryInfo(GetCurrentUtilityMetrics, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(metrics)
}

// GetSuperCommittees ..
func (s *PublicBlockchainService) GetSuperCommittees(
	ctx context.Context,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetSuperCommittees)
	defer DoRPCRequestDuration(GetSuperCommittees, timer)

	err := s.wait(s.limiterGetSuperCommittees, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetSuperCommittees, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetSuperCommittees, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch super committees
	cmt, err := s.hmy.GetSuperCommittees()
	if err != nil {
		DoMetricRPCQueryInfo(GetSuperCommittees, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(cmt)
}

// GetCurrentBadBlocks ..
func (s *PublicBlockchainService) GetCurrentBadBlocks(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetCurrentBadBlocks)
	defer DoRPCRequestDuration(GetCurrentBadBlocks, timer)

	err := s.wait(s.limiter, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetCurrentBadBlocks, RateLimitedNumber)
		return nil, err
	}

	// Fetch bad blocks and format
	badBlocks := []StructuredResponse{}
	for _, blk := range s.hmy.GetCurrentBadBlocks() {
		// Response output is the same for all versions
		fmtBadBlock, err := NewStructuredResponse(blk)
		if err != nil {
			DoMetricRPCQueryInfo(GetCurrentBadBlocks, FailedNumber)
			return nil, err
		}
		badBlocks = append(badBlocks, fmtBadBlock)
	}

	return badBlocks, nil
}

// GetTotalSupply ..
func (s *PublicBlockchainService) GetTotalSupply(
	ctx context.Context,
) (numeric.Dec, error) {
	timer := DoMetricRPCRequest(GetTotalSupply)
	defer DoRPCRequestDuration(GetTotalSupply, timer)
	return stakingReward.GetTotalTokens(s.hmy.BlockChain)
}

// GetCirculatingSupply ...
func (s *PublicBlockchainService) GetCirculatingSupply(
	ctx context.Context,
) (numeric.Dec, error) {
	timer := DoMetricRPCRequest(GetCirculatingSupply)
	defer DoRPCRequestDuration(GetCirculatingSupply, timer)
	return chain.GetCirculatingSupply(s.hmy.BlockChain)
}

// GetStakingNetworkInfo ..
func (s *PublicBlockchainService) GetStakingNetworkInfo(
	ctx context.Context,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetStakingNetworkInfo)
	defer DoRPCRequestDuration(GetStakingNetworkInfo, timer)

	err := s.wait(s.limiterGetStakingNetworkInfo, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	totalStaking := s.hmy.GetTotalStakingSnapshot()
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, err
	}
	medianSnapshot, err := s.hmy.GetMedianRawStakeSnapshot()
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, err
	}
	epochLastBlock, err := s.EpochLastBlock(ctx, header.Epoch().Uint64())
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, err
	}
	totalSupply, err := s.GetTotalSupply(ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, err
	}
	circulatingSupply, err := s.GetCirculatingSupply(ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetStakingNetworkInfo, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(StakingNetworkInfo{
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		EpochLastBlock:    epochLastBlock,
		TotalStaking:      totalStaking,
		MedianRawStake:    medianSnapshot.MedianStake,
	})
}

const (
	// If peer have block height difference smaller or equal to 10 blocks, the node is considered inSync
	inSyncTolerance = 10
)

// InSync returns if shard chain is syncing
func (s *PublicBlockchainService) InSync(ctx context.Context) (bool, error) {
	timer := DoMetricRPCRequest(InSync)
	defer DoRPCRequestDuration(InSync, timer)
	inSync, _, diff := s.hmy.NodeAPI.SyncStatus(s.hmy.BlockChain.ShardID())
	if !inSync && diff <= inSyncTolerance {
		inSync = true
	}
	return inSync, nil
}

// BeaconInSync returns if beacon chain is syncing
func (s *PublicBlockchainService) BeaconInSync(ctx context.Context) (bool, error) {
	timer := DoMetricRPCRequest(BeaconInSync)
	defer DoRPCRequestDuration(BeaconInSync, timer)
	inSync, _, diff := s.hmy.NodeAPI.SyncStatus(s.hmy.BeaconChain.ShardID())
	if !inSync && diff <= inSyncTolerance {
		inSync = true
	}
	return inSync, nil
}

// getBlockOptions block args given an interface option from RPC params.
func (s *PublicBlockchainService) getBlockOptions(opts interface{}) (*rpc_common.BlockArgs, error) {
	blockArgs, ok := opts.(*rpc_common.BlockArgs)
	if ok {
		return blockArgs, nil
	}
	switch s.version {
	case V1, Eth:
		fullTx, ok := opts.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid type for block arguments")
		}
		return &rpc_common.BlockArgs{
			WithSigners: false,
			FullTx:      fullTx,
			InclStaking: true,
		}, nil
	case V2:
		parsedBlockArgs := rpc_common.BlockArgs{}
		if err := parsedBlockArgs.UnmarshalFromInterface(opts); err != nil {
			return nil, err
		}
		return &parsedBlockArgs, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

func (s *PublicBlockchainService) GetFullHeader(
	ctx context.Context, blockNumber BlockNumber,
) (response StructuredResponse, err error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure valid block number
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	var rpcHeader interface{}
	switch s.version {
	case V2:
		rpcHeader, err = v2.NewBlockHeader(header)
	default:
		return nil, ErrUnknownRPCVersion
	}
	if err != nil {
		return nil, err
	}

	response, err = NewStructuredResponse(rpcHeader)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func isBlockGreaterThanLatest(hmy *hmy.Harmony, blockNum rpc.BlockNumber) bool {
	// rpc.BlockNumber is int64 (latest = -1. pending = -2) and currentBlockNum is uint64.
	if blockNum == rpc.PendingBlockNumber {
		return true
	}
	if blockNum == rpc.LatestBlockNumber {
		return false
	}
	return uint64(blockNum) > hmy.CurrentBlock().NumberU64()
}

func (s *PublicBlockchainService) SetNodeToBackupMode(ctx context.Context, isBackup bool) (bool, error) {
	timer := DoMetricRPCRequest(SetNodeToBackupMode)
	defer DoRPCRequestDuration(SetNodeToBackupMode, timer)
	return s.hmy.NodeAPI.SetNodeBackupMode(isBackup), nil
}

const (
	blockCacheSize      = 2048
	signersCacheSize    = blockCacheSize
	stakingTxsCacheSize = blockCacheSize
	leaderCacheSize     = blockCacheSize
)

type (
	// bcServiceHelper is the getHelper for block factory. Implements
	// rpc_common.BlockDataProvider
	bcServiceHelper struct {
		version Version
		hmy     *hmy.Harmony
		cache   *bcServiceCache
	}

	bcServiceCache struct {
		signersCache    *lru.Cache // numberU64 -> []string
		stakingTxsCache *lru.Cache // numberU64 -> interface{} (v1.StakingTransactions / v2.StakingTransactions)
		leaderCache     *lru.Cache // numberUint64 -> string
	}
)

func (s *PublicBlockchainService) newHelper() *bcServiceHelper {
	signerCache, _ := lru.New(signersCacheSize)
	stakingTxsCache, _ := lru.New(stakingTxsCacheSize)
	leaderCache, _ := lru.New(leaderCacheSize)
	cache := &bcServiceCache{
		signersCache:    signerCache,
		stakingTxsCache: stakingTxsCache,
		leaderCache:     leaderCache,
	}
	return &bcServiceHelper{
		version: s.version,
		hmy:     s.hmy,
		cache:   cache,
	}
}

func (helper *bcServiceHelper) GetLeader(b *types.Block) string {
	x, ok := helper.cache.leaderCache.Get(b.NumberU64())
	if ok && x != nil {
		return x.(string)
	}
	leader := helper.hmy.GetLeaderAddress(b.Coinbase(), b.Epoch())
	helper.cache.leaderCache.Add(b.NumberU64(), leader)
	return leader
}

func (helper *bcServiceHelper) GetStakingTxs(b *types.Block) (interface{}, error) {
	x, ok := helper.cache.stakingTxsCache.Get(b.NumberU64())
	if ok && x != nil {
		return x, nil
	}
	var (
		rpcStakings interface{}
		err         error
	)
	switch helper.version {
	case V1:
		rpcStakings, err = v1.StakingTransactionsFromBlock(b)
	case V2:
		rpcStakings, err = v2.StakingTransactionsFromBlock(b)
	case Eth:
		err = errors.New("staking transaction data is unsupported to Eth service")
	default:
		err = fmt.Errorf("unsupported version %v", helper.version)
	}
	if err != nil {
		return nil, err
	}
	helper.cache.stakingTxsCache.Add(b.NumberU64(), rpcStakings)
	return rpcStakings, nil
}

func (helper *bcServiceHelper) GetStakingTxHashes(b *types.Block) []common.Hash {
	stkTxs := b.StakingTransactions()

	res := make([]common.Hash, 0, len(stkTxs))
	for _, tx := range stkTxs {
		res = append(res, tx.Hash())
	}
	return res
}

// signerData is the cached signing data for a block
type signerData struct {
	signers    []string           // one address for signers
	blsSigners []string           // bls hex for signers
	slots      shard.SlotList     // computed slots for epoch shard committee
	mask       *internal_bls.Mask // mask for the block
}

func (helper *bcServiceHelper) GetSigners(b *types.Block) ([]string, error) {
	sd, err := helper.getSignerData(b.NumberU64())
	if err != nil {
		return nil, err
	}
	return sd.signers, nil
}

func (helper *bcServiceHelper) GetBLSSigners(bn uint64) ([]string, error) {
	sd, err := helper.getSignerData(bn)
	if err != nil {
		return nil, err
	}
	return sd.blsSigners, nil
}

func (helper *bcServiceHelper) IsSigner(oneAddr string, bn uint64) (bool, error) {
	sd, err := helper.getSignerData(bn)
	if err != nil {
		return false, err
	}
	for _, signer := range sd.signers {
		if oneAddr == signer {
			return true, nil
		}
	}
	return false, nil
}

func (helper *bcServiceHelper) getSignerData(bn uint64) (*signerData, error) {
	x, ok := helper.cache.signersCache.Get(bn)
	if ok && x != nil {
		return x.(*signerData), nil
	}
	sd, err := getSignerData(helper.hmy, bn)
	if err != nil {
		return nil, errors.Wrap(err, "getSignerData")
	}
	helper.cache.signersCache.Add(bn, sd)
	return sd, nil
}

func getSignerData(hmy *hmy.Harmony, number uint64) (*signerData, error) {
	slots, mask, err := hmy.GetBlockSigners(context.Background(), rpc.BlockNumber(number))
	if err != nil {
		return nil, err
	}
	signers := make([]string, 0, len(slots))
	blsSigners := make([]string, 0, len(slots))
	for _, validator := range slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		if ok, err := mask.KeyEnabled(validator.BLSPublicKey); err == nil && ok {
			blsSigners = append(blsSigners, validator.BLSPublicKey.Hex())
			signers = append(signers, oneAddress)
		}
	}
	return &signerData{
		signers:    signers,
		blsSigners: blsSigners,
		slots:      slots,
		mask:       mask,
	}, nil
}
