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
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/quorum"
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
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/network"
	staking "github.com/harmony-one/harmony/staking/types"
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

// GetBlockByNumber returns the requested block. When blockNum is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
// When withSigners in BlocksArgs is true it shows block signers for this block in list of one addresses.
func (s *PublicBlockchainService) GetBlockByNumber(
	ctx context.Context, blockNumber BlockNumber, opts interface{},
) (response rpc_common.Response, err error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
	var blockArgs rpc_common.BlockArgs
	switch s.version {
	case V1:
		// Handle GetBlockByNumberNew alias
		testBlockArgs, ok := opts.(rpc_common.BlockArgs)
		if ok {
			blockArgs = testBlockArgs
			break
		}
		fullTx, ok := opts.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid type for second argument")
		}
		blockArgs = rpc_common.BlockArgs{
			WithSigners: false,
			FullTx:      fullTx,
			InclStaking: true,
		}
	case V2:
		parsedBlockArgs := rpc_common.BlockArgs{}
		if err := parsedBlockArgs.UnmarshalFromInterface(opts); err != nil {
			return nil, err
		}
		blockArgs = parsedBlockArgs
	default:
		return nil, ErrUnknownRpcVersion
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
		blockArgs.Signers, err = s.GetBlockSigners(ctx, blockNum)
		if err != nil {
			return nil, err
		}
	}

	// Format the response according to version
	leader := s.hmy.GetLeaderAddress(blk.Header().Coinbase(), blk.Header().Epoch())
	switch s.version {
	case V1:
		response, err = v1.RPCMarshalBlock(blk, blockArgs, leader)
	case V2:
		response, err = v2.RPCMarshalBlock(blk, blockArgs, leader)
	default:
		return nil, ErrUnknownRpcVersion
	}

	// Pending blocks need to nil out a few fields
	if err == nil && blockNum == rpc.PendingBlockNumber {
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
) (response rpc_common.Response, err error) {
	// Process arguments based on version
	var blockArgs rpc_common.BlockArgs
	switch s.version {
	case V1:
		// Handle GetBlockByHashNew alias
		testBlockArgs, ok := opts.(rpc_common.BlockArgs)
		if ok {
			blockArgs = testBlockArgs
			break
		}
		fullTx, ok := opts.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid type for second argument")
		}
		blockArgs = rpc_common.BlockArgs{
			WithSigners: false,
			FullTx:      fullTx,
			InclStaking: true,
		}
	case V2:
		parsedBlockArgs := rpc_common.BlockArgs{}
		if err := parsedBlockArgs.UnmarshalFromInterface(opts); err != nil {
			return nil, err
		}
		blockArgs = parsedBlockArgs
	default:
		return nil, ErrUnknownRpcVersion
	}
	blockArgs.InclTx = true

	// Fetch the block
	blk, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if blockArgs.WithSigners {
		blockArgs.Signers, err = s.GetBlockSigners(ctx, rpc.BlockNumber(blk.NumberU64()))
		if err != nil {
			return nil, err
		}
	}

	// Format the response according to version
	leader := s.hmy.GetLeaderAddress(blk.Header().Coinbase(), blk.Header().Epoch())
	switch s.version {
	case V1:
		return v1.RPCMarshalBlock(blk, blockArgs, leader)
	case V2:
		return v1.RPCMarshalBlock(blk, blockArgs, leader)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetBlockByNumberNew is an alias for GetBlockByNumber using BlockArgs
func (s *PublicBlockchainService) GetBlockByNumberNew(
	ctx context.Context, blockNum BlockNumber, blockArgs rpc_common.BlockArgs,
) (rpc_common.Response, error) {
	return s.GetBlockByNumber(ctx, blockNum, blockArgs)
}

// GetBlockByHashNew is an alias for GetBlocksByHash using BlockArgs
func (s *PublicBlockchainService) GetBlockByHashNew(
	ctx context.Context, blockHash common.Hash, blockArgs rpc_common.BlockArgs,
) (rpc_common.Response, error) {
	return s.GetBlockByHash(ctx, blockHash, blockArgs)
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockchainService) GetBlocks(
	ctx context.Context, blockNumberStart BlockNumber,
	blockNumberEnd BlockNumber, blockArgs rpc_common.BlockArgs,
) ([]rpc_common.Response, error) {
	// Process arguments for both versions
	blockStart := blockNumberStart.Int64()
	blockEnd := blockNumberEnd.Int64()

	// Fetch blocks within given range
	result := []rpc_common.Response{}
	for i := blockStart; i <= blockEnd; i++ {
		blockNum := BlockNumber(i)
		if blockNum.Int64() >= s.hmy.CurrentBlock().Number().Int64() {
			break
		}
		rpcBlock, err := s.GetBlockByNumber(ctx, blockNum, blockArgs)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("RPC Error")
		}
		if rpcBlock != nil {
			result = append(result, rpcBlock)
		}
	}
	return result, nil
}

