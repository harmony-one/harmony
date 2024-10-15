package rpc

import (
	"context"
	"math/big"
	"reflect"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

const (
	validatorsPageSize = 100

	validatorInfoCacheSize = 128
)

// PublicStakingService provides an API to access Harmony's staking services.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicStakingService struct {
	hmy     *hmy.Harmony
	version Version

	validatorInfoCache *lru.Cache // cache for detailed validator information per page and block
	// TEMP SOLUTION to rpc node spamming issue
	limiterGetAllValidatorInformation  *rate.Limiter
	limiterGetAllDelegationInformation *rate.Limiter
	limiterGetDelegationsByValidator   *rate.Limiter
}

// NewPublicStakingAPI creates a new API for the RPC interface
func NewPublicStakingAPI(hmy *hmy.Harmony, version Version) rpc.API {
	viCache, _ := lru.New(validatorInfoCacheSize)
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service: &PublicStakingService{
			hmy:                                hmy,
			version:                            version,
			validatorInfoCache:                 viCache,
			limiterGetAllValidatorInformation:  rate.NewLimiter(1, 3),
			limiterGetAllDelegationInformation: rate.NewLimiter(1, 3),
			limiterGetDelegationsByValidator:   rate.NewLimiter(5, 20),
		},
		Public: true,
	}
}

func (s *PublicStakingService) wait(limiter *rate.Limiter, ctx context.Context) error {
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

// getBalanceByBlockNumber returns balance by block number at given eth blockNum without checks
func (s *PublicStakingService) getBalanceByBlockNumber(
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

// GetTotalStaking returns total staking by validators, only meant to be called on beaconchain
// explorer node
func (s *PublicStakingService) GetTotalStaking(
	ctx context.Context,
) (*big.Int, error) {
	timer := DoMetricRPCRequest(GetTotalStaking)
	defer DoRPCRequestDuration(GetTotalStaking, timer)

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Response output is the same for all versions
	return s.hmy.GetTotalStakingSnapshot(), nil
}

// GetMedianRawStakeSnapshot returns the raw median stake, only meant to be called on beaconchain
// explorer node
func (s *PublicStakingService) GetMedianRawStakeSnapshot(
	ctx context.Context,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetMedianRawStakeSnapshot)
	defer DoRPCRequestDuration(GetMedianRawStakeSnapshot, timer)

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Fetch snapshot
	snapshot, err := s.hmy.GetMedianRawStakeSnapshot()
	if err != nil {
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(snapshot)
}

// GetElectedValidatorAddresses returns elected validator addresses.
func (s *PublicStakingService) GetElectedValidatorAddresses(
	ctx context.Context,
) ([]string, error) {
	timer := DoMetricRPCRequest(GetElectedValidatorAddresses)
	defer DoRPCRequestDuration(GetElectedValidatorAddresses, timer)

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
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

// GetValidators returns validators list for a particular epoch.
func (s *PublicStakingService) GetValidators(
	ctx context.Context, epoch int64,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetValidators)
	defer DoRPCRequestDuration(GetValidators, timer)

	// Fetch the Committee
	cmt, err := s.hmy.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}
	balanceQueryBlock := shard.Schedule.EpochLastBlock(uint64(epoch))
	if balanceQueryBlock > s.hmy.CurrentBlock().NumberU64() {
		balanceQueryBlock = s.hmy.CurrentBlock().NumberU64()
	}

	validators := []StructuredResponse{}
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
		var validatorsFields StructuredResponse
		switch s.version {
		case V1:
			validatorsFields = StructuredResponse{
				"address": oneAddress,
				"balance": (*hexutil.Big)(validatorBalance),
			}
		case V2:
			validatorsFields = StructuredResponse{
				"address": oneAddress,
				"balance": validatorBalance,
			}
		default:
			return nil, ErrUnknownRPCVersion
		}
		validators = append(validators, validatorsFields)
	}
	result := StructuredResponse{
		"shardID":    cmt.ShardID,
		"validators": validators,
	}
	return result, nil
}

