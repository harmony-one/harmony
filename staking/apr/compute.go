package apr

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// Reader ..
type Reader interface {
	GetHeaderByNumber(number uint64) *block.Header
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
	// TODO some more sanity checks of some sort?

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
	twoEpochAgo, oneEpochAgo :=
		new(big.Int).Sub(now, common.Big2),
		new(big.Int).Sub(now, common.Big1)

	twoSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
		twoEpochAgo,
		validatorNow.Address,
	)

	if err != nil {
		return nil, ErrNotTwoEpochsAgo
	}

	oneSnapshotAgo, err := bc.ReadValidatorSnapshotAtEpoch(
		oneEpochAgo,
		validatorNow.Address,
	)

	if err != nil {
		return nil, ErrNotOneEpochsAgo
	}

	blockNumAtTwoEpochAgo, blockNumAtOneEpochAgo :=
		shard.Schedule.EpochLastBlock(twoEpochAgo.Uint64()),
		shard.Schedule.EpochLastBlock(oneEpochAgo.Uint64())

	estimatedRewardPerYear, err := expectedRewardPerYear(
		bc.GetHeaderByNumber(blockNumAtOneEpochAgo),
		bc.GetHeaderByNumber(blockNumAtTwoEpochAgo),
		oneSnapshotAgo,
		twoSnapshotAgo,
		blocksPerEpoch,
	)

	return new(big.Int).Div(
		estimatedRewardPerYear, validatorNow.TotalDelegation(),
	), nil
}