// GetValidators returns validators list for a particular epoch.
func (s *PublicBlockchainService) GetValidators(ctx context.Context, epoch int64) (map[string]interface{}, error) {
	cmt, err := s.hmy.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}
	balanceQueryBlock := shard.Schedule.EpochLastBlock(uint64(epoch))
	if balanceQueryBlock > s.hmy.CurrentBlock().NumberU64() {
		balanceQueryBlock = s.hmy.CurrentBlock().NumberU64()
	}
	validators := make([]map[string]interface{}, 0)
	for _, validator := range cmt.Slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		validatorBalance, err := s.getBalanceByBlockNumber(ctx, oneAddress, rpc.BlockNumber(balanceQueryBlock))
		if err != nil {
			return nil, err
		}
		validatorsFields := map[string]interface{}{
			"address": oneAddress,
			"balance": (*hexutil.Big)(validatorBalance),
		}
		validators = append(validators, validatorsFields)
	}
	result := map[string]interface{}{
		"shardID":    cmt.ShardID,
		"validators": validators,
	}
	return result, nil
}

// GetValidatorKeys returns list of bls public keys in the committee for a particular epoch.
func (s *PublicBlockchainService) GetValidatorKeys(ctx context.Context, epoch int64) ([]string, error) {
	cmt, err := s.hmy.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}

	validators := make([]string, len(cmt.Slots))
	for i, v := range cmt.Slots {
		validators[i] = v.BLSPublicKey.Hex()
	}
	return validators, nil
}

// IsLastBlock checks if block is last epoch block.
func (s *PublicBlockchainService) IsLastBlock(blockNum uint64) (bool, error) {
	if err := s.isBeaconShard(); err != nil {
		return false, err
	}
	return shard.Schedule.IsLastBlock(blockNum), nil
}

// EpochLastBlock returns epoch last block.
func (s *PublicBlockchainService) EpochLastBlock(epoch uint64) (uint64, error) {
	if err := s.isBeaconShard(); err != nil {
		return 0, err
	}
	return shard.Schedule.EpochLastBlock(epoch), nil
}