// GetAllValidatorAddresses returns all validator addresses.
func (s *PublicStakingService) GetAllValidatorAddresses(
	ctx context.Context,
) ([]string, error) {
	timer := DoMetricRPCRequest(GetAllValidatorAddresses)
	defer DoRPCRequestDuration(GetAllValidatorAddresses, timer)

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
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

// GetValidatorKeys returns list of bls public keys in the committee for a particular epoch.
func (s *PublicStakingService) GetValidatorKeys(
	ctx context.Context, epoch int64,
) ([]string, error) {
	timer := DoMetricRPCRequest(GetValidatorKeys)
	defer DoRPCRequestDuration(GetValidatorKeys, timer)

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

// GetAllValidatorInformation returns information about all validators.
func (s *PublicStakingService) GetAllValidatorInformation(
	ctx context.Context, page int,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetAllValidatorInformation)
	defer DoRPCRequestDuration(GetAllValidatorInformation, timer)

	err := s.wait(s.limiterGetAllValidatorInformation, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetAllValidatorInformation, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetAllValidatorInformation, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	res, err := s.getPagedValidatorInformationCached(ctx, page, LatestBlockNumber)
	if err != nil {
		DoMetricRPCQueryInfo(GetAllValidatorInformation, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return res, nil
}

// GetAllValidatorInformationByBlockNumber returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicStakingService) GetAllValidatorInformationByBlockNumber(
	ctx context.Context, page int, blockNumber BlockNumber,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetAllValidatorInformationByBlockNumber)
	defer DoRPCRequestDuration(GetAllValidatorInformationByBlockNumber, timer)

	err := s.wait(s.limiterGetAllValidatorInformation, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetAllValidatorInformationByBlockNumber, RateLimitedNumber)
		return nil, err
	}

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetAllValidatorInformationByBlockNumber, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetAllValidatorInformationByBlockNumber, FailedNumber)
		return nil, ErrRequestedBlockTooHigh
	}

	// Response output is the same for all versions
	res, err := s.getPagedValidatorInformationCached(ctx, page, blockNumber)
	if err != nil {
		DoMetricRPCQueryInfo(GetAllValidatorInformationByBlockNumber, FailedNumber)
		return nil, err
	}
	return res, nil
}

func (s *PublicStakingService) getPagedValidatorInformationCached(ctx context.Context, page int, blockNumber BlockNumber) (interface{}, error) {
	type cacheKey struct {
		bn   uint64
		page int
	}
	var bn uint64
	if blockNumber == LatestBlockNumber {
		bn = s.hmy.CurrentBlock().NumberU64()
	} else {
		bn = uint64(blockNumber)
	}

	key := cacheKey{bn, page}
	val, ok := s.validatorInfoCache.Get(key)
	if ok && val != nil {
		return val, nil
	}

	res, err := s.getAllValidatorInformation(ctx, page, bn)
	if err != nil {
		return nil, err
	}
	s.validatorInfoCache.Add(key, res)
	return res, nil
}

// getAllValidatorInformation is the helper function to get all validator information for a given eth block number
func (s *PublicStakingService) getAllValidatorInformation(
	ctx context.Context, page int, blockNum uint64,
) (interface{}, error) {
	if page < 0 {
		return nil, errors.Errorf("page given %d cannot be less than 0", page)
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()
	if len(addresses) <= page*validatorsPageSize {
		return []StructuredResponse{}, nil
	}

	// Set page start
	start := 0
	validatorsNum := validatorsPageSize
	start = page * validatorsPageSize
	if len(addresses)-start < validatorsPageSize {
		validatorsNum = len(addresses) - start
	}

	// Fetch block
	blk, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNum))
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}

	// Fetch validator information for block
	validators := make([]*staking.ValidatorRPCEnhanced, 0, validatorsNum)
	for i := start; i < start+validatorsNum; i++ {
		validatorInfo, err := s.hmy.GetValidatorInformation(addresses[i], blk)
		if err != nil {
			if errors.Cause(err) != state.ErrAddressNotPresent {
				return nil, err
			} else {
				// GetAllValidatorAddresses is as of current block
				// but we are querying state as of prior block
				// which means we can ignore ErrAddressNotPresent
				continue
			}
		}
		// Response output is the same for all versions
		validators = append(validators, validatorInfo)
	}
	return validators, nil
}

