package apr

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Reader ..
type Reader interface {
	ReadValidatorStats(addr common.Address) *staking.ValidatorStats
	ReadValidatorSnapshot(
		addr common.Address,
	) (*staking.ValidatorWrapper, error)
}

const (
	nanoSecondsInYear = time.Nanosecond * 3.154e+16
)

var (
	oneYear = big.NewInt(int64(nanoSecondsInYear))
)

func expectedNumBlocksPerYear(
	oneEpochAgo, twoEpochAgo *block.Header, blocksPerEpoch uint64,
) (*big.Int, error) {
	oneTAgo, twoTAgo := oneEpochAgo.Time(), twoEpochAgo.Time()
	diff := new(big.Int).Sub(twoTAgo, oneTAgo)
	// impossibility but keep sane
	if diff.Sign() == -1 {
		return nil, errors.New("time stamp diff cannot be negative")
	}
	avgBlockTimePerEpoch := new(big.Int).Div(
		diff, big.NewInt(int64(blocksPerEpoch)),
	)
	return new(big.Int).Div(oneYear, avgBlockTimePerEpoch), nil
}

// ComputeForValidator ..
func ComputeForValidator(
	bc Reader,
	state *state.DB,
	wrapper *staking.ValidatorWrapper,
	blocksPerEpoch uint64,
) (*big.Int, error) {
	stats, err := bc.ReadValidatorStats(wrapper.Address)
	if err != nil {
		return nil, err
	}
	snapshot, err := bc.ReadValidatorSnapshot(wrapper.Address)
	if err != nil {
		return nil, err
	}
	// avg_reward_per_block = total_reward_per_epoch / blocks_per_epoch
	// estimated_reward_per_year = avg_reward_per_block * blocks_per_year
	// estimated_reward_per_year / total_stake
	perEpoch := new(big.Int).SetUint64(blocksPerEpoch)
	avgRewardPerBlock := new(big.Int).Div(stats.BlockReward, perEpoch)
	estimatedBlocksPerYear, err := expectedNumBlocksPerYear(
		nil, nil, blocksPerEpoch,
	)
	estimatedRewardPerYear := new(big.Int).Div(
		avgRewardPerBlock, estimatedBlocksPerYear,
	)

	return new(big.Int).Div(estimatedRewardPerYear, snapshot.TotalDelegation()), nil
}