// GetBlockSigners returns signers for a particular block.
func (s *PublicBlockchainService) GetBlockSigners(ctx context.Context, blockNum rpc.BlockNumber) ([]string, error) {
	if uint64(blockNum) == 0 {
		return []string{}, nil
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return nil, err
	}
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
func (s *PublicBlockchainService) GetBlockSignerKeys(ctx context.Context, blockNum rpc.BlockNumber) ([]string, error) {
	if uint64(blockNum) == 0 {
		return []string{}, nil
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return nil, err
	}
	signers := []string{}
	for _, validator := range slots {
		if ok, err := mask.KeyEnabled(validator.BLSPublicKey); err == nil && ok {
			signers = append(signers, validator.BLSPublicKey.Hex())
		}
	}
	return signers, nil
}

// IsBlockSigner returns true if validator with address signed blockNum block.
func (s *PublicBlockchainService) IsBlockSigner(ctx context.Context, blockNum rpc.BlockNumber, address string) (bool, error) {
	if uint64(blockNum) == 0 {
		return false, nil
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return false, err
	}
	slots, mask, err := s.hmy.GetBlockSigners(ctx, blockNum)
	if err != nil {
		return false, err
	}
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
func (s *PublicBlockchainService) GetSignedBlocks(ctx context.Context, address string) hexutil.Uint64 {
	totalSigned := uint64(0)
	lastBlock := uint64(0)
	blockHeight := uint64(s.BlockNumber())
	if blockHeight >= defaultBlocksPeriod {
		lastBlock = blockHeight - defaultBlocksPeriod + 1
	}
	for i := lastBlock; i <= blockHeight; i++ {
		signed, err := s.IsBlockSigner(ctx, rpc.BlockNumber(i), address)
		if err == nil && signed {
			totalSigned++
		}
	}
	return hexutil.Uint64(totalSigned)
}

// GetEpoch returns current epoch.
func (s *PublicBlockchainService) GetEpoch(ctx context.Context) hexutil.Uint64 {
	return hexutil.Uint64(s.LatestHeader(ctx).Epoch)
}

// GetLeader returns current shard leader.
func (s *PublicBlockchainService) GetLeader(ctx context.Context) string {
	return s.LatestHeader(ctx).Leader
}

// GetValidatorSelfDelegation returns validator stake.
func (s *PublicBlockchainService) GetValidatorSelfDelegation(ctx context.Context, address string) (hexutil.Uint64, error) {
	if err := s.isBeaconShard(); err != nil {
		return hexutil.Uint64(0), err
	}
	return hexutil.Uint64(s.hmy.GetValidatorSelfDelegation(internal_common.ParseAddr(address)).Uint64()), nil
}

// GetValidatorTotalDelegation returns total balace stacking for validator with delegation.
func (s *PublicBlockchainService) GetValidatorTotalDelegation(ctx context.Context, address string) (hexutil.Uint64, error) {
	if err := s.isBeaconShard(); err != nil {
		return hexutil.Uint64(0), err
	}
	delegations := s.hmy.GetDelegationsByValidator(internal_common.ParseAddr(address))
	totalStake := big.NewInt(0)
	for _, delegation := range delegations {
		totalStake.Add(totalStake, delegation.Amount)
	}
	// TODO: return more than uint64
	return hexutil.Uint64(totalStake.Uint64()), nil
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockchainService) GetShardingStructure(ctx context.Context) ([]map[string]interface{}, error) {
	// Get header and number of shards.
	epoch := s.GetEpoch(ctx)
	numShard := shard.Schedule.InstanceForEpoch(big.NewInt(int64(epoch))).NumShards()

	// Return shareding structure for each case.
	return shard.Schedule.GetShardingStructure(int(numShard), int(s.hmy.ShardID)), nil
}

// GetShardID returns shard ID of the requested node.
func (s *PublicBlockchainService) GetShardID(ctx context.Context) (int, error) {
	return int(s.hmy.ShardID), nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockchainService) GetCode(ctx context.Context, addr string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockchainService) GetStorageAt(ctx context.Context, addr string, key string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

func (s *PublicBlockchainService) getBalanceByBlockNumber(ctx context.Context, address string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	addr := internal_common.ParseAddr(address)
	balance, err := s.hmy.GetBalance(ctx, addr, blockNr)
	if balance == nil {
		return nil, err
	}
	return (*hexutil.Big)(balance), err
}

// GetBalanceByBlockNumber returns balance by block number.
func (s *PublicBlockchainService) GetBalanceByBlockNumber(ctx context.Context, address string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	if err := s.isBlockGreaterThanLatest(blockNr); err != nil {
		return nil, err
	}
	return s.getBalanceByBlockNumber(ctx, address, blockNr)
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (s *PublicBlockchainService) GetAccountNonce(ctx context.Context, address string, blockNr rpc.BlockNumber) (uint64, error) {
	addr := internal_common.ParseAddr(address)
	return s.hmy.GetAccountNonce(ctx, addr, rpc.BlockNumber(blockNr))
}

// GetBalance returns the amount of Atto for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockchainService) GetBalance(ctx context.Context, address string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	return s.getBalanceByBlockNumber(ctx, address, rpc.BlockNumber(blockNr))
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockchainService) BlockNumber() hexutil.Uint64 {
	header, _ := s.hmy.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	if header == nil {
		return 0
	}
	return hexutil.Uint64(header.Number().Uint64())
}

// ResendCx requests that the egress receipt for the given cross-shard
// transaction be sent to the destination shard for credit.  This is used for
// unblocking a half-complete cross-shard transaction whose fund has been
// withdrawn already from the source shard but not credited yet in the
// destination account due to transient failures.
func (s *PublicBlockchainService) ResendCx(ctx context.Context, txID common.Hash) (bool, error) {
	_, success := s.hmy.ResendCx(ctx, txID)
	return success, nil
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockchainService) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, _, err := doCall(ctx, s.hmy, args, blockNr, vm.Config{}, 5*time.Second, s.hmy.RPCGasCap)
	return (hexutil.Bytes)(result), err
}

// LatestHeader returns the latest header information
func (s *PublicBlockchainService) LatestHeader(ctx context.Context) *HeaderInformation {
	header, _ := s.hmy.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return newHeaderInformation(header, leader)
}

// GetHeaderByNumber returns block header at given number
func (s *PublicBlockchainService) GetHeaderByNumber(ctx context.Context, blockNum rpc.BlockNumber) (*HeaderInformation, error) {
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	header, err := s.hmy.HeaderByNumber(context.Background(), blockNum)
	if err != nil {
		return nil, err
	}
	leader := s.hmy.GetLeaderAddress(header.Coinbase(), header.Epoch())
	return newHeaderInformation(header, leader), nil
}

// GetTotalStaking returns total staking by validators, only meant to be called on beaconchain
// explorer node
func (s *PublicBlockchainService) GetTotalStaking() (*big.Int, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	return s.hmy.GetTotalStakingSnapshot(), nil
}

// GetMedianRawStakeSnapshot returns the raw median stake, only meant to be called on beaconchain
// explorer node
func (s *PublicBlockchainService) GetMedianRawStakeSnapshot() (
	*committee.CompletedEPoSRound, error,
) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	return s.hmy.GetMedianRawStakeSnapshot()
}

// GetLatestChainHeaders ..
func (s *PublicBlockchainService) GetLatestChainHeaders() *block.HeaderPair {
	return s.hmy.GetLatestChainHeaders()
}

// GetAllValidatorAddresses returns all validator addresses.
func (s *PublicBlockchainService) GetAllValidatorAddresses() ([]string, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	validatorAddresses := s.hmy.GetAllValidatorAddresses()
	addresses := make([]string, len(validatorAddresses))
	for i, addr := range validatorAddresses {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		addresses[i] = oneAddr
	}
	return addresses, nil
}

// GetElectedValidatorAddresses returns elected validator addresses.
func (s *PublicBlockchainService) GetElectedValidatorAddresses() ([]string, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	electedAddresses := s.hmy.GetElectedValidatorAddresses()
	addresses := make([]string, len(electedAddresses))
	for i, addr := range electedAddresses {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		addresses[i] = oneAddr
	}
	return addresses, nil
}

// GetValidatorInformation returns information about a validator.
func (s *PublicBlockchainService) GetValidatorInformation(
	ctx context.Context, address string,
) (*staking.ValidatorRPCEnhanced, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	blk, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(rpc.LatestBlockNumber))
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the latest blk information")
	}
	return s.hmy.GetValidatorInformation(
		internal_common.ParseAddr(address), blk,
	)
}

