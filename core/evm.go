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
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	staking "github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus_engine.Engine

	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash, uint64) *block.Header

	// GetHeaderByNumber retrieves a block header from the database by number
	GetHeaderByNumber(number uint64) *block.Header

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

	CurrentBlock() *types.Block
	ReadValidatorInformation(common.Address) (*stakingTypes.ValidatorWrapper, error)
	ReadValidatorInformationAtState(common.Address, *state.DB) (*stakingTypes.ValidatorWrapper, error)
	StateAt(common.Hash) (*state.DB, error)
	ValidatorCandidates() []common.Address
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
		CanTransfer:     CanTransfer,
		Transfer:        Transfer,
		IsValidator:     IsValidator,
		GetHash:         GetHashFn(header, chain),
		GetVRF:          GetVRFFn(header, chain),
		CreateValidator: CreateValidatorFn(header, chain),
		EditValidator:   EditValidatorFn(header, chain),
		Delegate:        DelegateFn(header, chain),
		Undelegate:      UndelegateFn(header, chain),
		CollectRewards:  CollectRewardsFn(header, chain),
		//MigrateDelegations:    MigrateDelegationsFn(header, chain),
		// CalculateMigrationGas: CalculateMigrationGasFn(chain),
		Origin:        msg.From(),
		Coinbase:      beneficiary,
		BlockNumber:   header.Number(),
		EpochNumber:   header.Epoch(),
		VRF:           vrf,
		Time:          header.Time(),
		GasLimit:      header.GasLimit(),
		GasPrice:      new(big.Int).Set(msg.GasPrice()),
		ShardID:       chain.ShardID(),
		RoStakingInfo: RoStakingInfoFn(header, chain),
		RoStakingGas:  RoStakingGasFn(header, chain),
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
		// Changed as a result of https://github.com/harmony-one/harmony/pull/3906#issuecomment-1080642074
		// To be in line with expectations for events to contain params as well
		if chain.Config().IsROStakingPrecompile(ref.Epoch()) {
			db.AddLog(&types.Log{
				Address:     collectRewards.DelegatorAddress,
				Topics:      []common.Hash{staking.CollectRewardsTopicV2},
				Data:        common.LeftPadBytes(totalRewards.Bytes(), 32),
				BlockNumber: ref.Number().Uint64(),
			})
		} else {
			db.AddLog(&types.Log{
				Address:     collectRewards.DelegatorAddress,
				Topics:      []common.Hash{staking.CollectRewardsTopic},
				Data:        totalRewards.Bytes(),
				BlockNumber: ref.Number().Uint64(),
			})
		}

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

