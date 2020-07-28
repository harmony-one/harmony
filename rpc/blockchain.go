package rpc

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	"github.com/harmony-one/harmony/rpc/v1"
	"github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

const (
	defaultGasPrice     = denominations.Nano
	defaultFromAddress  = "0x0000000000000000000000000000000000000000"
	defaultBlocksPeriod = 15000
	validatorsPageSize  = 100
	initSupply          = int64(12600000000)
)

// PublicBlockchainService provides an API to access the Harmony blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockchainService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicBlockChainAPI creates a new API for the RPC interface
func NewPublicBlockChainAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicBlockchainService{hmy, version},
		Public:    true,
	}
}

func (s *PublicBlockchainService) isBeaconShard() error {
	if s.hmy.ShardID != shard.BeaconChainShardID {
		return ErrNotBeaconShard
	}
	return nil
}

func (s *PublicBlockchainService) isBlockGreaterThanLatest(blockNum rpc.BlockNumber) error {
	// rpc.BlockNumber is int64 (latest = -1. pending = -2) and currentBlockNum is uint64.
	// Most straightfoward to make sure to return nil error for latest and pending block num
	// since they are never greater than latest
	if blockNum != rpc.PendingBlockNumber &&
		blockNum != rpc.LatestBlockNumber &&
		uint64(blockNum) > s.hmy.CurrentBlock().NumberU64() {
		return ErrRequestedBlockTooHigh
	}
	return nil
}

// getBlockOptions is a helper to get block args given an interface option from RPC params.
func (s *PublicBlockchainService) getBlockOptions(opts interface{}) (*rpc_common.BlockArgs, error) {
	switch s.version {
	case V1:
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
		return nil, ErrUnknownRpcVersion
	}
}

// GetBlockByNumber returns the requested block. When blockNum is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
// When withSigners in BlocksArgs is true it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByNumber(
	ctx context.Context, blockNumber BlockNumber, opts interface{},
) (response rpc_common.StructuredResponse, err error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
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
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}
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
		rpcBlock, err = v1.NewRPCBlock(blk, blockArgs, leader)
	case V2:
		rpcBlock, err = v2.NewRPCBlock(blk, blockArgs, leader)
	default:
		return nil, ErrUnknownRpcVersion
	}
	if err != nil {
		return nil, err
	}
	response, err = rpc_common.NewStructuredResponse(rpcBlock)
	if err != nil {
		return nil, err
	}

	// Pending blocks need to nil out a few fields
	if blockNum == rpc.PendingBlockNumber {
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}
	return response, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByHash(
	ctx context.Context, blockHash common.Hash, opts interface{},
) (response rpc_common.StructuredResponse, err error) {
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
	if err != nil {
		return nil, err
	}
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
		rpcBlock, err = v1.NewRPCBlock(blk, blockArgs, leader)
	case V2:
		rpcBlock, err = v2.NewRPCBlock(blk, blockArgs, leader)
	default:
		return nil, ErrUnknownRpcVersion
	}
	if err != nil {
		return nil, err
	}
	return rpc_common.NewStructuredResponse(rpcBlock)
}

// GetBlockByNumberNew is an alias for GetBlockByNumber using BlockArgs
func (s *PublicBlockchainService) GetBlockByNumberNew(
	ctx context.Context, blockNum BlockNumber, blockArgs *rpc_common.BlockArgs,
) (rpc_common.StructuredResponse, error) {
	return s.GetBlockByNumber(ctx, blockNum, blockArgs)
}