// GetValidatorInformationByBlockNumber returns information about a validator.
func (s *PublicBlockchainService) GetValidatorInformationByBlockNumber(
	ctx context.Context, address string, blockNr rpc.BlockNumber,
) (*staking.ValidatorRPCEnhanced, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNr); err != nil {
		return nil, err
	}
	blk, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNr))
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNr)
	}
	return s.hmy.GetValidatorInformation(
		internal_common.ParseAddr(address), blk,
	)
}

func (s *PublicBlockchainService) getAllValidatorInformation(
	ctx context.Context, page int, blockNr rpc.BlockNumber,
) ([]*staking.ValidatorRPCEnhanced, error) {
	if page < -1 {
		return nil, errors.Errorf("page given %d cannot be less than -1", page)
	}
	addresses := s.hmy.GetAllValidatorAddresses()
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return make([]*staking.ValidatorRPCEnhanced, 0), nil
	}
	validatorsNum := len(addresses)
	start := 0
	if page != -1 {
		validatorsNum = validatorsPageSize
		start = page * validatorsPageSize
		if len(addresses)-start < validatorsPageSize {
			validatorsNum = len(addresses) - start
		}
	}
	validators := []*staking.ValidatorRPCEnhanced{}
	blk, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNr))
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNr)
	}
	for i := start; i < start+validatorsNum; i++ {
		information, err := s.hmy.GetValidatorInformation(addresses[i], blk)
		if err == nil {
			validators = append(validators, information)
		}
	}
	return validators, nil
}

