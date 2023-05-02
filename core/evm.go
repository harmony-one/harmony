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
	"errors"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash, uint64) *block.Header

	// ReadDelegationsByDelegator returns the validators list of a delegator
	ReadDelegationsByDelegator(common.Address) (stakingTypes.DelegationIndexes, error)

	// ReadDelegationsByDelegatorAt reads the addresses of validators delegated by a delegator at a given block
	ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (m stakingTypes.DelegationIndexes, err error)

	// ReadValidatorSnapshot returns the snapshot of validator at the beginning of current epoch.
	ReadValidatorSnapshot(common.Address) (*stakingTypes.ValidatorSnapshot, error)

	// ReadValidatorList returns the list of all validators
	ReadValidatorList() ([]common.Address, error)

	// Config returns chain config
	Config() *params.ChainConfig

	ShardID() uint32 // this is implemented by blockchain.go already
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
		CanTransfer:           CanTransfer,
		Transfer:              Transfer,
		GetHash:               GetHashFn(header, chain),
		GetVRF:                GetVRFFn(header, chain),
		IsValidator:           IsValidator,
		Origin:                msg.From(),
		GasPrice:              new(big.Int).Set(msg.GasPrice()),
		Coinbase:              beneficiary,
		GasLimit:              header.GasLimit(),
		BlockNumber:           header.Number(),
		EpochNumber:           header.Epoch(),
		Time:                  header.Time(),
		VRF:                   vrf,
		TxType:                0,
		CreateValidator:       CreateValidatorFn(header, chain),
		EditValidator:         EditValidatorFn(header, chain),
		Delegate:              DelegateFn(header, chain),
		Undelegate:            UndelegateFn(header, chain),
		CollectRewards:        CollectRewardsFn(header, chain),
		CalculateMigrationGas: CalculateMigrationGasFn(chain),
		ShardID:               chain.ShardID(),
		NumShards:             shard.Schedule.InstanceForEpoch(header.Epoch()).NumShards(),
	}
}

// HandleStakeMsgFn returns a function which accepts
// (1) the chain state database
// (2) the processed staking parameters
// the function can then be called through the EVM context
func CreateValidatorFn(ref *block.Header, chain ChainContext) vm.CreateValidatorFunc {
	// moved from state_transition.go to here, with some modifications
	return func(db vm.StateDB, rosettaTracer vm.RosettaTracer, createValidator *stakingTypes.CreateValidator) error {
		wrapper, err := VerifyAndCreateValidatorFromMsg(
			db, chain, ref.Epoch(), ref.Number(), createValidator,
		)
		if err != nil {
			return err
		}
		if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
			return err
		}
		db.SetValidatorFlag(createValidator.ValidatorAddress)
		db.SubBalance(createValidator.ValidatorAddress, createValidator.Amount)

		//add rosetta log
		if rosettaTracer != nil {
			rosettaTracer.AddRosettaLog(
				vm.CALL,
				&vm.RosettaLogAddressItem{
					Account: &createValidator.ValidatorAddress,
				},
				&vm.RosettaLogAddressItem{
					Account:    &createValidator.ValidatorAddress,
					SubAccount: &createValidator.ValidatorAddress,
					Metadata:   map[string]interface{}{"type": "delegation"},
				},
				createValidator.Amount,
			)
		}

		return nil
	}
}

func EditValidatorFn(ref *block.Header, chain ChainContext) vm.EditValidatorFunc {
	// moved from state_transition.go to here, with some modifications
	return func(db vm.StateDB, rosettaTracer vm.RosettaTracer, editValidator *stakingTypes.EditValidator) error {
		wrapper, err := VerifyAndEditValidatorFromMsg(
			db, chain, ref.Epoch(), ref.Number(), editValidator,
		)
		if err != nil {
			return err
		}
		return db.UpdateValidatorWrapper(wrapper.Address, wrapper)
	}
}