// GetBlockByHashNew is an alias for GetBlocksByHash using BlockArgs
func (s *PublicBlockchainService) GetBlockByHashNew(
	ctx context.Context, blockHash common.Hash, blockArgs *rpc_common.BlockArgs,
) (rpc_common.StructuredResponse, error) {
	return s.GetBlockByHash(ctx, blockHash, blockArgs)
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockchainService) GetBlocks(
	ctx context.Context, blockNumberStart BlockNumber,
	blockNumberEnd BlockNumber, blockArgs *rpc_common.BlockArgs,
) ([]rpc_common.StructuredResponse, error) {
	blockStart := blockNumberStart.Int64()
	blockEnd := blockNumberEnd.Int64()

	// Fetch blocks within given range
	result := []rpc_common.StructuredResponse{}
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

// GetValidators returns validators list for a particular epoch.
func (s *PublicBlockchainService) GetValidators(
	ctx context.Context, epoch int64,
) (rpc_common.StructuredResponse, error) {
	// Fetch the Committee
	cmt, err := s.hmy.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}
	balanceQueryBlock := shard.Schedule.EpochLastBlock(uint64(epoch))
	if balanceQueryBlock > s.hmy.CurrentBlock().NumberU64() {
		balanceQueryBlock = s.hmy.CurrentBlock().NumberU64()
	}

	validators := []rpc_common.StructuredResponse{}
	for _, validator := range cmt.Slots {
		// Fetch the balance of the validator
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		validatorBalance, err := s.getBalanceByBlockNumber(ctx, oneAddress, rpc.BlockNumber(balanceQueryBlock))
		if err != nil {
			return nil, err
		}

		// Format the response according to the version
		var validatorsFields rpc_common.StructuredResponse
		switch s.version {
		case V1:
			validatorsFields = rpc_common.StructuredResponse{
				"address": oneAddress,
				"balance": (*hexutil.Big)(validatorBalance),
			}
		case V2:
			validatorsFields = rpc_common.StructuredResponse{
				"address": oneAddress,
				"balance": validatorBalance,
			}
		default:
			return nil, ErrUnknownRpcVersion
		}
		validators = append(validators, validatorsFields)
	}
	result := rpc_common.StructuredResponse{
		"shardID":    cmt.ShardID,
		"validators": validators,
	}
	return result, nil
}

// GetValidatorKeys returns list of bls public keys in the committee for a particular epoch.
func (s *PublicBlockchainService) GetValidatorKeys(
	ctx context.Context, epoch int64,
) ([]string, error) {
	// Fetch the Committee
	cmt, err := s.hmy.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	validators := make([]string, len(cmt.Slots))
	for i, v := range cmt.Slots {
		validators[i] = v.BLSPublicKey.Hex()
	}
	return validators, nil
}

// IsLastBlock checks if block is last epoch block.
func (s *PublicBlockchainService) IsLastBlock(ctx context.Context, blockNum uint64) (bool, error) {
	if err := s.isBeaconShard(); err != nil {
		return false, err
	}
	return shard.Schedule.IsLastBlock(blockNum), nil
}

