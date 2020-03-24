package apr

import (
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Reader ..
type Reader interface {
	GetHeaderByNumber(number uint64) *block.Header
	Config() *params.ChainConfig
	GetHeaderByHash(hash common.Hash) *block.Header
	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *block.Header
	CurrentHeader() *block.Header
	ReadValidatorSnapshotAtEpoch(
		epoch *big.Int,
		addr common.Address,
	) (*staking.ValidatorWrapper, error)
}

const (
	secondsInYear = int64(31557600)
)

var (
	oneYear = big.NewInt(int64(secondsInYear))
)

func expectedRewardPerYear(
	now, oneEpochAgo *block.Header,
	curValidator, snapshotLastEpoch *staking.ValidatorWrapper,
) (*big.Int, error) {
	timeNow, oneTAgo := now.Time(), oneEpochAgo.Time()
	diffTime, diffReward :=
		new(big.Int).Sub(timeNow, oneTAgo),
		new(big.Int).Sub(curValidator.BlockReward, snapshotLastEpoch.BlockReward)

	// impossibility but keep sane
	if diffTime.Sign() == -1 {
		return nil, errors.New("time stamp diff cannot be negative")
	}
	if diffTime.Cmp(common.Big0) == 0 {
		return nil, errors.New("cannot div by zero of diff in time")
	}

	// TODO some more sanity checks of some sort?
	expectedValue := new(big.Int).Div(diffReward, diffTime)
	expectedPerYear := new(big.Int).Mul(expectedValue, oneYear)
	utils.Logger().Info().Interface("now", curValidator).Interface("before", snapshotLastEpoch).
		Uint64("diff-reward", diffReward.Uint64()).
		Uint64("diff-time", diffTime.Uint64()).
		Interface("expected-value", expectedValue).
		Interface("expected-per-year", expectedPerYear).
		Msg("expected reward per year computed")
	return expectedPerYear, nil
}

var (
	zero = numeric.ZeroDec()
)

// ComputeForValidator ..
func ComputeForValidator(
	bc Reader,
	block *types.Block,
	validatorNow *staking.ValidatorWrapper,
) (*numeric.Dec, error) {
	oneEpochAgo, zero :=
		new(big.Int).Sub(block.Epoch(), common.Big1),
		numeric.ZeroDec()

	utils.Logger().Info().
		Uint64("now", block.Epoch().Uint64()).
		Uint64("one-epoch-ago", oneEpochAgo.Uint64()).
		Msg("apr - begin compute for validator ")

	oneSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
		oneEpochAgo,
		validatorNow.Address,
	)

	if err != nil {
		return &zero, nil
	}

	blockNumAtOneEpochAgo := shard.Schedule.EpochLastBlock(oneEpochAgo.Uint64())

	headerOneEpochAgo := bc.GetHeaderByNumber(blockNumAtOneEpochAgo)
	if block.Header() == nil || headerOneEpochAgo == nil || err != nil {
		utils.Logger().Debug().
			Msgf("apr compute headers epochs ago %+v %+v %+v",
				oneEpochAgo,
				blockNumAtOneEpochAgo,
				headerOneEpochAgo,
			)
		return &zero, nil
	}

	utils.Logger().Info().
		RawJSON("current-epoch-header", []byte(bc.CurrentHeader().String())).
		RawJSON("one-epoch-ago-header", []byte(headerOneEpochAgo.String())).
		Msg("headers used for apr computation")

	estimatedRewardPerYear, err := expectedRewardPerYear(
		block.Header(), headerOneEpochAgo,
		validatorNow, oneSnapshotAgo,
	)

	if err != nil {
		return nil, err
	}

	if estimatedRewardPerYear.Cmp(common.Big0) == 0 {
		return &zero, nil
	}

	total := numeric.NewDecFromBigInt(validatorNow.TotalDelegation())
	if total.IsZero() {
		return nil, errors.New("zero total delegation will cause div by zero")
	}

	result := numeric.NewDecFromBigInt(estimatedRewardPerYear).Quo(
		total,
	)
	return &result, nil
}
