// Copyright 2016 The go-ethereum Authors
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

package core

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	staking2 "github.com/harmony-one/harmony/staking"
	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	staking "github.com/harmony-one/harmony/staking/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus_engine.Engine

	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash, uint64) *block.Header

	// ReadDelegationsByDelegator returns the validators list of a delegator
	ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error)

	ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error)

	// ReadValidatorSnapshot returns the snapshot of validator at the beginning of current epoch.
	ReadValidatorSnapshot(common.Address) (*staking.ValidatorSnapshot, error)

	// ReadValidatorList returns the list of all validators
	ReadValidatorList() ([]common.Address, error)

	// Config returns chain config
	Config() *params.ChainConfig
}

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(msg Message, header *block.Header, chain ChainContext, author *common.Address) vm.Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	var beneficiary common.Address
	if author == nil {
		beneficiary = common.Address{} // Ignore error, we're past header validation
	} else {
		beneficiary = *author
	}
	vrf := common.Hash{}
	if len(header.Vrf()) >= 32 {
		vrfAndProof := header.Vrf()
		copy(vrf[:], vrfAndProof[:32])
	}
	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		IsValidator: IsValidator,
		GetHash:     GetHashFn(header, chain),
		GetVRF:      GetVRFFn(header, chain),
		Origin:      msg.From(),
		Coinbase:    beneficiary,
		BlockNumber: header.Number(),
		EpochNumber: header.Epoch(),
		VRF:         vrf,
		Time:        header.Time(),
		GasLimit:    header.GasLimit(),
		GasPrice:    new(big.Int).Set(msg.GasPrice()),

		CanStaking:      header.ShardID() == shard.BeaconChainShardID, // Is it safe to get shard from header?
		CreateValidator: CreateValidatorFn(chain, header.Epoch(), header.Number()),
		EditValidator:   EditValidatorFn(chain, header.Epoch(), header.Number()),
		Delegate:        DelegateFn(chain, header.Epoch(), header.Number()),
		Undelegate:      UndelegateFn(chain, header.Epoch(), header.Number()),
		CollectRewards:  CollectRewardsFn(chain, header.Epoch(), header.Number()),

		GetDelegationsByDelegator: GetDelegationsByDelegatorFn(chain),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *block.Header, chain ChainContext) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.Number().Uint64() - 1: ref.ParentHash(),
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.ParentHash(), ref.Number().Uint64()-1); header != nil; header = chain.GetHeader(header.ParentHash(), header.Number().Uint64()-1) {
			cache[header.Number().Uint64()-1] = header.ParentHash()
			if n == header.Number().Uint64()-1 {
				return header.ParentHash()
			}
		}
		return common.Hash{}
	}
}