// GetValidatorInformation returns information about a validator.
func (s *PublicStakingService) GetValidatorInformation(
	ctx context.Context, address string,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetValidatorInformation)
	defer DoRPCRequestDuration(GetValidatorInformation, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetValidatorInformation, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch latest block
	blk, err := s.hmy.BlockByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformation, FailedNumber)
		return nil, errors.Wrapf(err, "could not retrieve the latest blk information")
	}

	// Fetch validator information
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformation, FailedNumber)
		return nil, err
	}
	validatorInfo, err := s.hmy.GetValidatorInformation(addr, blk)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformation, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(validatorInfo)
}

// GetValidatorInformationByBlockNumber returns information about a validator.
func (s *PublicStakingService) GetValidatorInformationByBlockNumber(
	ctx context.Context, address string, blockNumber BlockNumber,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetValidatorInformationByBlockNumber)
	defer DoRPCRequestDuration(GetValidatorInformationByBlockNumber, timer)

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetValidatorInformationByBlockNumber, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetValidatorInformationByBlockNumber, FailedNumber)
		return nil, ErrRequestedBlockTooHigh
	}
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformationByBlockNumber, FailedNumber)
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}

	// Fetch validator info
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformationByBlockNumber, FailedNumber)
		return nil, err
	}
	validatorInfo, err := s.hmy.GetValidatorInformation(addr, blk)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorInformationByBlockNumber, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(validatorInfo)
}

// GetValidatorsStakeByBlockNumber returns the stake per validator at the specified block
func (s *PublicStakingService) GetValidatorsStakeByBlockNumber(
	ctx context.Context, blockNumber BlockNumber,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetValidatorsStakeByBlockNumber)
	defer DoRPCRequestDuration(GetValidatorsStakeByBlockNumber, timer)

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetValidatorsStakeByBlockNumber, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		DoMetricRPCQueryInfo(GetValidatorsStakeByBlockNumber, FailedNumber)
		return nil, ErrRequestedBlockTooHigh
	}
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorsStakeByBlockNumber, FailedNumber)
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}
	response, err := s.hmy.GetValidatorsStakeByBlockNumber(blk)
	if err != nil {
		DoMetricRPCQueryInfo(GetValidatorsStakeByBlockNumber, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return NewStructuredResponse(response)
}

// GetValidatorSelfDelegation returns validator stake.
func (s *PublicStakingService) GetValidatorSelfDelegation(
	ctx context.Context, address string,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetValidatorSelfDelegation)
	defer DoRPCRequestDuration(GetValidatorSelfDelegation, timer)

	// Ensure node is for beacon shard
	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	// Fetch self delegation
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	selfDelegation := s.hmy.GetValidatorSelfDelegation(addr).Uint64()

	// Format the response according to the version
	switch s.version {
	case V1:
		return hexutil.Uint64(selfDelegation), nil
	case V2:
		return selfDelegation, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetValidatorTotalDelegation returns total balance stacking for validator with delegation.
func (s *PublicStakingService) GetValidatorTotalDelegation(
	ctx context.Context, address string,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetValidatorTotalDelegation)
	defer DoRPCRequestDuration(GetValidatorTotalDelegation, timer)

	// Ensure node is for beacon shard
	if s.hmy.ShardID != shard.BeaconChainShardID {
		return nil, ErrNotBeaconShard
	}

	// Fetch delegations & sum
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	delegations := s.hmy.GetDelegationsByValidator(addr)
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
		return nil, ErrUnknownRPCVersion
	}
}

