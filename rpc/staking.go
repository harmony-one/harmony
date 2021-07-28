package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

const (
	validatorsPageSize = 100
)

// PublicStakingService provides an API to access Harmony's staking services.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicStakingService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicStakingAPI creates a new API for the RPC interface
func NewPublicStakingAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicStakingService{hmy, version},
		Public:    true,
	}
}

// getBalanceByBlockNumber returns balance by block number at given eth blockNum without checks
func (s *PublicStakingService) getBalanceByBlockNumber(
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

// getAllValidatorInformation getHelper function to get all validator information for a given eth block number
func (s *PublicStakingService) getAllValidatorInformation(
	ctx context.Context, page int, blockNum rpc.BlockNumber,
) ([]StructuredResponse, error) {
	if page < -1 {
		return nil, errors.Errorf("page given %d cannot be less than -1", page)
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return []StructuredResponse{}, nil
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
	validators := []StructuredResponse{}
	for i := start; i < start+validatorsNum; i++ {
		validatorInfo, err := s.hmy.GetValidatorInformation(addresses[i], blk)
		if err == nil {
			// Response output is the same for all versions
			information, err := NewStructuredResponse(validatorInfo)
			if err != nil {
				return nil, err
			}
			validators = append(validators, information)
		}
	}
	return validators, nil
}

// GetTotalStaking returns total staking by validators, only meant to be called on beaconchain
// explorer node
func (s *PublicStakingService) GetTotalStaking(
	ctx context.Context,
) (*big.Int, error) {
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
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicStakingService) GetAllValidatorInformation(
	ctx context.Context, page int,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetAllValidatorInformation)
	defer DoRPCRequestDuration(GetAllValidatorInformation, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetAllValidatorInformation, FailedNumber)
		return nil, ErrNotBeaconShard
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
		DoMetricRPCQueryInfo(GetAllValidatorInformation, FailedNumber)
		return nil, err
	}

	// Response output is the same for all versions
	return res.([]StructuredResponse), nil
}

// GetAllValidatorInformationByBlockNumber returns information about all validators.
// If page is -1, return all instead of `validatorsPageSize` elements.
func (s *PublicStakingService) GetAllValidatorInformationByBlockNumber(
	ctx context.Context, page int, blockNumber BlockNumber,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetAllValidatorInformationByBlockNumber)
	defer DoRPCRequestDuration(GetAllValidatorInformationByBlockNumber, timer)

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
	return s.getAllValidatorInformation(ctx, page, blockNum)
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

// GetValidatorSelfDelegation returns validator stake.
func (s *PublicStakingService) GetValidatorSelfDelegation(
	ctx context.Context, address string,
) (interface{}, error) {
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
// If page is -1, return all instead of `validatorsPageSize` elements.
// TODO(dm): optimize with single flight
func (s *PublicStakingService) GetAllDelegationInformation(
	ctx context.Context, page int,
) ([][]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetAllDelegationInformation)
	defer DoRPCRequestDuration(GetAllDelegationInformation, timer)

	if !isBeaconShard(s.hmy) {
		DoMetricRPCQueryInfo(GetAllDelegationInformation, FailedNumber)
		return nil, ErrNotBeaconShard
	}
	if page < -1 {
		return make([][]StructuredResponse, 0), nil
	}

	// Get all validators
	addresses := s.hmy.GetAllValidatorAddresses()

	// Return nothing if no delegation on page
	if page != -1 && len(addresses) <= page*validatorsPageSize {
		return make([][]StructuredResponse, 0), nil
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
	validators := make([][]StructuredResponse, validatorsNum)
	var err error
	for i := start; i < start+validatorsNum; i++ {
		validators[i-start], err = s.GetDelegationsByValidator(ctx, addresses[i].String())
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

	// Fetch delegations
	validatorAddress, err := internal_common.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	delegations := s.hmy.GetDelegationsByValidator(validatorAddress)

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
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegation.DelegatorAddress)

		// Skip delegations with zero amount and empty undelegation
		if delegation.Amount.Cmp(common.Big0) == 0 && len(undelegations) == 0 {
			continue
		}

		// Response output is the same for all versions
		del, err := NewStructuredResponse(Delegation{
			ValidatorAddress: valAddr,
			DelegatorAddress: delAddr,
			Amount:           delegation.Amount,
			Reward:           delegation.Reward,
			Undelegations:    undelegations,
		})
		if err != nil {
			DoMetricRPCQueryInfo(GetDelegationsByValidator, FailedNumber)
			return nil, err
		}
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