// EpochLastBlock returns epoch last block.
func (s *PublicBlockchainService) EpochLastBlock(ctx context.Context, epoch uint64) (uint64, error) {
	if err := s.isBeaconShard(); err != nil {
		return 0, err
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
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
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
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
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
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return false, err
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

// GetSignedBlocks returns how many blocks a particular validator signed for last defaultBlocksPeriod (3 hours ~ 1500 blocks).
func (s *PublicBlockchainService) GetSignedBlocks(
	ctx context.Context, address string,
) (interface{}, error) {
	// Fetch the number of signed blocks within default period
	totalSigned := uint64(0)
	lastBlock := uint64(0)
	blockHeight := s.hmy.CurrentBlock().Number().Uint64()
	if blockHeight >= defaultBlocksPeriod {
		lastBlock = blockHeight - defaultBlocksPeriod + 1
	}
	for i := lastBlock; i <= blockHeight; i++ {
		signed, err := s.IsBlockSigner(ctx, BlockNumber(i), address)
		if err == nil && signed {
			totalSigned++
		}
	}

	// Format the response according to the version
	switch s.version {
	case V1:
		return hexutil.Uint64(totalSigned), nil
	case V2:
		return totalSigned, nil
	default:
		return nil, ErrUnknownRpcVersion
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
	case V1:
		return hexutil.Uint64(epoch), nil
	case V2:
		return epoch, nil
	default:
		return nil, ErrUnknownRpcVersion
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

// GetValidatorSelfDelegation returns validator stake.
func (s *PublicBlockchainService) GetValidatorSelfDelegation(
	ctx context.Context, address string,
) (interface{}, error) {
	// Ensure node is for beacon shard
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch self delegation
	selfDelegation := s.hmy.GetValidatorSelfDelegation(internal_common.ParseAddr(address)).Uint64()

	// Format the response according to the version
	switch s.version {
	case V1:
		return hexutil.Uint64(selfDelegation), nil
	case V2:
		return selfDelegation, nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetValidatorTotalDelegation returns total balance stacking for validator with delegation.
func (s *PublicBlockchainService) GetValidatorTotalDelegation(
	ctx context.Context, address string,
) (interface{}, error) {
	// Ensure node is for beacon shard
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch delegations & sum
	delegations := s.hmy.GetDelegationsByValidator(internal_common.ParseAddr(address))
	totalStake := big.NewInt(0)
	for _, delegation := range delegations {
		totalStake.Add(totalStake, delegation.Amount)
	}

	// Format the response according to the version
	switch s.version {
	case V1:
		return hexutil.Uint64(totalStake.Uint64()), nil
	case V2:
		return totalStake, nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockchainService) GetShardingStructure(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
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

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockchainService) GetCode(
	ctx context.Context, addr string, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch state
	address := internal_common.ParseAddr(addr)
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)

	// Response output is the same for all versions
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockchainService) GetStorageAt(
	ctx context.Context, addr string, key string, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch state
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return nil, err
	}
	address := internal_common.ParseAddr(addr)
	res := state.GetState(address, common.HexToHash(key))

	// Response output is the same for all versions
	return res[:], state.Error()
}

// getBalanceByBlockNumber returns balance by block number at given eth blockNum without checks
func (s *PublicBlockchainService) getBalanceByBlockNumber(
	ctx context.Context, address string, blockNum rpc.BlockNumber,
) (*big.Int, error) {
	addr := internal_common.ParseAddr(address)
	balance, err := s.hmy.GetBalance(ctx, addr, blockNum)
	if err != nil {
		return nil, err
	}
	return balance, nil
}

// GetBalanceByBlockNumber returns balance by block number
func (s *PublicBlockchainService) GetBalanceByBlockNumber(
	ctx context.Context, address string, blockNumber BlockNumber,
) (interface{}, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch balance
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	balance, err := s.getBalanceByBlockNumber(ctx, address, blockNum)
	if err != nil {
		return nil, err
	}

	// Format return base on version
	switch s.version {
	case V1:
		return (*hexutil.Big)(balance), nil
	case V2:
		return balance, nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (s *PublicBlockchainService) GetAccountNonce(
	ctx context.Context, address string, blockNumber BlockNumber,
) (uint64, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Response output is the same for all versions
	addr := internal_common.ParseAddr(address)
	return s.hmy.GetAccountNonce(ctx, addr, blockNum)
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
	case V1:
		return hexutil.Uint64(header.Number().Uint64()), nil
	case V2:
		return header.Number().Uint64(), nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// ResendCx requests that the egress receipt for the given cross-shard
// transaction be sent to the destination shard for credit.  This is used for
// unblocking a half-complete cross-shard transaction whose fund has been
// withdrawn already from the source shard but not credited yet in the
// destination account due to transient failures.
func (s *PublicBlockchainService) ResendCx(ctx context.Context, txID common.Hash) (bool, error) {
	_, success := s.hmy.ResendCx(ctx, txID)

	// Response output is the same for all versions
	return success, nil
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockchainService) Call(
	ctx context.Context, args CallArgs, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Execute call
	result, _, _, err := doCall(ctx, s.hmy, args, blockNum, vm.Config{}, CallTimeout, s.hmy.RPCGasCap)

	// Response output is the same for all versions
	return result, err
}

// LatestHeader returns the latest header information
func (s *PublicBlockchainService) LatestHeader(ctx context.Context) (rpc_common.StructuredResponse, error) {
	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return rpc_common.NewStructuredResponse(NewHeaderInformation(header, leader))
}

// GetLatestChainHeaders ..
func (s *PublicBlockchainService) GetLatestChainHeaders(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(s.hmy.GetLatestChainHeaders())
}

// GetLastCrossLinks ..
func (s *PublicBlockchainService) GetLastCrossLinks(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch crosslinks
	crossLinks, err := s.hmy.GetLastCrossLinks()
	if err != nil {
		return nil, err
	}

	// Format response, all output is the same for all versions
	responseSlice := []rpc_common.StructuredResponse{}
	for _, el := range crossLinks {
		response, err := rpc_common.NewStructuredResponse(el)
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
) (rpc_common.StructuredResponse, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Ensure valid block number
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}

	// Fetch Header
	header, err := s.hmy.HeaderByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return rpc_common.NewStructuredResponse(NewHeaderInformation(header, leader))
}

// GetTotalStaking returns total staking by validators, only meant to be called on beaconchain
// explorer node
func (s *PublicBlockchainService) GetTotalStaking(
	ctx context.Context,
) (*big.Int, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return s.hmy.GetTotalStakingSnapshot(), nil
}

// GetMedianRawStakeSnapshot returns the raw median stake, only meant to be called on beaconchain
// explorer node
func (s *PublicBlockchainService) GetMedianRawStakeSnapshot(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch snapshot
	snapshot, err := s.hmy.GetMedianRawStakeSnapshot()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(snapshot)
}

// GetAllValidatorAddresses returns all validator addresses.
func (s *PublicBlockchainService) GetAllValidatorAddresses(
	ctx context.Context,
) ([]string, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch all validator addresses
	validatorAddresses := s.hmy.GetAllValidatorAddresses()
	addresses := make([]string, len(validatorAddresses))
	for i, addr := range validatorAddresses {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		// Response output is the same for all versions
		addresses[i] = oneAddr
	}
	return addresses, nil
}

// GetElectedValidatorAddresses returns elected validator addresses.
func (s *PublicBlockchainService) GetElectedValidatorAddresses(
	ctx context.Context,
) ([]string, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch elected validators
	electedAddresses := s.hmy.GetElectedValidatorAddresses()
	addresses := make([]string, len(electedAddresses))
	for i, addr := range electedAddresses {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		// Response output is the same for all versions
		addresses[i] = oneAddr
	}
	return addresses, nil
}

// GetValidatorInformation returns information about a validator.
func (s *PublicBlockchainService) GetValidatorInformation(
	ctx context.Context, address string,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch latest block
	blk, err := s.hmy.BlockByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the latest blk information")
	}

	// Fetch validator information
	validatorInfo, err := s.hmy.GetValidatorInformation(internal_common.ParseAddr(address), blk)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(validatorInfo)
}

// GetValidatorInformationByBlockNumber returns information about a validator.
func (s *PublicBlockchainService) GetValidatorInformationByBlockNumber(
	ctx context.Context, address string, blockNumber BlockNumber,
) (rpc_common.StructuredResponse, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}

	// Fetch validator info
	validatorInfo, err := s.hmy.GetValidatorInformation(internal_common.ParseAddr(address), blk)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(validatorInfo)
}

// getAllValidatorInformation helper function to get all validator information for a given eth block number
func (s *PublicBlockchainService) getAllValidatorInformation(
	ctx context.Context, page int, blockNum rpc.BlockNumber,
) ([]rpc_common.StructuredResponse, error) {
	if page < -1 {
		return nil, errors.Errorf("page given %d cannot be less than -1", page)
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return []rpc_common.StructuredResponse{}, nil
	}

	// Set page start
	validatorsNum := len(addresses)
	start := 0
	if page != -1 {
		validatorsNum = validatorsPageSize
		start = page * validatorsPageSize
		if len(addresses)-start < validatorsPageSize {
			validatorsNum = len(addresses) - start
		}
	}

	// Fetch block
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}

	// Fetch validator information for block
	validators := []rpc_common.StructuredResponse{}
	for i := start; i < start+validatorsNum; i++ {
		validatorInfo, err := s.hmy.GetValidatorInformation(addresses[i], blk)
		if err == nil {
			// Response output is the same for all versions
			information, err := rpc_common.NewStructuredResponse(validatorInfo)
			if err != nil {
				return nil, err
			}
			validators = append(validators, information)
		}
	}
	return validators, nil
}

// GetAllValidatorInformation returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicBlockchainService) GetAllValidatorInformation(
	ctx context.Context, page int,
) ([]rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// fetch current block number
	blockNum := s.hmy.CurrentBlock().NumberU64()

	// delete cache for previous block
	prevKey := fmt.Sprintf("all-info-%d", blockNum-1)
	s.hmy.SingleFlightForgetKey(prevKey)

	// Fetch all validator information in a single flight request
	key := fmt.Sprintf("all-info-%d", blockNum)
	res, err := s.hmy.SingleFlightRequest(
		key,
		func() (interface{}, error) {
			return s.getAllValidatorInformation(ctx, page, rpc.LatestBlockNumber)
		},
	)
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return res.([]rpc_common.StructuredResponse), nil
}

// GetAllValidatorInformationByBlockNumber returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicBlockchainService) GetAllValidatorInformationByBlockNumber(
	ctx context.Context, page int, blockNumber BlockNumber,
) ([]rpc_common.StructuredResponse, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return s.getAllValidatorInformation(ctx, page, blockNum)
}

// GetAllDelegationInformation returns delegation information about `validatorsPageSize` validators,
// starting at `page*validatorsPageSize`.
// If page is -1, return all instead of `validatorsPageSize` elements.
// TODO(dm): optimize with single flight
func (s *PublicBlockchainService) GetAllDelegationInformation(
	ctx context.Context, page int,
) ([][]rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if page < -1 {
		return make([][]rpc_common.StructuredResponse, 0), nil
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()

	// Return nothing if no delegation on page
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return make([][]rpc_common.StructuredResponse, 0), nil
	}

	// Set page start
	validatorsNum := len(addresses)
	start := 0
	if page != -1 {
		validatorsNum = validatorsPageSize
		start = page * validatorsPageSize
		if len(addresses)-start < validatorsPageSize {
			validatorsNum = len(addresses) - start
		}
	}

	// Fetch all delegations
	validators := make([][]rpc_common.StructuredResponse, validatorsNum)
	var err error
	for i := start; i < start+validatorsNum; i++ {
		validators[i-start], err = s.GetDelegationsByValidator(ctx, addresses[i].String())
		if err != nil {
			return nil, err
		}
	}

	// Response output is the same for all versions
	return validators, nil
}

// GetDelegationsByDelegator returns list of delegations for a delegator address.
func (s *PublicBlockchainService) GetDelegationsByDelegator(
	ctx context.Context, address string,
) ([]rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch delegation
	delegatorAddress := internal_common.ParseAddr(address)
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)

	// Format response
	result := []rpc_common.StructuredResponse{}
	for i := range delegations {
		delegation := delegations[i]
		undelegations := make([]Undelegation, len(delegation.Undelegations))

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, Undelegation{
				Amount: delegation.Undelegations[j].Amount,
				Epoch:  delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validators[i])
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)

		// Response output is the same for all versions
		del, err := rpc_common.NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, del)
	}
	return result, nil
}