// GetVRFFn returns a GetVRFFn which retrieves header vrf by number
func GetVRFFn(ref *block.Header, chain ChainContext) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			curVRF := common.Hash{}
			if len(ref.Vrf()) >= 32 {
				vrfAndProof := ref.Vrf()
				copy(curVRF[:], vrfAndProof[:32])
			}

			cache = map[uint64]common.Hash{
				ref.Number().Uint64(): curVRF,
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.ParentHash(), ref.Number().Uint64()-1); header != nil; header = chain.GetHeader(header.ParentHash(), header.Number().Uint64()-1) {

			curVRF := common.Hash{}
			if len(header.Vrf()) >= 32 {
				vrfAndProof := header.Vrf()
				copy(curVRF[:], vrfAndProof[:32])
			}

			cache[header.Number().Uint64()] = curVRF

			if n == header.Number().Uint64() {
				return curVRF
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// IsValidator determines whether it is a validator address or not
func IsValidator(db vm.StateDB, addr common.Address) bool {
	return db.IsValidator(addr)
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int, txType types.TransactionType) {
	if txType == types.SameShardTx || txType == types.SubtractionOnly {
		db.SubBalance(sender, amount)
	}
	if txType == types.SameShardTx {
		db.AddBalance(recipient, amount)
	}
}

func CreateValidatorFn(chain ChainContext, epoch *big.Int, blockNum *big.Int) vm.CreateValidatorFunc {
	return func(db vm.StateDB, createValidator *staking.CreateValidator) error {
		wrapper, err := VerifyAndCreateValidatorFromMsg(
			db, chain, epoch, blockNum, createValidator,
		)
		if err != nil {
			return err
		}
		if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
			return err
		}
		db.SetValidatorFlag(createValidator.ValidatorAddress)
		db.SubBalance(createValidator.ValidatorAddress, createValidator.Amount)
		return nil
	}
}

func EditValidatorFn(chain ChainContext, epoch *big.Int, blockNum *big.Int) vm.EditValidatorFunc {
	return func(db vm.StateDB, editValidator *staking.EditValidator) error {
		wrapper, err := VerifyAndEditValidatorFromMsg(
			db, chain, epoch, blockNum, editValidator,
		)
		if err != nil {
			return err
		}
		return db.UpdateValidatorWrapper(wrapper.Address, wrapper)
	}
}

func DelegateFn(chain ChainContext, epoch *big.Int, blockNum *big.Int) vm.DelegateFunc {
	return func(db vm.StateDB, delegate *staking.Delegate) error {
		delegations, err := chain.ReadDelegationsByDelegator(delegate.DelegatorAddress)
		if err != nil {
			return err
		}
		updatedValidatorWrappers, balanceToBeDeducted, fromLockedTokens, err := VerifyAndDelegateFromMsg(
			db, epoch, delegate, delegations, chain.Config())
		if err != nil {
			return err
		}

		for _, wrapper := range updatedValidatorWrappers {
			if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
				return err
			}
		}

		db.SubBalance(delegate.DelegatorAddress, balanceToBeDeducted)

		if len(fromLockedTokens) > 0 {
			sortedKeys := []common.Address{}
			for key := range fromLockedTokens {
				sortedKeys = append(sortedKeys, key)
			}
			sort.SliceStable(sortedKeys, func(i, j int) bool {
				return bytes.Compare(sortedKeys[i][:], sortedKeys[j][:]) < 0
			})
			// Add log if everything is good
			for _, key := range sortedKeys {
				redelegatedToken, ok := fromLockedTokens[key]
				if !ok {
					return errors.New("Key missing for delegation receipt")
				}
				encodedRedelegationData := []byte{}
				addrBytes := key.Bytes()
				encodedRedelegationData = append(encodedRedelegationData, addrBytes...)
				encodedRedelegationData = append(encodedRedelegationData, redelegatedToken.Bytes()...)
				// The data field format is:
				// [first 20 bytes]: Validator address from which the locked token is used for redelegation.
				// [rest of the bytes]: the bigInt serialized bytes for the token amount.
				db.AddLog(&types.Log{
					Address:     delegate.DelegatorAddress,
					Topics:      []common.Hash{staking2.DelegateTopic},
					Data:        encodedRedelegationData,
					BlockNumber: blockNum.Uint64(),
				})
			}
		}
		return nil
	}
}

func UndelegateFn(chain ChainContext, epoch *big.Int, blockNum *big.Int) vm.UndelegateFunc {
	return func(db vm.StateDB, undelegate *staking.Undelegate) error {
		wrapper, err := VerifyAndUndelegateFromMsg(db, epoch, undelegate)
		if err != nil {
			return err
		}
		return db.UpdateValidatorWrapper(wrapper.Address, wrapper)
	}
}
func CollectRewardsFn(chain ChainContext, epoch *big.Int, blockNum *big.Int) vm.CollectRewardsFunc {
	return func(db vm.StateDB, collectRewards *staking.CollectRewards) error {
		if chain == nil {
			return errors.New("[CollectRewards] No chain context provided")
		}
		delegations, err := chain.ReadDelegationsByDelegator(collectRewards.DelegatorAddress)
		if err != nil {
			return err
		}
		updatedValidatorWrappers, totalRewards, err := VerifyAndCollectRewardsFromDelegation(
			db, delegations,
		)
		if err != nil {
			return err
		}
		for _, wrapper := range updatedValidatorWrappers {
			if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
				return err
			}
		}
		db.AddBalance(collectRewards.DelegatorAddress, totalRewards)

		// Add log if everything is good
		db.AddLog(&types.Log{
			Address:     collectRewards.DelegatorAddress,
			Topics:      []common.Hash{staking2.CollectRewardsTopic},
			Data:        totalRewards.Bytes(),
			BlockNumber: blockNum.Uint64(),
		})
		return nil
	}
}

func GetDelegationsByDelegatorFn(chain ChainContext) vm.GetDelegationsByDelegatorFuncView {
	return func(db vm.StateDB, delegator common.Address) ([]common.Address, []*staking.Delegation, error) {
		delegationIndexes, err := chain.ReadDelegationsByDelegator(delegator)
		if err != nil {
			return nil, nil, err
		}
		addresses := []common.Address{}
		delegations := []*staking.Delegation{}
		for i := range delegationIndexes {
			delegationI := delegationIndexes[i]
			wrapper, err := chain.ReadValidatorInformationAtState(delegationI.ValidatorAddress, db.(*state.DB))
			if err == nil && wrapper != nil {
				err = errors.New("no validator info")
			}
			if err != nil {
				return nil, nil, err
			}
			if uint64(len(wrapper.Delegations)) > delegationI.Index {
				delegations = append(delegations, &wrapper.Delegations[delegationI.Index])
				addresses = append(addresses, delegationI.ValidatorAddress)
			}
		}
		return addresses, delegations, nil
	}
}