// GetAllValidatorInformation returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicBlockchainService) GetAllValidatorInformation(
	ctx context.Context, page int,
) ([]*staking.ValidatorRPCEnhanced, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	blockNr := s.hmy.CurrentBlock().NumberU64()

	// delete cache for previous block
	prevKey := fmt.Sprintf("all-info-%d", blockNr-1)
	s.hmy.SingleFlightForgetKey(prevKey)

	key := fmt.Sprintf("all-info-%d", blockNr)
	res, err := s.hmy.SingleFlightRequest(
		key,
		func() (interface{}, error) {
			return s.getAllValidatorInformation(ctx, page, rpc.LatestBlockNumber)
		},
	)
	if err != nil {
		return nil, err
	}
	return res.([]*staking.ValidatorRPCEnhanced), nil
}

// GetAllValidatorInformationByBlockNumber returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicBlockchainService) GetAllValidatorInformationByBlockNumber(
	ctx context.Context, page int, blockNr rpc.BlockNumber,
) ([]*staking.ValidatorRPCEnhanced, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNr); err != nil {
		return nil, err
	}
	return s.getAllValidatorInformation(ctx, page, blockNr)
}

// GetAllDelegationInformation returns delegation information about `validatorsPageSize` validators,
// starting at `page*validatorsPageSize`.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicBlockchainService) GetAllDelegationInformation(ctx context.Context, page int) ([][]*RPCDelegation, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if page < -1 {
		return make([][]*RPCDelegation, 0), nil
	}
	addresses := s.hmy.GetAllValidatorAddresses()
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return make([][]*RPCDelegation, 0), nil
	}
	validatorsNum := len(addresses)
	start := 0
	if page != -1 {
		validatorsNum = validatorsPageSize
		start = page * validatorsPageSize
		if len(addresses)-start < validatorsPageSize {
			validatorsNum = len(addresses) - start
		}
	}
	validators := make([][]*RPCDelegation, validatorsNum)
	var err error
	for i := start; i < start+validatorsNum; i++ {
		validators[i-start], err = s.GetDelegationsByValidator(ctx, addresses[i].String())
		if err != nil {
			return nil, err
		}
	}
	return validators, nil
}

// GetDelegationsByDelegator returns list of delegations for a delegator address.
func (s *PublicBlockchainService) GetDelegationsByDelegator(ctx context.Context, address string) ([]*RPCDelegation, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	delegatorAddress := internal_common.ParseAddr(address)
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)
	result := []*RPCDelegation{}
	for i := range delegations {
		delegation := delegations[i]

		undelegations := make([]RPCUndelegation, len(delegation.Undelegations))

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validators[i])
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)
		result = append(result, &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		})
	}
	return result, nil
}

// GetDelegationsByDelegatorByBlockNumber returns list of delegations for a delegator address at given block number
func (s *PublicBlockchainService) GetDelegationsByDelegatorByBlockNumber(
	ctx context.Context, address string, blockNum rpc.BlockNumber,
) ([]*RPCDelegation, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	if err := s.isBlockGreaterThanLatest(blockNum); err != nil {
		return nil, err
	}
	delegatorAddress := internal_common.ParseAddr(address)
	blk, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNum))
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}
	validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(delegatorAddress, blk)
	result := make([]*RPCDelegation, len(delegations))
	for i := range delegations {
		delegation := delegations[i]

		undelegations := make([]RPCUndelegation, len(delegation.Undelegations))

		for j := range delegation.Undelegations {
			undelegations[j] = RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			}
		}
		valAddr, _ := internal_common.AddressToBech32(validators[i])
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)
		result[i] = &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		}
	}
	return result, nil
}