// GetDelegationsByDelegatorByBlockNumber returns list of delegations for a delegator address at given block number
func (s *PublicBlockchainService) GetDelegationsByDelegatorByBlockNumber(
	ctx context.Context, address string, blockNumber BlockNumber,
) ([]rpc_common.StructuredResponse, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}

	// Fetch delegation for block number
	delegatorAddress := internal_common.ParseAddr(address)
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}
	validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(delegatorAddress, blk)

	// Format response
	result := []rpc_common.StructuredResponse{}
	for i := range delegations {
		delegation := delegations[i]
		undelegations := make([]Undelegation, len(delegation.Undelegations))

		for j := range delegation.Undelegations {
			undelegations[j] = Undelegation{
				Amount: delegation.Undelegations[j].Amount,
				Epoch:  delegation.Undelegations[j].Epoch,
			}
		}
		valAddr, _ := internal_common.AddressToBech32(validators[i])
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)

		// Response output is the same for all versions
		del, err := rpc_common.NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, del)
	}
	return result, nil
}

// GetDelegationsByValidator returns list of delegations for a validator address.
func (s *PublicBlockchainService) GetDelegationsByValidator(
	ctx context.Context, address string,
) ([]rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch delegations
	validatorAddress := internal_common.ParseAddr(address)
	delegations := s.hmy.GetDelegationsByValidator(validatorAddress)

	// Format response
	result := []rpc_common.StructuredResponse{}
	for i := range delegations {
		delegation := delegations[i]
		undelegations := []Undelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, Undelegation{
				Amount: delegation.Undelegations[j].Amount,
				Epoch:  delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegation.DelegatorAddress)

		// Response output is the same for all versions
		del, err := rpc_common.NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, del)
	}
	return result, nil
}

