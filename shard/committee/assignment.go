package committee

import (
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/staking/availability"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// ValidatorListProvider ..
type ValidatorListProvider interface {
	Compute(
		epoch *big.Int, reader DataProvider,
	) (*shard.State, error)
	ReadFromDB(epoch *big.Int, reader DataProvider) (*shard.State, error)
}

// Reader is committee.Reader and it is the API that committee membership assignment needs
type Reader interface {
	ValidatorListProvider
}

// StakingCandidatesReader ..
type StakingCandidatesReader interface {
	CurrentBlock() *types.Block
	ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error)
	ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorWrapper, error)
	ValidatorCandidates() []common.Address
}

// CandidatesForEPoS ..
type CandidatesForEPoS struct {
	Orders                             map[common.Address]effective.SlotOrder
	OpenSlotCountForExternalValidators int
}

// CompletedEPoSRound ..
type CompletedEPoSRound struct {
	MedianStake         numeric.Dec              `json:"epos-median-stake"`
	MaximumExternalSlot int                      `json:"max-external-slots"`
	AuctionWinners      []effective.SlotPurchase `json:"epos-slot-winners"`
	AuctionCandidates   []*CandidateOrder        `json:"epos-slot-candidates"`
}

// CandidateOrder ..
type CandidateOrder struct {
	*effective.SlotOrder
	Validator common.Address
}

// MarshalJSON ..
func (p CandidateOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*effective.SlotOrder
		Validator string `json:"validator"`
	}{
		p.SlotOrder, common2.MustAddressToBech32(p.Validator),
	})
}

// NewEPoSRound runs a fresh computation of EPoS using
// latest data always
func NewEPoSRound(stakedReader StakingCandidatesReader) (
	*CompletedEPoSRound, error,
) {
	eligibleCandidate, err := prepareOrders(stakedReader)
	if err != nil {
		return nil, err
	}
	maxExternalSlots := shard.ExternalSlotsAvailableForEpoch(
		stakedReader.CurrentBlock().Epoch(),
	)
	median, winners := effective.Apply(
		eligibleCandidate, maxExternalSlots,
	)
	auctionCandidates := make([]*CandidateOrder, len(eligibleCandidate))

	i := 0
	for key := range eligibleCandidate {
		auctionCandidates[i] = &CandidateOrder{
			SlotOrder: eligibleCandidate[key],
			Validator: key,
		}
		i++
	}

	return &CompletedEPoSRound{
		MedianStake:         median,
		MaximumExternalSlot: maxExternalSlots,
		AuctionWinners:      winners,
		AuctionCandidates:   auctionCandidates,
	}, nil
}

func prepareOrders(
	stakedReader StakingCandidatesReader,
) (map[common.Address]*effective.SlotOrder, error) {
	candidates := stakedReader.ValidatorCandidates()
	blsKeys := map[shard.BlsPublicKey]struct{}{}
	essentials := map[common.Address]*effective.SlotOrder{}
	totalStaked, tempZero := big.NewInt(0), numeric.ZeroDec()

	for i := range candidates {
		validator, err := stakedReader.ReadValidatorInformation(
			candidates[i],
		)
		if err != nil {
			return nil, err
		}
		snapshot, err := stakedReader.ReadValidatorSnapshot(
			candidates[i],
		)
		if err != nil {
			return nil, err
		}
		if !IsEligibleForEPoSAuction(snapshot, validator) {
			continue
		}

		validatorStake := big.NewInt(0)
		for i := range validator.Delegations {
			validatorStake.Add(
				validatorStake, validator.Delegations[i].Amount,
			)
		}

		totalStaked.Add(totalStaked, validatorStake)

		found := false
		for _, key := range validator.SlotPubKeys {
			if _, ok := blsKeys[key]; ok {
				found = true
			} else {
				blsKeys[key] = struct{}{}
			}
		}

		if found {
			continue
		}

		essentials[validator.Address] = &effective.SlotOrder{
			validatorStake,
			validator.SlotPubKeys,
			tempZero,
		}
	}
	totalStakedDec := numeric.NewDecFromBigInt(totalStaked)

	for _, value := range essentials {
		value.Percentage = numeric.NewDecFromBigInt(value.Stake).Quo(totalStakedDec)
	}

	return essentials, nil
}

// IsEligibleForEPoSAuction ..
func IsEligibleForEPoSAuction(snapshot, validator *staking.ValidatorWrapper) bool {
	if snapshot.Counters.NumBlocksToSign.Cmp(validator.Counters.NumBlocksToSign) != 0 {
		// validator was in last epoch's committee
		// validator with below-threshold signing activity won't be considered for next epoch
		// and their status will be turned to inactive in FinalizeNewBlock
		computed := availability.ComputeCurrentSigning(snapshot, validator)
		if computed.IsBelowThreshold {
			return false
		}
	}
	// For validators who were not in last epoch's committee
	// or for those who were and signed enough blocks,
	// the decision is based on the status
	switch validator.Status {
	case effective.Active:
		return true
	default:
		return false
	}
}