// FetchStakingInfoFn is the EVM implementation of the read only staking precompile
// avoid import cycle by putting this function here
func RoStakingInfoFn(ref *block.Header, chain ChainContext) vm.RoStakingInfoFunc {
	return func(db vm.StateDB, roStakeMsg *stakingTypes.ReadOnlyStakeMsg) ([]byte, error) {
		switch roStakeMsg.What {
		case "DelegationByDelegatorAndValidator":
			{
				// either we iterate over ReadDelegationsByDelegator
				// or over wrapper.Delegations. since ReadDelegationsByDelegator
				// is only written at the end of a block, prefer wrapper.Delegations
				wrapper, err := db.ValidatorWrapper(roStakeMsg.ValidatorAddress, true, false)
				if err != nil {
					return nil, err
				}
				for _, delegation := range wrapper.Delegations {
					if bytes.Equal(
						delegation.DelegatorAddress[:],
						roStakeMsg.DelegatorAddress[:],
					) {
						return common.LeftPadBytes(
							delegation.Amount.Bytes(), 32,
						), nil
					}
				}
				return big.NewInt(0).Bytes(), nil
			}
		case "ValidatorMaxTotalDelegation":
			{
				wrapper, err := db.ValidatorWrapper(roStakeMsg.ValidatorAddress, true, false)
				if err != nil {
					return nil, err
				}
				return common.LeftPadBytes(
					wrapper.MaxTotalDelegation.Bytes(), 32,
				), nil
			}
		case "ValidatorTotalDelegation":
			{
				wrapper, err := db.ValidatorWrapper(roStakeMsg.ValidatorAddress, true, false)
				if err != nil {
					return nil, err
				}
				return common.LeftPadBytes(
					wrapper.TotalDelegation().Bytes(), 32,
				), nil
			}
		case "BalanceAvailableForRedelegation":
			{
				delegations, err := chain.ReadDelegationsByDelegatorAt(
					roStakeMsg.DelegatorAddress,
					big.NewInt(0).Sub(ref.Number(), big.NewInt(1)),
				)
				if err != nil {
					return nil, err
				}
				redelegationTotal := big.NewInt(0)
				for _, delegationIndex := range delegations {
					wrapper, err := db.ValidatorWrapper(delegationIndex.ValidatorAddress, true, false)
					if err != nil {
						return nil, err
					}
					if uint64(len(wrapper.Delegations)) > delegationIndex.Index {
						delegation := wrapper.Delegations[delegationIndex.Index]
						for _, undelegation := range delegation.Undelegations {
							if undelegation.Epoch.Cmp(ref.Epoch()) < 1 { // Undelegation.Epoch < currentEpoch
								redelegationTotal.Add(redelegationTotal, undelegation.Amount)
							}
						}
					}
				}
				return common.LeftPadBytes(
					redelegationTotal.Bytes(), 32,
				), nil
			}
		case "ValidatorCommissionRate":
			{
				wrapper, err := db.ValidatorWrapper(roStakeMsg.ValidatorAddress, true, false)
				if err != nil {
					return nil, err
				}
				// Solidity doesn't support float directly, so just return the bytes
				// the bytes can't be an exact number but can be used for comparison
				// for example, a 1.1 commission rate shows up as 1100000000000000000
				// divide by 1e18 (in Python) to get the actual rate 1.1
				return common.LeftPadBytes(
					wrapper.CommissionRates.Rate.Bytes(), 32,
				), nil
			}
		case "ValidatorStatus":
			{
				wrapper, err := db.ValidatorWrapper(roStakeMsg.ValidatorAddress, true, false)
				if err != nil {
					return nil, err
				}
				return wrapper.Status.Bytes(), nil
			}
		case "BalanceDelegatedByDelegator":
			{
				delegations, err := chain.ReadDelegationsByDelegator(roStakeMsg.DelegatorAddress)
				if err != nil {
					return nil, err
				}
				answer := big.NewInt(0)
				for _, delegation := range delegations {
					wrapper, err := db.ValidatorWrapper(delegation.ValidatorAddress, true, false)
					if err != nil {
						return nil, err
					}
					answer = answer.Add(answer, wrapper.Delegations[delegation.Index].Amount)
				}
				return common.LeftPadBytes(answer.Bytes(), 32), nil
			}
		case "MedianRawStakeSnapshot":
			{
				epoch := big.NewInt(0).Add(ref.Epoch(), big.NewInt(1))
				instance := shard.Schedule.InstanceForEpoch(epoch)
				res, err := committee.NewEPoSRound(epoch, chain, chain.Config().IsEPoSBound35(epoch), instance.SlotsLimit(), int(instance.NumShards()))
				if err != nil {
					return nil, err
				}
				return res.MedianStake.Bytes(), nil
			}
		case "TotalStakingSnapshot":
			{
				candidates := chain.ValidatorCandidates()
				fmt.Println(len(candidates))
				staked := big.NewInt(0)
				for i := range candidates {
					snapshot, _ := chain.ReadValidatorSnapshot(candidates[i])
					validator, _ := chain.ReadValidatorInformation(candidates[i])
					if !committee.IsEligibleForEPoSAuction(
						snapshot, validator,
					) {
						continue
					}
					staked = staked.Add(staked, validator.TotalDelegation())
				}
				return common.LeftPadBytes(staked.Bytes(), 32), nil
			}
		}
		return nil, errors.New("invalid ro staking message")
	}
}

func RoStakingGasFn(ref *block.Header, chain ChainContext) vm.RoStakingGasFunc {
	return func(db vm.StateDB, input []byte) (uint64, error) {
		if chain.ShardID() == shard.BeaconChainShardID {
			roStakeMsg, err := staking.ParseReadOnlyStakeMsg(input)
			if err == nil {
				switch roStakeMsg.What {
				case "DelegationByDelegatorAndValidator":
					return params.ValidatorInformationGasLoops, nil
				case "ValidatorMaxTotalDelegation":
					// no loops
					return params.ValidatorInformationGas, nil
				case "ValidatorTotalDelegation":
					return params.ValidatorInformationGasLoops, nil
				case "ValidatorCommissionRate":
					return params.ValidatorInformationGas, nil
				case "ValidatorStatus":
					return params.ValidatorInformationGas, nil
				case "TotalStakingSnapshot":
					// loop over each validator's delegations
					return params.ValidatorInformationGasLoops * uint64(len(chain.ValidatorCandidates())), nil
				case "MedianRawStakeSnapshot":
					// loop over each validator but more complex than just adding things up
					return params.ValidatorInformationGasLoopsComplex * uint64(len(chain.ValidatorCandidates())), nil
				case "BalanceAvailableForRedelegation":
					return params.DelegatorInformationGas, nil
				case "BalanceDelegatedByDelegator":
					return params.DelegatorInformationGas, nil
				}
			}
		} else {
			return params.TxGas, errors.New("not beacon shard")
		}
		return params.TxGas, errors.New("unknown operation")
	}
}