func DelegateFn(ref *block.Header, chain ChainContext) vm.DelegateFunc {
	// moved from state_transition.go to here, with some modifications
	return func(db vm.StateDB, rosettaTracer vm.RosettaTracer, delegate *stakingTypes.Delegate) error {
		delegations, err := chain.ReadDelegationsByDelegatorAt(delegate.DelegatorAddress, big.NewInt(0).Sub(ref.Number(), big.NewInt(1)))
		if err != nil {
			return err
		}
		updatedValidatorWrappers, balanceToBeDeducted, fromLockedTokens, err := VerifyAndDelegateFromMsg(
			db, ref.Epoch(), delegate, delegations, chain.Config())
		if err != nil {
			return err
		}
		for _, wrapper := range updatedValidatorWrappers {
			if err := db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper); err != nil {
				return err
			}
		}

		db.SubBalance(delegate.DelegatorAddress, balanceToBeDeducted)

		if rosettaTracer != nil && balanceToBeDeducted != big.NewInt(0) {
			//add rosetta log
			rosettaTracer.AddRosettaLog(
				vm.CALL,
				&vm.RosettaLogAddressItem{
					Account: &delegate.DelegatorAddress,
				},
				&vm.RosettaLogAddressItem{
					Account:    &delegate.DelegatorAddress,
					SubAccount: &delegate.ValidatorAddress,
					Metadata:   map[string]interface{}{"type": "delegation"},
				},
				balanceToBeDeducted,
			)
		}

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
					Topics:      []common.Hash{staking.DelegateTopic},
					Data:        encodedRedelegationData,
					BlockNumber: ref.Number().Uint64(),
				})

				//add rosetta log
				if rosettaTracer != nil {
					// copy from address
					fromAccount := common.BytesToAddress(key.Bytes())
					rosettaTracer.AddRosettaLog(
						vm.CALL,
						&vm.RosettaLogAddressItem{
							Account:    &delegate.DelegatorAddress,
							SubAccount: &fromAccount,
							Metadata:   map[string]interface{}{"type": "undelegation"},
						},
						&vm.RosettaLogAddressItem{
							Account:    &delegate.DelegatorAddress,
							SubAccount: &delegate.ValidatorAddress,
							Metadata:   map[string]interface{}{"type": "delegation"},
						},
						redelegatedToken,
					)
				}
			}
		}
		return nil
	}
}

func UndelegateFn(ref *block.Header, chain ChainContext) vm.UndelegateFunc {
	// moved from state_transition.go to here, with some modifications
	return func(db vm.StateDB, rosettaTracer vm.RosettaTracer, undelegate *stakingTypes.Undelegate) error {
		wrapper, err := VerifyAndUndelegateFromMsg(db, ref.Epoch(), undelegate)
		if err != nil {
			return err
		}

		//add rosetta log
		if rosettaTracer != nil {
			rosettaTracer.AddRosettaLog(
				vm.CALL,
				&vm.RosettaLogAddressItem{
					Account:    &undelegate.DelegatorAddress,
					SubAccount: &undelegate.ValidatorAddress,
					Metadata:   map[string]interface{}{"type": "delegation"},
				},
				&vm.RosettaLogAddressItem{
					Account:    &undelegate.DelegatorAddress,
					SubAccount: &undelegate.ValidatorAddress,
					Metadata:   map[string]interface{}{"type": "undelegation"},
				},
				undelegate.Amount,
			)
		}

		return db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper)
	}
}

func CollectRewardsFn(ref *block.Header, chain ChainContext) vm.CollectRewardsFunc {
	return func(db vm.StateDB, rosettaTracer vm.RosettaTracer, collectRewards *stakingTypes.CollectRewards) error {
		if chain == nil {
			return errors.New("[CollectRewards] No chain context provided")
		}
		delegations, err := chain.ReadDelegationsByDelegatorAt(collectRewards.DelegatorAddress, big.NewInt(0).Sub(ref.Number(), big.NewInt(1)))
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
			if err := db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper); err != nil {
				return err
			}
		}
		db.AddBalance(collectRewards.DelegatorAddress, totalRewards)

		// Add log if everything is good
		db.AddLog(&types.Log{
			Address:     collectRewards.DelegatorAddress,
			Topics:      []common.Hash{staking.CollectRewardsTopic},
			Data:        totalRewards.Bytes(),
			BlockNumber: ref.Number().Uint64(),
		})

		//add rosetta log
		if rosettaTracer != nil {
			rosettaTracer.AddRosettaLog(
				vm.CALL,
				nil,
				&vm.RosettaLogAddressItem{
					Account: &collectRewards.DelegatorAddress,
				},
				totalRewards,
			)
		}

		return nil
	}
}

