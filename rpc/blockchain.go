package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"

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

	lru "github.com/hashicorp/golang-lru"
)

// PublicBlockchainService provides an API to access the Harmony blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockchainService struct {
	hmy        *hmy.Harmony
	version    Version
	limiter    *rate.Limiter
	blockCache *lru.Cache
}

const (
	DefaultRateLimiterWaitTimeout = 5 * time.Second
	blockCacheLimit               = 256
)

// NewPublicBlockchainAPI creates a new API for the RPC interface
func NewPublicBlockchainAPI(hmy *hmy.Harmony, version Version, limiterEnable bool, limit int) rpc.API {
	blockCache, _ := lru.New(blockCacheLimit)
	if limiterEnable {
		return rpc.API{
			Namespace: version.Namespace(),
			Version:   APIVersion,
			Service:   &PublicBlockchainService{hmy, version, rate.NewLimiter(rate.Limit(limit), 1), blockCache},
			Public:    true,
		}
	} else {
		return rpc.API{
			Namespace: version.Namespace(),
			Version:   APIVersion,
			Service:   &PublicBlockchainService{hmy, version, nil, blockCache},
			Public:    true,
		}
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

// getBlockOptions is a helper to get block args given an interface option from RPC params.
func (s *PublicBlockchainService) getBlockOptions(opts interface{}) (*rpc_common.BlockArgs, error) {
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

// getBalanceByBlockNumber returns balance by block number at given eth blockNum without checks
func (s *PublicBlockchainService) getBalanceByBlockNumber(
	ctx context.Context, address string, blockNum rpc.BlockNumber,
) (*big.Int, error) {
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	balance, err := s.hmy.GetBalance(ctx, addr, blockNum)
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

func (s *PublicBlockchainService) wait(ctx context.Context) error {
	if s.limiter != nil {
		deadlineCtx, cancel := context.WithTimeout(ctx, DefaultRateLimiterWaitTimeout)
		defer cancel()
		return s.limiter.Wait(deadlineCtx)
	}
	return nil
}

// GetBlockByNumber returns the requested block. When blockNum is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
// When withSigners in BlocksArgs is true it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByNumber(
	ctx context.Context, blockNumber BlockNumber, opts interface{},
) (response StructuredResponse, err error) {

	// Process arguments based on version
	var blockArgs *rpc_common.BlockArgs
	blockArgs, ok := opts.(*rpc_common.BlockArgs)
	if !ok {
		blockArgs, err = s.getBlockOptions(opts)
		if err != nil {
			return nil, err
		}
	}
	blockArgs.InclTx = true

	blockNum := blockNumber.EthBlockNumber()
	if blockNum != rpc.LatestBlockNumber && blockNum != rpc.PendingBlockNumber {
		cacheKey := combineCacheKey(uint64(blockNum), s.version, blockArgs)
		if block, ok := s.blockCache.Get(cacheKey); ok {
			return block.(StructuredResponse), nil
		}
	}

	err = s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Some Ethereum tools (such as Truffle) rely on being able to query for future blocks without the chain returning errors.
	// These tools implement retry mechanisms that will query & retry for a given block until it has been finalized.
	// Throwing an error like "requested block number greater than current block number" breaks this retry functionality.
	// Disable isBlockGreaterThanLatest checks for Ethereum RPC:s, but keep them in place for legacy hmy_ RPC:s for now to ensure backwards compatibility
	if s.version != Eth && isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}

	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if blk != nil && err == nil {
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, blockNumber)
			if err != nil {
				return nil, err
			}
		}

		// Format the response according to version
		leader := s.hmy.GetLeaderAddress(blk.Header().Coinbase(), blk.Header().Epoch())
		var rpcBlock interface{}
		switch s.version {
		case V1:
			rpcBlock, err = v1.NewBlock(blk, blockArgs, leader)
		case V2:
			rpcBlock, err = v2.NewBlock(blk, blockArgs, leader)
		case Eth:
			rpcBlock, err = eth.NewBlock(blk, blockArgs, leader)
		default:
			return nil, ErrUnknownRPCVersion
		}
		if err != nil {
			return nil, err
		}

		response, err = NewStructuredResponse(rpcBlock)
		if err != nil {
			return nil, err
		}

		// Pending blocks need to nil out a few fields
		if blockNum == rpc.PendingBlockNumber {
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}

		if blockNum != rpc.PendingBlockNumber {
			cacheKey := combineCacheKey(blk.NumberU64(), s.version, blockArgs)
			s.blockCache.Add(cacheKey, response)
		}
		return response, err
	}

	return nil, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByHash(
	ctx context.Context, blockHash common.Hash, opts interface{},
) (response StructuredResponse, err error) {

	err = s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Process arguments based on version
	var blockArgs *rpc_common.BlockArgs
	blockArgs, ok := opts.(*rpc_common.BlockArgs)
	if !ok {
		blockArgs, err = s.getBlockOptions(opts)
		if err != nil {
			return nil, err
		}
	}
	blockArgs.InclTx = true

	// Fetch the block
	blk, err := s.hmy.GetBlock(ctx, blockHash)
	if blk != nil && err == nil {
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, BlockNumber(blk.NumberU64()))
			if err != nil {
				return nil, err
			}
		}

		// Format the response according to version
		leader := s.hmy.GetLeaderAddress(blk.Header().Coinbase(), blk.Header().Epoch())
		var rpcBlock interface{}
		switch s.version {
		case V1:
			rpcBlock, err = v1.NewBlock(blk, blockArgs, leader)
		case V2:
			rpcBlock, err = v2.NewBlock(blk, blockArgs, leader)
		case Eth:
			rpcBlock, err = eth.NewBlock(blk, blockArgs, leader)
		default:
			return nil, ErrUnknownRPCVersion
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(rpcBlock)
	}

	return nil, err
}