// GetAllDelegationInformation returns delegation information about `validatorsPageSize` validators,
// starting at `page*validatorsPageSize`.
// TODO(dm): optimize with single flight
func (s *PublicStakingService) GetAllDelegationInformation(
	ctx context.Context, page int,
) ([][]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetAllDelegationInformation)
	defer DoRPCRequestDuration(GetAllDelegationInformation, timer)

	err := s.wait(s.limiterGetAllDelegationInformation, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetAllDelegationInformation, RateLimitedNumber)
		return nil, err
	}

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetAllDelegationInformation, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	if page < 0 {
		return make([][]StructuredResponse, 0), nil
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()

	// Return nothing if no delegation on page
	if len(addresses) <= page*validatorsPageSize {
		return make([][]StructuredResponse, 0), nil
	}

	// Set page start
	start := 0
	validatorsNum := validatorsPageSize
	start = page * validatorsPageSize
	if len(addresses)-start < validatorsPageSize {
		validatorsNum = len(addresses) - start
	}

	// Fetch all delegations
	validators := make([][]StructuredResponse, validatorsNum)
	for i := start; i < start+validatorsNum; i++ {
		validators[i-start], err = s.getDelegationByValidatorHelper(addresses[i].String())
		if err != nil {
			DoMetricRPCQueryInfo(GetAllDelegationInformation, FailedNumber)
			return nil, err
		}
	}

	// Response output is the same for all versions
	return validators, nil
}

// GetDelegationsByDelegator returns list of delegations for a delegator address.
func (s *PublicStakingService) GetDelegationsByDelegator(
	ctx context.Context, address string,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetDelegationsByDelegator)
	defer DoRPCRequestDuration(GetDelegationsByDelegator, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetDelegationsByDelegator, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch delegation
	delegatorAddress, err := internal_common.ParseAddr(address)
	if err != nil {
		DoMetricRPCQueryInfo(GetDelegationsByDelegator, FailedNumber)
		return nil, err
	}
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)

	// Format response
	result := []StructuredResponse{}
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
		del, err := NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
		if err != nil {
			DoMetricRPCQueryInfo(GetDelegationsByDelegator, FailedNumber)
			return nil, err
		}
		result = append(result, del)
	}
	return result, nil
}

// GetDelegationsByDelegatorByBlockNumber returns list of delegations for a delegator address at given block number
func (s *PublicStakingService) GetDelegationsByDelegatorByBlockNumber(
	ctx context.Context, aol AddressOrList, blockNumber BlockNumber,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetDelegationsByDelegatorByBlockNumber)
	defer DoRPCRequestDuration(GetDelegationsByDelegatorByBlockNumber, timer)

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}
	if isBlockGreaterThanLatest(s.hmy, blockNum) {
		return nil, ErrRequestedBlockTooHigh
	}
	blk, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve the blk information for blk number: %d", blockNum)
	}

	if aol.Address != nil { // single address
		delegatorAddress := *aol.Address
		validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(delegatorAddress, blk)
		return s.parseGetDelegationsByDelegatorResp(delegatorAddress, validators, delegations)

	} else { // multiple address
		srs := make([][]StructuredResponse, 0, len(aol.AddressList))
		for _, delegatorAddress := range aol.AddressList {
			validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(delegatorAddress, blk)
			res, err := s.parseGetDelegationsByDelegatorResp(delegatorAddress, validators, delegations)
			if err != nil {
				return nil, err
			}
			srs = append(srs, res)
		}
		return srs, nil
	}
}