// GetDelegationByDelegatorAndValidator returns a delegation for delegator and validator.
func (s *PublicBlockchainService) GetDelegationByDelegatorAndValidator(
	ctx context.Context, address string, validator string,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch delegations
	delegatorAddress := internal_common.ParseAddr(address)
	validatorAddress := internal_common.ParseAddr(validator)
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)

	// Format response
	for i := range delegations {
		if validators[i] != validatorAddress {
			continue
		}
		delegation := delegations[i]

		undelegations := []Undelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, Undelegation{
				Amount: delegation.Undelegations[j].Amount,
				Epoch:  delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)

		// Response output is the same for all versions
		return rpc_common.NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
	}
	return nil, nil
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockchainService) EstimateGas(
	ctx context.Context, args CallArgs,
) (hexutil.Uint64, error) {
	gas, err := doEstimateGas(ctx, s.hmy, args, nil)
	if err != nil {
		return 0, err
	}

	// Response output is the same for all versions
	return (hexutil.Uint64)(gas), nil
}

// GetCurrentUtilityMetrics ..
func (s *PublicBlockchainService) GetCurrentUtilityMetrics(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch metrics
	metrics, err := s.hmy.GetCurrentUtilityMetrics()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(metrics)
}

// GetSuperCommittees ..
func (s *PublicBlockchainService) GetSuperCommittees(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}

	// Fetch super committees
	cmt, err := s.hmy.GetSuperCommittees()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return rpc_common.NewStructuredResponse(cmt)
}

