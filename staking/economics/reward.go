package economics

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// Produced is a record rewards given out after a successful round of consensus
type Produced struct {
	blockNumber uint64
	accum       votepower.RewardAccumulation
}

// NewProduced ..
func NewProduced(b uint64, r []votepower.VoterReward, t *big.Int) *Produced {
	return &Produced{b, votepower.RewardAccumulation{t, r}}
}

// ReadBlockNumber ..
func (p *Produced) ReadBlockNumber() uint64 {
	return p.blockNumber
}

// Read ..
func (p *Produced) Read() *votepower.RewardAccumulation {
	return &p.accum
}

// ReadRewarded ..
func (p *Produced) ReadRewarded() []votepower.VoterReward {
	return p.accum.ValidatorReward
}

// ReadTotalPayout ..
func (p *Produced) ReadTotalPayout() *big.Int {
	return p.accum.NetworkTotalPayout
}

const (
	nanoSecondsInYear = time.Nanosecond * 3.154e+16
	initSupply        = 12_600_000_000
)

var (
	// BlockReward is the block reward, to be split evenly among block signers.
	BlockReward = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))
	// BaseStakedReward is the base block reward for epos.
	BaseStakedReward = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))
	totalTokens = numeric.NewDecFromBigInt(
		new(big.Int).Mul(big.NewInt(initSupply), big.NewInt(denominations.One)),
	)
	targetStakedPercentage = numeric.MustNewDecFromStr("0.35")
	dynamicAdjust          = numeric.MustNewDecFromStr("0.4")
	oneHundred             = numeric.NewDec(100)
	potentialAdjust        = oneHundred.Mul(dynamicAdjust)
	zero                   = numeric.ZeroDec()
	// ErrPayoutNotEqualBlockReward ..
	ErrPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
	// ErrNotTwoEpochsPastStaking ..
	ErrNotTwoEpochsPastStaking = errors.New("two epochs ago was not >= staking epoch")
	oneYear                    = big.NewInt(int64(nanoSecondsInYear))
)

// ExpectedValueBlocksPerYear ..
func ExpectedValueBlocksPerYear(
	oneEpochAgo, twoEpochAgo *block.Header, blocksPerEpoch int64,
) (numeric.Dec, error) {
	oneTAgo, twoTAgo := oneEpochAgo.Time(), twoEpochAgo.Time()
	diff := new(big.Int).Sub(twoTAgo, oneTAgo)
	// impossibility but keep sane
	if diff.Sign() == -1 {
		return numeric.ZeroDec(), errors.New("time stamp diff cannot be negative")
	}
	avgBlockTimePerEpoch := new(big.Int).Div(diff, big.NewInt(blocksPerEpoch))
	return numeric.NewDecFromBigInt(new(big.Int).Div(oneYear, avgBlockTimePerEpoch)), nil
}

// NewNoReward ..
func NewNoReward(blockNum uint64) *Produced {
	return &Produced{blockNum, votepower.RewardAccumulation{common.Big0, nil}}
}

func adjust(amount numeric.Dec) numeric.Dec {
	return amount.MulTruncate(
		numeric.NewDecFromBigInt(big.NewInt(denominations.One)),
	)
}

// Adjustment ..
func Adjustment(percentageStaked numeric.Dec) (numeric.Dec, numeric.Dec) {
	howMuchOff := targetStakedPercentage.Sub(percentageStaked)
	adjustBy := adjust(
		howMuchOff.MulTruncate(oneHundred).Mul(dynamicAdjust),
	)
	return howMuchOff, adjustBy
}

// Snapshot ..
type Snapshot struct {
	Rewards          *votepower.RewardAccumulation `json:"accumulated-rewards"`
	APR              []ComputedAPR                 `json:"active-validators-apr"`
	StakedPercentage *numeric.Dec                  `json:"current-percent-token-staked"`
}

// NewSnapshot returns a record with metrics on
// the network accumulated rewards,
// and by validator.
func NewSnapshot(
	beaconchain engine.ChainReader,
	timestamp int64,
	includeAPRs bool,
) (*Snapshot, error) {
	headerNow := beaconchain.CurrentHeader()
	blockNow := headerNow.Number().Uint64()
	blocksPerEpoch := shard.Schedule.BlocksPerEpoch()
	oneEpochAgo := beaconchain.GetHeaderByNumber(blockNow - blocksPerEpoch)
	twoEpochAgo := beaconchain.GetHeaderByNumber(blockNow - (blocksPerEpoch * 2))

	if !beaconchain.Config().IsStaking(twoEpochAgo.Header.Epoch()) {
		return nil, ErrNotTwoEpochsPastStaking
	}

	soFarDoledOut, err := beaconchain.ReadBlockRewardAccumulator(blockNow)

	if err != nil {
		return nil, err
	}

	stakedNow, rates, junk :=
		numeric.ZeroDec(), []ComputedAPR{}, numeric.ZeroDec()
	// Only active validators' stake is counted in
	// stake ratio because only their stake is under slashing risk
	active, err := beaconchain.ReadActiveValidatorList()
	if err != nil {
		return nil, err
	}

	if includeAPRs {
		rates = make([]ComputedAPR, len(active))
	}

	dole := numeric.NewDecFromBigInt(soFarDoledOut.NetworkTotalPayout)

	for i := range active {
		wrapper, err := beaconchain.ReadValidatorInformation(active[i])
		if err != nil {
			return nil, err
		}
		total := wrapper.TotalDelegation()
		stakedNow = stakedNow.Add(numeric.NewDecFromBigInt(total))
		if includeAPRs {
			rates[i] = ComputedAPR{active[i], total, junk, numeric.ZeroDec()}
		}
	}

	circulatingSupply := totalTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	).Add(dole)

	if blocksPerYear, err := ExpectedValueBlocksPerYear(
		oneEpochAgo, twoEpochAgo, int64(blocksPerEpoch),
	); includeAPRs && err != nil {
		for i := range rates {
			rates[i].StakeRatio = numeric.NewDecFromBigInt(
				rates[i].TotalStakedToken,
			).Quo(circulatingSupply)
			if reward := BaseStakedReward.Sub(
				rates[i].StakeRatio.Sub(targetStakedPercentage).Mul(potentialAdjust),
			); reward.GT(zero) {
				rates[i].APR = blocksPerYear.Mul(reward).Quo(stakedNow)
			}
		}
	}

	percentage := stakedNow.Quo(circulatingSupply)
	utils.Logger().Info().
		Str("so-far-doled-out", dole.String()).
		Str("staked-percentage", percentage.String()).
		Str("currently-staked", stakedNow.String()).
		Msg("Computed how much staked right now")
	return &Snapshot{soFarDoledOut, rates, &percentage}, nil
}