//func MigrateDelegationsFn(ref *block.Header, chain ChainContext) vm.MigrateDelegationsFunc {
//	return func(db vm.StateDB, migrationMsg *stakingTypes.MigrationMsg) ([]interface{}, error) {
//		// get existing delegations
//		fromDelegations, err := chain.ReadDelegationsByDelegator(migrationMsg.From)
//		if err != nil {
//			return nil, err
//		}
//		// get list of modified wrappers
//		wrappers, delegates, err := VerifyAndMigrateFromMsg(db, migrationMsg, fromDelegations)
//		if err != nil {
//			return nil, err
//		}
//		// add to state db
//		for _, wrapper := range wrappers {
//			if err := db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper); err != nil {
//				return nil, err
//			}
//		}
//		return delegates, nil
//	}
//}

// calculate the gas for migration; no checks done here similar to other functions
// the checks are handled by staking_verifier.go, ex, if you try to delegate to an address
// who is not a validator - you will be charged all gas passed in two steps
// - 22k initially when gas is calculated
// - remainder when the tx inevitably is a no-op
// i have followed the same logic here, this only produces an error if can't read from db
func CalculateMigrationGasFn(chain ChainContext) vm.CalculateMigrationGasFunc {
	return func(db vm.StateDB, migrationMsg *stakingTypes.MigrationMsg, homestead bool, istanbul bool) (uint64, error) {
		var gas uint64 = 0
		delegations, err := chain.ReadDelegationsByDelegator(migrationMsg.From)
		if err != nil {
			return 0, err
		}
		for i := range delegations {
			delegationIndex := &delegations[i]
			wrapper, err := db.ValidatorWrapper(delegationIndex.ValidatorAddress, true, false)
			if err != nil {
				return 0, err
			}
			if uint64(len(wrapper.Delegations)) <= delegationIndex.Index {
				utils.Logger().Warn().
					Str("validator", delegationIndex.ValidatorAddress.String()).
					Uint64("delegation index", delegationIndex.Index).
					Int("delegations length", len(wrapper.Delegations)).
					Msg("Delegation index out of bound")
				return 0, errors.New("Delegation index out of bound")
			}
			foundDelegation := &wrapper.Delegations[delegationIndex.Index]
			// no need to migrate if amount and undelegations are 0
			if foundDelegation.Amount.Cmp(common.Big0) == 0 && len(foundDelegation.Undelegations) == 0 {
				continue
			}
			delegate := stakingTypes.Delegate{
				DelegatorAddress: migrationMsg.From,
				ValidatorAddress: delegationIndex.ValidatorAddress,
				Amount:           foundDelegation.Amount,
			}
			encoded, err := rlp.EncodeToBytes(delegate)
			if err != nil {
				return 0, err
			}
			thisGas, err := vm.IntrinsicGas(
				encoded,
				false,
				homestead,
				istanbul,
				false, // isValidatorCreation
			)
			if err != nil {
				return 0, err
			}
			// overflow when gas + thisGas > Math.MaxUint64
			// or Math.MaxUint64 < gas + thisGas
			// or Math.MaxUint64 - gas < thisGas
			if (math.MaxUint64 - gas) < thisGas {
				return 0, vm.ErrOutOfGas
			}
			gas += thisGas
		}
		if gas != 0 {
			return gas, nil
		} else {
			// base gas fee if nothing to do, for example, when
			// there are no delegations to migrate
			return vm.IntrinsicGas(
				[]byte{},
				false,
				homestead,
				istanbul,
				false, // isValidatorCreation
			)
		}
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