func (s *PublicStakingService) parseGetDelegationsByDelegatorResp(
	delegator common.Address, validators []common.Address, delegations []*staking.Delegation,
) ([]StructuredResponse, error) {
	var result []StructuredResponse
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
		delAddr, _ := internal_common.AddressToBech32(delegator)

		// Response output is the same for all versions
		del, err := NewStructuredResponse(Delegation{
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
func (s *PublicStakingService) GetDelegationsByValidator(
	ctx context.Context, address string,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetDelegationsByValidator)
	defer DoRPCRequestDuration(GetDelegationsByValidator, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetDelegationsByValidator, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	err := s.wait(s.limiterGetDelegationsByValidator, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(GetDelegationsByValidator, RateLimitedNumber)
		return nil, err
	}
	return s.getDelegationByValidatorHelper(address)
}

func (s *PublicStakingService) getDelegationByValidatorHelper(address string) ([]StructuredResponse, error) {
	// Fetch delegations
	validatorAddress, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	delegations := s.hmy.GetDelegationsByValidator(validatorAddress)

	// Format response
	result := make([]StructuredResponse, 0, len(delegations))
	for _, delegation := range delegations {
		undelegations := make([]Undelegation, 0, len(delegation.Undelegations))

		for _, undelegation := range delegation.Undelegations {
			undelegations = append(undelegations, Undelegation{
				Amount: undelegation.Amount,
				Epoch:  undelegation.Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegation.DelegatorAddress)

		// Skip delegations with zero amount and empty undelegation
		if delegation.Amount.Cmp(common.Big0) == 0 && len(undelegations) == 0 {
			continue
		}

		// Response output is the same for all versions
		del := Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		}.IntoStructuredResponse()
		result = append(result, del)
	}
	return result, nil
}

// GetDelegationByDelegatorAndValidator returns a delegation for delegator and validator.
func (s *PublicStakingService) GetDelegationByDelegatorAndValidator(
	ctx context.Context, address string, validator string,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetDelegationByDelegatorAndValidator)
	defer DoRPCRequestDuration(GetDelegationByDelegatorAndValidator, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetDelegationByDelegatorAndValidator, FailedNumber)
		return nil, ErrNotBeaconShard
	}

	// Fetch delegations
	delegatorAddress, err := internal_common.ParseAddr(address)
	if err != nil {
		DoMetricRPCQueryInfo(GetDelegationByDelegatorAndValidator, FailedNumber)
		return nil, err
	}
	validatorAddress, err := internal_common.ParseAddr(validator)
	if err != nil {
		DoMetricRPCQueryInfo(GetDelegationByDelegatorAndValidator, FailedNumber)
		return nil, err
	}
	validators, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddress)

	// Format response
	for i := range delegations {
		if validators[i] != validatorAddress {
			continue
		}
		delegation := delegations[i]
		undelegations := make([]Undelegation, len(delegation.Undelegations))

		for j := range delegation.Undelegations {
			undelegations[j] = Undelegation{
				Amount: delegation.Undelegations[j].Amount,
				Epoch:  delegation.Undelegations[j].Epoch,
			}
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)

		// Response output is the same for all versions
		return NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
	}
	return nil, nil
}

// GetAvailableRedelegationBalance returns the amount of locked undelegated tokens
func (s *PublicStakingService) GetAvailableRedelegationBalance(
	ctx context.Context, address string,
) (*big.Int, error) {
	timer := DoMetricRPCRequest(GetAvailableRedelegationBalance)
	defer DoRPCRequestDuration(GetAvailableRedelegationBalance, timer)

	if !isBeaconShard(s.hmy) {
		return nil, ErrNotBeaconShard
	}

	currEpoch := s.hmy.BlockChain.CurrentHeader().Epoch()

	delegatorAddr, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	_, delegations := s.hmy.GetDelegationsByDelegator(delegatorAddr)

	redelegationTotal := big.NewInt(0)
	for _, d := range delegations {
		for _, u := range d.Undelegations {
			if u.Epoch.Cmp(currEpoch) < 1 { // Undelegation.Epoch < currentEpoch
				redelegationTotal.Add(redelegationTotal, u.Amount)
			}
		}
	}
	return redelegationTotal, nil
}

func isBeaconShard(hmy *hmy.Harmony) bool {
	return hmy.ShardID == shard.BeaconChainShardID
}