// GetCurrentBadBlocks ..
func (s *PublicBlockchainService) GetCurrentBadBlocks(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// Fetch bad blocks and format
	badBlocks := []rpc_common.StructuredResponse{}
	for _, blk := range s.hmy.GetCurrentBadBlocks() {
		// Response output is the same for all versions
		fmtBadBlock, err := rpc_common.NewStructuredResponse(blk)
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
	// Response output is the same for all versions
	return numeric.NewDec(initSupply), nil
}

// GetCirculatingSupply ..
func (s *PublicBlockchainService) GetCirculatingSupply(
	ctx context.Context,
) (numeric.Dec, error) {
	timestamp := time.Now()

	// Response output is the same for all versions
	return numeric.NewDec(initSupply).Mul(reward.PercentageForTimeStamp(timestamp.Unix())), nil
}

// GetStakingNetworkInfo ..
func (s *PublicBlockchainService) GetStakingNetworkInfo(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	header, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	totalStaking, err := s.GetTotalStaking(ctx)
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
	return rpc_common.NewStructuredResponse(StakingNetworkInfo{
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		EpochLastBlock:    epochLastBlock,
		TotalStaking:      totalStaking,
		MedianRawStake:    medianSnapshot.MedianStake,
	})
}

// docall executes an EVM call
func doCall(
	ctx context.Context, hmy *hmy.Harmony, args CallArgs, blockNum rpc.BlockNumber,
	vmCfg vm.Config, timeout time.Duration, globalGasCap *big.Int,
) ([]byte, uint64, bool, error) {
	defer func(start time.Time) {
		utils.Logger().Debug().
			Dur("runtime", time.Since(start)).
			Msg("Executing EVM call finished")
	}(time.Now())

	// Fetch state
	state, header, err := hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return nil, 0, false, err
	}

	// Set sender address or use a default if none specified
	var addr common.Address
	if args.From == nil {
		// Any address does not affect the logic of this call.
		addr = common.HexToAddress(defaultFromAddress)
	} else {
		addr = *args.From
	}

	// Set default gas & gas price if none were set
	gas := uint64(math.MaxUint64 / 2)
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != nil && globalGasCap.Uint64() < gas {
		utils.Logger().Warn().
			Uint64("requested", gas).
			Uint64("cap", globalGasCap.Uint64()).
			Msg("Caller gas above allowance, capping")
		gas = globalGasCap.Uint64()
	}
	gasPrice := new(big.Int).SetUint64(defaultGasPrice)
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	}

	// Set value & data
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	var data []byte
	if args.Data != nil {
		data = *args.Data
	}

	// Create new call message
	msg := types.NewMessage(addr, args.To, 0, value, gas, gasPrice, data, false)

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, vmError, err := hmy.GetEVM(ctx, msg, state, header)
	if err != nil {
		return nil, 0, false, err
	}

	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(core.GasPool).AddGas(math.MaxUint64)
	res, gas, failed, err := core.ApplyMessage(evm, msg, gp)
	if err := vmError(); err != nil {
		return nil, 0, false, err
	}

	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		return nil, 0, false, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}

	// Response output is the same for all versions
	return res, gas, failed, err
}

// doEstimateGas ..
func doEstimateGas(
	ctx context.Context, hmy *hmy.Harmony, args CallArgs, gasCap *big.Int,
) (uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  = params.TxGas - 1
		hi  uint64
		max uint64
	)
	blockNum := rpc.LatestBlockNumber
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Retrieve the blk to act as the gas ceiling
		blk, err := hmy.BlockByNumber(ctx, blockNum)
		if err != nil {
			return 0, err
		}
		hi = blk.GasLimit()
	}
	if gasCap != nil && hi > gasCap.Uint64() {
		hi = gasCap.Uint64()
	}
	max = hi

	// Use zero-address if none other is available
	if args.From == nil {
		args.From = &common.Address{}
	}

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = (*hexutil.Uint64)(&gas)

		_, _, failed, err := doCall(ctx, hmy, args, blockNum, vm.Config{}, 0, big.NewInt(int64(max)))
		if err != nil || failed {
			return false
		}
		return true
	}

	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}

	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == max {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance (%d) or always failing transaction", max)
		}
	}
	return hi, nil
}
