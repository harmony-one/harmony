package apr

import (
	"math/big"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	// ErrInsufficientEpoch is returned when insufficient past epochs for apr computation
	ErrInsufficientEpoch = errors.New("insufficient past epochs to compute apr")
	// ErrCouldNotRetreiveHeaderByNumber is returned when fail to retrieve header by number
	ErrCouldNotRetreiveHeaderByNumber = errors.New("could not retrieve header by number")
	// ErrZeroStakeOneEpochAgo is returned when total delegation is zero for one epoch ago
	ErrZeroStakeOneEpochAgo = errors.New("zero total delegation one epoch ago")
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
	) (*staking.ValidatorSnapshot, error)
}

const (
	secondsInYear = int64(31557600)
)

var (
	oneYear = big.NewInt(int64(secondsInYear))
)

func expectedRewardPerYear(
	now, oneEpochAgo *block.Header,
	wrapper, snapshot *staking.ValidatorWrapper,
) (*big.Int, error) {
	timeNow, oneTAgo := now.Time(), oneEpochAgo.Time()
	diffTime, diffReward :=
		new(big.Int).Sub(timeNow, oneTAgo),
		new(big.Int).Sub(wrapper.BlockReward, snapshot.BlockReward)

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
	utils.Logger().Info().Interface("now", wrapper).Interface("before", snapshot).
		Uint64("diff-reward", diffReward.Uint64()).
		Uint64("diff-time", diffTime.Uint64()).
		Interface("expected-value", expectedValue).
		Interface("expected-per-year", expectedPerYear).
		Msg("expected reward per year computed")
	return expectedPerYear, nil
}

// ComputeForValidator ..
func ComputeForValidator(
	bc Reader,
	block *types.Block,
	wrapper *staking.ValidatorWrapper,
) (*numeric.Dec, error) {
	oneEpochAgo, zero :=
		new(big.Int).Sub(block.Epoch(), common.Big1),
		numeric.ZeroDec()

	utils.Logger().Debug().
		Uint64("now", block.Epoch().Uint64()).
		Uint64("one-epoch-ago", oneEpochAgo.Uint64()).
		Msg("apr - begin compute for validator ")

	snapshot, err := bc.ReadValidatorSnapshotAtEpoch(
		block.Epoch(),
		wrapper.Address,
	)

	if err != nil {
		return nil, errors.Wrapf(
			ErrInsufficientEpoch,
			"current epoch %d, one-epoch-ago %d",
			block.Epoch().Uint64(),
			oneEpochAgo.Uint64(),
		)
	}

	blockNumAtOneEpochAgo := shard.Schedule.EpochLastBlock(oneEpochAgo.Uint64())
	headerOneEpochAgo := bc.GetHeaderByNumber(blockNumAtOneEpochAgo)

	if headerOneEpochAgo == nil {
		utils.Logger().Debug().
			Msgf("apr compute headers epochs ago %+v %+v %+v",
				oneEpochAgo,
				blockNumAtOneEpochAgo,
				headerOneEpochAgo,
			)
		return nil, errors.Wrapf(
			ErrCouldNotRetreiveHeaderByNumber,
			"num header wanted %d",
			blockNumAtOneEpochAgo,
		)
	}

	estimatedRewardPerYear, err := expectedRewardPerYear(
		block.Header(), headerOneEpochAgo,
		wrapper, snapshot.Validator,
	)

	if err != nil {
		return nil, err
	}

	if estimatedRewardPerYear.Cmp(common.Big0) == 0 {
		return &zero, nil
	}

	total := numeric.NewDecFromBigInt(snapshot.Validator.TotalDelegation())
	if total.IsZero() {
		return nil, errors.Wrapf(
			ErrZeroStakeOneEpochAgo,
			"current epoch %d, one-epoch-ago %d",
			block.Epoch().Uint64(),
			oneEpochAgo.Uint64(),
		)
	}

	result := numeric.NewDecFromBigInt(estimatedRewardPerYear).Quo(
		total,
	)
	return &result, nil
}
