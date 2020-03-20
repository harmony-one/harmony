package apr

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Reader ..
type Reader interface {
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
	secondsInYear = int64(31_557_600)
)

var (
	oneYear = big.NewInt(int64(secondsInYear))
)

func expectedRewardPerYear(
	oneEpochAgo, twoEpochAgo *block.Header,
	oneSnapshotAgo, twoSnapshotAgo *staking.ValidatorWrapper,
	blocksPerEpoch uint64,
) (*big.Int, error) {
	oneTAgo, twoTAgo := oneEpochAgo.Time(), twoEpochAgo.Time()
	diffTime, diffReward :=
		new(big.Int).Sub(twoTAgo, oneTAgo),
		new(big.Int).Sub(twoSnapshotAgo.BlockReward, oneSnapshotAgo.BlockReward)

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
	utils.Logger().Info().
		Uint64("diff-reward", diffReward.Uint64()).
		Uint64("diff-time", diffTime.Uint64()).
		Uint64("expected-value", expectedValue.Uint64()).
		Uint64("expected-per-year", expectedPerYear.Uint64()).
		Msg("expected reward per year computed")
	return expectedPerYear, nil
}

func pastTwoEpochHeaders(
	bc Reader,
) (*block.Header, *block.Header, error) {
	current := bc.CurrentHeader()
	epochNow := current.Epoch()
	oneEpochAgo, twoEpochAgo :=
		new(big.Int).Sub(epochNow, common.Big1),
		new(big.Int).Sub(epochNow, common.Big2)

	bottomOut := new(big.Int).Add(
		bc.Config().StakingEpoch,
		common.Big3,
	)

	var oneAgoHeader, twoAgoHeader **block.Header

	for e1, e2 := false, false; ; {
		current = bc.GetHeader(current.ParentHash(), current.Number().Uint64()-1)

		if current == nil {
			return nil, nil, errors.New("could not go up parent")
		}

		if current.Epoch().Cmp(bottomOut) == 0 {
			if twoAgoHeader == nil || oneAgoHeader == nil {
				return nil, nil, errors.New(
					"could not find headers for apr computation",
				)
			}
		}

		switch {
		// haven't found either epoch yet
		case !e1 && !e2:
			if current.Epoch().Cmp(oneEpochAgo) == 0 {
				e1 = true
				oneAgoHeader = &current
				continue
			}
		case e1 && !e2:
			if current.Epoch().Cmp(twoEpochAgo) == 0 {
				e2 = true
				twoAgoHeader = &current
				break
			}
		}
	}

	return *oneAgoHeader, *twoAgoHeader, nil
}

var (
	zero = numeric.ZeroDec()
)

// ComputeForValidator ..
func ComputeForValidator(
	bc Reader,
	now *big.Int,
	state *state.DB,
	validatorNow *staking.ValidatorWrapper,
	blocksPerEpoch uint64,
) (*numeric.Dec, error) {
	return &zero, nil
	// twoEpochAgo, oneEpochAgo, zero :=
	// 	new(big.Int).Sub(now, common.Big2),
	// 	new(big.Int).Sub(now, common.Big1),
	// 	numeric.ZeroDec()

	// utils.Logger().Info().
	// 	Uint64("now", now.Uint64()).
	// 	Uint64("two-epoch-ago", twoEpochAgo.Uint64()).
	// 	Uint64("one-epoch-ago", oneEpochAgo.Uint64()).
	// 	Msg("apr - begin compute for validator ")

	// twoSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
	// 	twoEpochAgo,
	// 	validatorNow.Address,
	// )

	// if err != nil {
	// 	return &zero, nil
	// }

	// oneSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
	// 	oneEpochAgo,
	// 	validatorNow.Address,
	// )

	// if err != nil {
	// 	return &zero, nil
	// }

	// blockNumAtTwoEpochAgo, blockNumAtOneEpochAgo :=
	// 	shard.Schedule.EpochLastBlock(twoEpochAgo.Uint64()),
	// 	shard.Schedule.EpochLastBlock(oneEpochAgo.Uint64())

	// headerOneEpochAgo, headerTwoEpochAgo, err := pastTwoEpochHeaders(bc)

	// // TODO Figure out why this is happening
	// if headerOneEpochAgo == nil || headerTwoEpochAgo == nil || err != nil {
	// 	utils.Logger().Debug().
	// 		Msgf("apr compute headers epochs ago %+v %+v %+v %+v %+v %+v",
	// 			twoEpochAgo, oneEpochAgo,
	// 			blockNumAtTwoEpochAgo, blockNumAtOneEpochAgo,
	// 			headerOneEpochAgo, headerTwoEpochAgo,
	// 		)
	// 	return &zero, nil
	// }

	// utils.Logger().Info().
	// 	RawJSON("current-epoch-header", []byte(bc.CurrentHeader().String())).
	// 	RawJSON("one-epoch-ago-header", []byte(headerOneEpochAgo.String())).
	// 	RawJSON("two-epoch-ago-header", []byte(headerTwoEpochAgo.String())).
	// 	Msg("headers used for apr computation")

	// estimatedRewardPerYear, err := expectedRewardPerYear(
	// 	headerOneEpochAgo, headerTwoEpochAgo,
	// 	oneSnapshotAgo, twoSnapshotAgo,
	// 	blocksPerEpoch,
	// )

	// if err != nil {
	// 	return nil, err
	// }

	// if estimatedRewardPerYear.Cmp(common.Big0) == 0 {
	// 	return &zero, nil
	// }

	// total := numeric.NewDecFromBigInt(validatorNow.TotalDelegation())
	// if total.IsZero() {
	// 	return nil, errors.New("zero total delegation will cause div by zero")
	// }

	// result := numeric.NewDecFromBigInt(estimatedRewardPerYear).Quo(
	// 	total,
	// )
	// return &result, nil
}
