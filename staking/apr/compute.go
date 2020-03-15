package apr

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Reader ..
type Reader interface {
	// TODO Remove this field
	ReadValidatorStats(addr common.Address) (
		*staking.ValidatorStats, error,
	)

	ReadValidatorSnapshotAtEpoch(
		epoch *big.Int,
		addr common.Address,
	) (*staking.ValidatorWrapper, error)
}

const (
	nanoSecondsInYear = time.Nanosecond * 3.154e+16
)

var (
	oneYear = big.NewInt(int64(nanoSecondsInYear))
	// ErrNotTwoEpochsAgo ..
	ErrNotTwoEpochsAgo = errors.New("not two epochs ago yet")
	// ErrNotOneEpochsAgo ..
	ErrNotOneEpochsAgo = errors.New("not one epoch ago yet")
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

	expectedValue := new(big.Int).Div(diffReward, diffTime)
	return new(big.Int).Mul(expectedValue, oneYear), nil
}

// ComputeForValidator ..
func ComputeForValidator(
	bc Reader,
	now *big.Int,
	state *state.DB,
	validatorNow *staking.ValidatorWrapper,
	blocksPerEpoch uint64,
) (*big.Int, error) {
	twoSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
		new(big.Int).Sub(now, common.Big2),
		validatorNow.Address,
	)

	if err != nil {
		return nil, ErrNotTwoEpochsAgo
	}

	oneSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
		new(big.Int).Sub(now, common.Big1),
		validatorNow.Address,
	)

	if err != nil {
		return nil, ErrNotOneEpochsAgo
	}

	// avg_reward_per_block = total_reward_per_epoch / blocks_per_epoch
	// estimated_reward_per_year = avg_reward_per_block * blocks_per_year
	// estimated_reward_per_year / total_stake
	estimatedRewardPerYear, err := expectedRewardPerYear(
		nil, nil,
		oneSnapshotAgo, twoSnapshotAgo,
		blocksPerEpoch,
	)

	return new(big.Int).Div(
		estimatedRewardPerYear, validatorNow.TotalDelegation(),
	), nil
}
