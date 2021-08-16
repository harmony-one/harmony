package chain

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
	"github.com/pkg/errors"
)

// Circulating supply calculation
var (
	// InaccessibleAddresses are a list of known eth addresses that cannot spend ONE tokens.
	InaccessibleAddresses = []common.Address{
		// one10000000000000000000000000000dead5shlag
		common.HexToAddress("0x7bDeF7Bdef7BDeF7BDEf7bDef7bdef7bdeF6E7AD"),
	}
)

// InaccessibleAddressInfo is the structure for inaccessible addresses
type InaccessibleAddressInfo struct {
	EthAddress common.Address
	Address    string // One address
	Balance    numeric.Dec
	Nonce      uint64
}

// GetCirculatingSupply get the circulating supply.
// The value is the total circulating supply sub the tokens burnt at
// inaccessible addresses.
// WARNING: only works on beacon chain if in staking era.
func GetCirculatingSupply(chain engine.ChainReader) (numeric.Dec, error) {
	total, err := getTotalCirculatingSupply(chain)
	if err != nil {
		return numeric.Dec{}, err
	}
	invalid, err := getAllInaccessibleTokens(chain)
	if err != nil {
		return numeric.Dec{}, err
	}
	if total.LT(invalid) {
		return numeric.Dec{}, errors.New("FATAL: negative circulating supply")
	}
	return total.Sub(invalid), nil
}

// GetInaccessibleAddressInfo return the information of all inaccessible
// addresses.
func GetInaccessibleAddressInfo(chain engine.ChainReader) ([]*InaccessibleAddressInfo, error) {
	return getAllInaccessibleAddresses(chain)
}

// GetInaccessibleTokens get the total inaccessible tokens.
// The amount is the sum of balance at all inaccessible addresses.
func GetInaccessibleTokens(chain engine.ChainReader) (numeric.Dec, error) {
	return getAllInaccessibleTokens(chain)
}

// getTotalCirculatingSupply using the following formula:
// (TotalInitialTokens * percentReleased) + PreStakingBlockRewards + StakingBlockRewards
//
// Note that PreStakingBlockRewards is set to the amount of rewards given out by the
// LAST BLOCK of the pre-staking era regardless of what the current block height is
// if network is in the pre-staking era. This is for implementation reasons, reference
// stakingReward.GetTotalPreStakingTokens for more details.
func getTotalCirculatingSupply(chain engine.ChainReader) (ret numeric.Dec, err error) {
	currHeader, timestamp := chain.CurrentHeader(), time.Now().Unix()
	stakingBlockRewards := big.NewInt(0)

	if chain.Config().IsStaking(currHeader.Epoch()) {
		if chain.ShardID() != shard.BeaconChainShardID {
			return numeric.Dec{}, stakingReward.ErrInvalidBeaconChain
		}
		if stakingBlockRewards, err = chain.ReadBlockRewardAccumulator(currHeader.Number().Uint64()); err != nil {
			return numeric.Dec{}, err
		}
	}

	releasedInitSupply := stakingReward.TotalInitialTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	)
	preStakingBlockRewards := stakingReward.GetTotalPreStakingTokens().Sub(stakingReward.TotalInitialTokens)
	return releasedInitSupply.Add(preStakingBlockRewards).Add(
		numeric.NewDecFromBigIntWithPrec(stakingBlockRewards, 18),
	), nil
}

func getAllInaccessibleTokens(chain engine.ChainReader) (numeric.Dec, error) {
	ais, err := getAllInaccessibleAddresses(chain)
	if err != nil {
		return numeric.Dec{}, err
	}
	total := numeric.NewDecFromBigIntWithPrec(big.NewInt(0), numeric.Precision)
	for _, ai := range ais {
		total = total.Add(ai.Balance)
	}
	return total, nil
}

func getAllInaccessibleAddresses(chain engine.ChainReader) ([]*InaccessibleAddressInfo, error) {
	state, err := chain.StateAt(chain.CurrentHeader().Root())
	if err != nil {
		return nil, err
	}

	accs := make([]*InaccessibleAddressInfo, 0, len(InaccessibleAddresses))
	for _, addr := range InaccessibleAddresses {
		nonce := state.GetNonce(addr)
		oneAddr, _ := common2.AddressToBech32(addr)
		balance := state.GetBalance(addr)
		accs = append(accs, &InaccessibleAddressInfo{
			EthAddress: addr,
			Address:    oneAddr,
			Balance:    numeric.NewDecFromBigIntWithPrec(balance, numeric.Precision),
			Nonce:      nonce,
		})
	}
	return accs, nil
}