// GetBlockByNumberNew is an alias for GetBlockByNumber using rpc_common.BlockArgs
func (s *PublicBlockchainService) GetBlockByNumberNew(
	ctx context.Context, blockNum BlockNumber, blockArgs *rpc_common.BlockArgs,
) (StructuredResponse, error) {
	return s.GetBlockByNumber(ctx, blockNum, blockArgs)
}

// GetBlockByHashNew is an alias for GetBlocksByHash using rpc_common.BlockArgs
func (s *PublicBlockchainService) GetBlockByHashNew(
	ctx context.Context, blockHash common.Hash, blockArgs *rpc_common.BlockArgs,
) (StructuredResponse, error) {
	return s.GetBlockByHash(ctx, blockHash, blockArgs)
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockchainService) GetBlocks(
	ctx context.Context, blockNumberStart BlockNumber,
	blockNumberEnd BlockNumber, blockArgs *rpc_common.BlockArgs,
) ([]StructuredResponse, error) {

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	blockStart := blockNumberStart.Int64()
	blockEnd := blockNumberEnd.Int64()

	// Fetch blocks within given range
	result := []StructuredResponse{}
	for i := blockStart; i <= blockEnd; i++ {
		blockNum := BlockNumber(i)
		if blockNum.Int64() > s.hmy.CurrentBlock().Number().Int64() {
			break
		}
		// rpcBlock is already formatted according to version
		rpcBlock, err := s.GetBlockByNumber(ctx, blockNum, blockArgs)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("RPC Get Blocks Error")
		}
		if rpcBlock != nil {
			result = append(result, rpcBlock)
		}
	}
	return result, nil
}

// IsLastBlock checks if block is last epoch block.
func (s *PublicBlockchainService) IsLastBlock(ctx context.Context, blockNum uint64) (bool, error) {
	if !isBeaconShard(s.hmy) {
		return false, ErrNotBeaconShard
	}
	return shard.Schedule.IsLastBlock(blockNum), nil
}

// EpochLastBlock returns epoch last block.
func (s *PublicBlockchainService) EpochLastBlock(ctx context.Context, epoch uint64) (uint64, error) {
	if !isBeaconShard(s.hmy) {
		return 0, ErrNotBeaconShard
	}
	return shard.Schedule.EpochLastBlock(epoch), nil
}

// GetBlockSigners returns signers for a particular block.
func (s *PublicBlockchainService) GetBlockSigners(
	ctx context.Context, blockNumber BlockNumber,
) ([]string, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure correct block
	if blockNum.Int64() == 0 || blockNum.Int64() >= s.hmy.CurrentBlock().Number().Int64() {
		return []string{}, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}

	// Fetch signers
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	signers := []string{}
	for _, validator := range slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		if ok, err := mask.KeyEnabled(validator.BLSPublicKey); err == nil && ok {
			signers = append(signers, oneAddress)
		}
	}
	return signers, nil
}

// GetBlockSignerKeys returns bls public keys that signed the block.
func (s *PublicBlockchainService) GetBlockSignerKeys(
	ctx context.Context, blockNumber BlockNumber,
) ([]string, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure correct block
	if blockNum.Int64() == 0 || blockNum.Int64() >= s.hmy.CurrentBlock().Number().Int64() {
		return []string{}, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}

	// Fetch signers
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	signers := []string{}
	for _, validator := range slots {
		if ok, err := mask.KeyEnabled(validator.BLSPublicKey); err == nil && ok {
			signers = append(signers, validator.BLSPublicKey.Hex())
		}
	}
	return signers, nil
}

// IsBlockSigner returns true if validator with address signed blockNum block.
func (s *PublicBlockchainService) IsBlockSigner(
	ctx context.Context, blockNumber BlockNumber, address string,
) (bool, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure correct block
	if blockNum.Int64() == 0 {
		return false, nil
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return false, ErrRequestedBlockTooHigh
	}

	// Fetch signers
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return false, err
	}

	// Check if given address is in slots (response output is the same for all versions)
	for _, validator := range slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return false, err
		}
		if oneAddress != address {
			continue
		}
		if ok, err := mask.KeyEnabled(validator.BLSPublicKey); err == nil && ok {
			return true, nil
		}
	}
	return false, nil
}

