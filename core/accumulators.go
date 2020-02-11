// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package core implements the Ethereum consensus protocol.
package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/shard"
)

// ReadBlockRewardAccumulator must only be called on beaconchain
func (bc *BlockChain) ReadBlockRewardAccumulator(number uint64) (*big.Int, error) {
	if !bc.chainConfig.IsStaking(shard.Schedule.CalcEpochNumber(number)) {
		return big.NewInt(0), nil
	}
	if cached, ok := bc.blockAccumulatorCache.Get(number); ok {
		return cached.(*big.Int), nil
	}
	return rawdb.ReadBlockRewardAccumulator(bc.db, number)
}

// WriteBlockRewardAccumulator directly writes the BlockRewardAccumulator value
// Note: this should only be called once during staking launch.
func (bc *BlockChain) WriteBlockRewardAccumulator(reward *big.Int, number uint64) error {
	err := rawdb.WriteBlockRewardAccumulator(bc.db, reward, number)
	if err != nil {
		return err
	}
	bc.blockAccumulatorCache.Add(number, reward)
	return nil
}

//UpdateBlockRewardAccumulator ..
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) UpdateBlockRewardAccumulator(diff *big.Int, number uint64) error {
	current, err := bc.ReadBlockRewardAccumulator(number - 1)
	if err != nil {
		// one-off fix for pangaea, return after pangaea enter staking.
		current = big.NewInt(0)
		bc.WriteBlockRewardAccumulator(current, number)
	}
	return bc.WriteBlockRewardAccumulator(new(big.Int).Add(current, diff), number)
}

// ReadValidatorRewardAccumulator ..
func (bc *BlockChain) ReadValidatorRewardAccumulator(
	epoch *big.Int, addr common.Address,
) (*votepower.ValidatorReward, error) {
	if !bc.chainConfig.IsStaking(epoch) {
		return nil, nil
	}
	if cached, ok := bc.validatorRewardAccumulatorCache.Get(epoch.Uint64()); ok {
		return cached.(*votepower.ValidatorReward), nil
	}
	return rawdb.ReadValidatorRewardAccumulator(bc.db, addr, epoch)
}

// WriteValidatorRewardAccumulator directly writes the BlockRewardAccumulator value
// Note: this should only be called once during staking launch.
func (bc *BlockChain) WriteValidatorRewardAccumulator(
	epoch *big.Int, reward *votepower.ValidatorReward,
) error {
	if err := rawdb.WriteValidatorRewardAccumulator(
		bc.db, epoch, reward,
	); err != nil {
		return err
	}
	bc.blockAccumulatorCache.Add(epoch.Uint64(), reward)
	return nil
}

// UpdateValidatorRewardAccumulator  ..
// Note: this should only be called within the blockchain insertBlock process.
func (bc *BlockChain) UpdateValidatorRewardAccumulator(
	epoch *big.Int, reward *votepower.ValidatorReward,
) error {
	current, err := bc.ReadValidatorRewardAccumulator(epoch, reward.Address)
	if err != nil {
		// one-off fix for pangaea, return after pangaea enter staking.
		// current = big.NewInt(0)
		// bc.WriteBlockRewardAccumulator(current, number)
	}
	//

	return bc.WriteValidatorRewardAccumulator(epoch, current)
}