// ChainReader is a subset of Engine.ChainReader, just enough to do assignment
type ChainReader interface {
	// ReadShardState retrieves sharding state given the epoch number.
	// This api reads the shard state cached or saved on the chaindb.
	// Thus, only should be used to read the shard state of the current chain.
	ReadShardState(epoch *big.Int) (*shard.State, error)
	// GetHeader retrieves a block header from the database by hash and number.
	GetHeaderByHash(common.Hash) *block.Header
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig
	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *block.Header
}

// DataProvider ..
type DataProvider interface {
	StakingCandidatesReader
	ChainReader
}

type partialStakingEnabled struct{}

var (
	// WithStakingEnabled ..
	WithStakingEnabled Reader = partialStakingEnabled{}
	// ErrComputeForEpochInPast ..
	ErrComputeForEpochInPast = errors.New("cannot compute for epoch in past")
)

func preStakingEnabledCommittee(s shardingconfig.Instance) *shard.State {
	shardNum := int(s.NumShards())
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()
	shardSize := s.NumNodesPerShard()
	hmyAccounts := s.HmyAccounts()
	fnAccounts := s.FnAccounts()
	shardState := &shard.State{}
	// Shard state needs to be sorted by shard ID
	for i := 0; i < shardNum; i++ {
		com := shard.Committee{ShardID: uint32(i)}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardNum // The initial account to use for genesis nodes
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hmyAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.Slot{
				common2.ParseAddr(hmyAccounts[index].Address),
				pubKey,
				nil,
			}
			com.Slots = append(com.Slots, curNodeID)
		}
		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.Slot{
				common2.ParseAddr(fnAccounts[index].Address),
				pubKey,
				nil,
			}
			com.Slots = append(com.Slots, curNodeID)
		}
		shardState.Shards = append(shardState.Shards, com)
	}
	return shardState
}

func eposStakedCommittee(
	s shardingconfig.Instance, stakerReader DataProvider,
) (*shard.State, error) {
	shardCount := int(s.NumShards())
	shardState := &shard.State{}
	shardState.Shards = make([]shard.Committee, shardCount)
	hAccounts := s.HmyAccounts()
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()

	for i := 0; i < shardCount; i++ {
		shardState.Shards[i] = shard.Committee{uint32(i), shard.SlotList{}}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardCount
			pub := &bls.PublicKey{}
			if err := pub.DeserializeHexStr(hAccounts[index].BlsPublicKey); err != nil {
				return nil, err
			}
			pubKey := shard.BlsPublicKey{}
			if err := pubKey.FromLibBLSPublicKey(pub); err != nil {
				return nil, err
			}
			shardState.Shards[i].Slots = append(shardState.Shards[i].Slots, shard.Slot{
				common2.ParseAddr(hAccounts[index].Address),
				pubKey,
				nil,
			})
		}
	}

	completedEPoSRound, err := NewEPoSRound(stakerReader)

	if err != nil {
		return nil, err
	}

	shardBig := big.NewInt(int64(shardCount))
	for i := range completedEPoSRound.AuctionWinners {
		purchasedSlot := completedEPoSRound.AuctionWinners[i]
		shardID := int(new(big.Int).Mod(purchasedSlot.Key.Big(), shardBig).Int64())
		shardState.Shards[shardID].Slots = append(shardState.Shards[shardID].Slots, shard.Slot{
			purchasedSlot.Addr,
			purchasedSlot.Key,
			&purchasedSlot.Stake,
		})
	}

	return shardState, nil
}

// ReadFromDB is a wrapper on ReadShardState
func (def partialStakingEnabled) ReadFromDB(
	epoch *big.Int, reader DataProvider,
) (newSuperComm *shard.State, err error) {
	return reader.ReadShardState(epoch)
}

// Compute is single entry point for
// computing a new super committee, aka new shard state
func (def partialStakingEnabled) Compute(
	epoch *big.Int, stakerReader DataProvider,
) (newSuperComm *shard.State, err error) {
	preStaking := true
	if stakerReader != nil {
		config := stakerReader.Config()
		if config.IsStaking(epoch) {
			preStaking = false
		}
	}

	instance := shard.Schedule.InstanceForEpoch(epoch)
	if preStaking {
		// Pre-staking shard state doesn't need to set epoch (backward compatible)
		return preStakingEnabledCommittee(instance), nil
	}
	// Sanity check, can't compute against epochs in past
	if e := stakerReader.CurrentHeader().Epoch(); epoch.Cmp(e) == -1 {
		utils.Logger().Error().Uint64("header-epoch", e.Uint64()).
			Uint64("compute-epoch", epoch.Uint64()).
			Msg("Tried to compute committee for epoch in past")
		return nil, ErrComputeForEpochInPast
	}
	utils.AnalysisStart("computeEPoSStakedCommittee")
	shardState, err := eposStakedCommittee(instance, stakerReader)
	utils.AnalysisEnd("computeEPoSStakedCommittee")

	if err != nil {
		return nil, err
	}

	// Set the epoch of shard state
	shardState.Epoch = big.NewInt(0).Set(epoch)
	utils.Logger().Info().
		Uint64("computed-for-epoch", epoch.Uint64()).
		Msg("computed new super committee")
	return shardState, nil
}