// GetSignedBlocks returns how many blocks a particular validator signed for
// last blocksPeriod (1 epoch's worth of blocks).
func (s *PublicBlockchainService) GetSignedBlocks(
	ctx context.Context, address string,
) (interface{}, error) {
	// Fetch the number of signed blocks within default period
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
	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return "", err
	}

	// Response output is the same for all versions
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return leader, nil
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockchainService) GetShardingStructure(
	ctx context.Context,
) ([]StructuredResponse, error) {
	err := s.wait(ctx)
	if err != nil {
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

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch balance
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}
	balance, err := s.getBalanceByBlockNumber(ctx, address, blockNum)
	if err != nil {
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

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
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
	return NewStructuredResponse(s.hmy.GetLatestChainHeaders())
}

// GetLastCrossLinks ..
func (s *PublicBlockchainService) GetLastCrossLinks(
	ctx context.Context,
) ([]StructuredResponse, error) {

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Fetch crosslinks
	crossLinks, err := s.hmy.GetLastCrossLinks()
	if err != nil {
		return nil, err
	}

	// Format response, all output is the same for all versions
	responseSlice := []StructuredResponse{}
	for _, el := range crossLinks {
		response, err := NewStructuredResponse(el)
		if err != nil {
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
	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure valid block number
	if s.version != Eth && isBlockGreaterThanLatest(s.hmy, blockNum) {
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

// GetCurrentUtilityMetrics ..
func (s *PublicBlockchainService) GetCurrentUtilityMetrics(
	ctx context.Context,
) (StructuredResponse, error) {
	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Fetch metrics
	metrics, err := s.hmy.GetCurrentUtilityMetrics()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(metrics)
}

// GetSuperCommittees ..
func (s *PublicBlockchainService) GetSuperCommittees(
	ctx context.Context,
) (StructuredResponse, error) {
	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Fetch super committees
	cmt, err := s.hmy.GetSuperCommittees()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(cmt)
}

// GetCurrentBadBlocks ..
func (s *PublicBlockchainService) GetCurrentBadBlocks(
	ctx context.Context,
) ([]StructuredResponse, error) {

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch bad blocks and format
	badBlocks := []StructuredResponse{}
	for _, blk := range s.hmy.GetCurrentBadBlocks() {
		// Response output is the same for all versions
		fmtBadBlock, err := NewStructuredResponse(blk)
		if err != nil {
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
	return stakingReward.GetTotalTokens(s.hmy.BlockChain)
}

// GetCirculatingSupply ...
func (s *PublicBlockchainService) GetCirculatingSupply(
	ctx context.Context,
) (numeric.Dec, error) {
	return chain.GetCirculatingSupply(ctx, s.hmy.BlockChain)
}

// GetStakingNetworkInfo ..
func (s *PublicBlockchainService) GetStakingNetworkInfo(
	ctx context.Context,
) (StructuredResponse, error) {

	err := s.wait(ctx)
	if err != nil {
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}
	totalStaking := s.hmy.GetTotalStakingSnapshot()
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	medianSnapshot, err := s.hmy.GetMedianRawStakeSnapshot()
	if err != nil {
		return nil, err
	}
	epochLastBlock, err := s.EpochLastBlock(ctx, header.Epoch().Uint64())
	if err != nil {
		return nil, err
	}
	totalSupply, err := s.GetTotalSupply(ctx)
	if err != nil {
		return nil, err
	}
	circulatingSupply, err := s.GetCirculatingSupply(ctx)
	if err != nil {
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

// InSync returns if shard chain is syncing
func (s *PublicBlockchainService) InSync(ctx context.Context) (bool, error) {
	inSync, _ := s.hmy.NodeAPI.SyncStatus(s.hmy.BlockChain.ShardID())
	return inSync, nil
}

// BeaconInSync returns if beacon chain is syncing
func (s *PublicBlockchainService) BeaconInSync(ctx context.Context) (bool, error) {
	inSync, _ := s.hmy.NodeAPI.SyncStatus(s.hmy.BeaconChain.ShardID())
	return inSync, nil
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

func combineCacheKey(number uint64, version Version, blockArgs *rpc_common.BlockArgs) string {
	// no need format blockArgs.Signers[] as a part of cache key
	// because it's not input from rpc caller, it's caculate with blockArgs.WithSigners
	return strconv.FormatUint(number, 10) + strconv.FormatInt(int64(version), 10) + strconv.FormatBool(blockArgs.WithSigners) + strconv.FormatBool(blockArgs.InclTx) + strconv.FormatBool(blockArgs.FullTx) + strconv.FormatBool(blockArgs.InclStaking)
}