// GetDelegationsByValidator returns list of delegations for a validator address.
func (s *PublicBlockchainService) GetDelegationsByValidator(ctx context.Context, address string) ([]*RPCDelegation, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	validatorAddress := internal_common.ParseAddr(address)
	delegations := s.hmy.GetDelegationsByValidator(validatorAddress)
	result := make([]*RPCDelegation, 0)
	for _, delegation := range delegations {

		undelegations := []RPCUndelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegation.DelegatorAddress)
		result = append(result, &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		})
	}
	return result, nil
}

// GetDelegationByDelegatorAndValidator returns a delegation for delegator and validator.
func (s *PublicBlockchainService) GetDelegationByDelegatorAndValidator(ctx context.Context, address string, validator string) (*RPCDelegation, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	delegatorAddress := internal_common.ParseAddr(address)
	validatorAddress := internal_common.ParseAddr(validator)
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)
	for i := range delegations {
		if validators[i] != validatorAddress {
			continue
		}
		delegation := delegations[i]

		undelegations := []RPCUndelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)
		return &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		}, nil
	}
	return nil, nil
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockchainService) EstimateGas(ctx context.Context, args CallArgs) (hexutil.Uint64, error) {
	return doEstimateGas(ctx, s.hmy, args, nil)
}

// GetCurrentUtilityMetrics ..
func (s *PublicBlockchainService) GetCurrentUtilityMetrics() (*network.UtilityMetric, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	return s.hmy.GetCurrentUtilityMetrics()
}

// GetSuperCommittees ..
func (s *PublicBlockchainService) GetSuperCommittees() (*quorum.Transition, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	return s.hmy.GetSuperCommittees()
}

// GetCurrentBadBlocks ..
func (s *PublicBlockchainService) GetCurrentBadBlocks() []core.BadBlock {
	return s.hmy.GetCurrentBadBlocks()
}

// GetTotalSupply ..
func (s *PublicBlockchainService) GetTotalSupply() (numeric.Dec, error) {
	return numeric.NewDec(initSupply), nil
}

// GetCirculatingSupply ..
func (s *PublicBlockchainService) GetCirculatingSupply() (numeric.Dec, error) {
	timestamp := time.Now()
	return numeric.NewDec(initSupply).Mul(reward.PercentageForTimeStamp(timestamp.Unix())), nil
}

// GetStakingNetworkInfo ..
func (s *PublicBlockchainService) GetStakingNetworkInfo(
	ctx context.Context,
) (*StakingNetworkInfo, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	totalStaking, _ := s.GetTotalStaking()
	round, _ := s.GetMedianRawStakeSnapshot()
	epoch := s.LatestHeader(ctx).Epoch
	epochLastBlock, _ := s.EpochLastBlock(epoch)
	totalSupply, _ := s.GetTotalSupply()
	circulatingSupply, _ := s.GetCirculatingSupply()
	return &StakingNetworkInfo{
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		EpochLastBlock:    epochLastBlock,
		TotalStaking:      totalStaking,
		MedianRawStake:    round.MedianStake,
	}, nil
}

// GetLastCrossLinks ..
func (s *PublicBlockchainService) GetLastCrossLinks() ([]*types.CrossLink, error) {
	if err := s.isBeaconShard(); err != nil {
		return nil, err
	}
	return s.hmy.GetLastCrossLinks()
}

// docall executes an EVM call
func doCall(ctx context.Context, hmy *hmy.Harmony, args CallArgs, blockNr rpc.BlockNumber, vmCfg vm.Config, timeout time.Duration, globalGasCap *big.Int) ([]byte, uint64, bool, error) {
	defer func(start time.Time) {
		utils.Logger().Debug().
			Dur("runtime", time.Since(start)).
			Msg("Executing EVM call finished")
	}(time.Now())

	state, header, err := hmy.StateAndHeaderByNumber(ctx, blockNr)
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

	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}

	var data []byte
	if args.Data != nil {
		data = []byte(*args.Data)
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
	return res, gas, failed, err
}

// doEstimateGas ..
func doEstimateGas(ctx context.Context, hmy *hmy.Harmony, args CallArgs, gasCap *big.Int) (hexutil.Uint64, error) {
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
	return hexutil.Uint64(hi), nil
}
