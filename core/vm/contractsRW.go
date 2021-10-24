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

package vm

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/accounts/abi"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/types"
	staking "github.com/harmony-one/harmony/staking/types"

	coreTypes "github.com/harmony-one/harmony/core/types"
)

var PrecompiledRWContractsEVMStake = map[common.Address]PrecompiledContractRW{
	common.BytesToAddress([]byte{1}):   nil, // nil means it assigned to PrecompiledContract
	common.BytesToAddress([]byte{2}):   nil,
	common.BytesToAddress([]byte{3}):   nil,
	common.BytesToAddress([]byte{4}):   nil,
	common.BytesToAddress([]byte{5}):   nil,
	common.BytesToAddress([]byte{6}):   nil,
	common.BytesToAddress([]byte{7}):   nil,
	common.BytesToAddress([]byte{8}):   nil,
	common.BytesToAddress([]byte{9}):   nil,
	common.BytesToAddress([]byte{255}): nil,

	common.BytesToAddress([]byte{252}): newEvmStake(),
	common.BytesToAddress([]byte{253}): nil,
	common.BytesToAddress([]byte{254}): nil,
}

// PrecompiledContractRW is the basic interface for native Go contracts.
// The difference between PrecompiledContractRW and PrecompiledContract is that
// PrecompiledContractRW is allowed to read/write StateDB.
type PrecompiledContractRW interface {
	RequiredGas(input []byte) uint64 // RequiredPrice calculates the contract gas use
	RunRW(evm *EVM, contract *Contract, input []byte, readOnly bool) ([]byte, error)
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
func RunPrecompiledContractRW(p PrecompiledContractRW, evm *EVM, contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	gas := p.RequiredGas(input)
	if !contract.UseGas(gas) {
		return nil, ErrOutOfGas
	}
	return p.RunRW(evm, contract, input, readOnly)
}

// ABI from core/vm/staking.sol
var stakingJsonABI = `[{"inputs":[{"components":[{"internalType":"address","name":"DelegatorAddress","type":"address"}],"internalType":"struct CollectRewards","name":"stkMsg","type":"tuple"}],"name":"collectRewards","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"ValidatorAddress","type":"address"},{"components":[{"internalType":"string","name":"Name","type":"string"},{"internalType":"string","name":"Identity","type":"string"},{"internalType":"string","name":"Website","type":"string"},{"internalType":"string","name":"SecurityContact","type":"string"},{"internalType":"string","name":"Details","type":"string"}],"internalType":"struct Description","name":"_Description","type":"tuple"},{"components":[{"internalType":"uint256","name":"Rate","type":"uint256"},{"internalType":"uint256","name":"MaxRate","type":"uint256"},{"internalType":"uint256","name":"MaxChangeRate","type":"uint256"}],"internalType":"struct CommissionRates","name":"_CommissionRates","type":"tuple"},{"internalType":"uint256","name":"MinSelfDelegation","type":"uint256"},{"internalType":"uint256","name":"MaxTotalDelegation","type":"uint256"},{"internalType":"bytes[]","name":"SlotPubKeys","type":"bytes[]"},{"internalType":"bytes[]","name":"SlotKeySigs","type":"bytes[]"},{"internalType":"uint256","name":"Amount","type":"uint256"}],"internalType":"struct CreateValidator","name":"stkMsg","type":"tuple"}],"name":"createValidator","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"DelegatorAddress","type":"address"},{"internalType":"address","name":"ValidatorAddress","type":"address"},{"internalType":"uint256","name":"Amount","type":"uint256"}],"internalType":"struct Delegate","name":"stkMsg","type":"tuple"}],"name":"delegate","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"ValidatorAddress","type":"address"},{"components":[{"internalType":"string","name":"Name","type":"string"},{"internalType":"string","name":"Identity","type":"string"},{"internalType":"string","name":"Website","type":"string"},{"internalType":"string","name":"SecurityContact","type":"string"},{"internalType":"string","name":"Details","type":"string"}],"internalType":"struct Description","name":"_Description","type":"tuple"},{"internalType":"uint256","name":"CommissionRate","type":"uint256"},{"internalType":"uint256","name":"MinSelfDelegation","type":"uint256"},{"internalType":"uint256","name":"MaxTotalDelegation","type":"uint256"},{"internalType":"bytes","name":"SlotKeyToRemove","type":"bytes"},{"internalType":"bytes","name":"SlotKeyToAdd","type":"bytes"},{"internalType":"bytes","name":"SlotKeyToAddSig","type":"bytes"},{"internalType":"bytes1","name":"EPOSStatus","type":"bytes1"}],"internalType":"struct EditValidator","name":"stkMsg","type":"tuple"}],"name":"editValidator","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"delegator","type":"address"}],"name":"getDelegationsByDelegator","outputs":[{"components":[{"internalType":"address","name":"Validator","type":"address"},{"components":[{"internalType":"address","name":"DelegatorAddress","type":"address"},{"internalType":"uint256","name":"Amount","type":"uint256"},{"internalType":"uint256","name":"Reward","type":"uint256"},{"components":[{"internalType":"uint256","name":"Amount","type":"uint256"},{"internalType":"uint256","name":"Epoch","type":"uint256"}],"internalType":"struct Undelegation[]","name":"Undelegations","type":"tuple[]"}],"internalType":"struct _Delegation","name":"Delegation","type":"tuple"}],"internalType":"struct Delegation[]","name":"","type":"tuple[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"components":[{"internalType":"address","name":"DelegatorAddress","type":"address"},{"internalType":"address","name":"ValidatorAddress","type":"address"},{"internalType":"uint256","name":"Amount","type":"uint256"}],"internalType":"struct Undelegate","name":"stkMsg","type":"tuple"}],"name":"undelegate","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

type Description struct {
	Name            string
	Identity        string
	Website         string
	SecurityContact string
	Details         string
}
type CommissionRates struct {
	// the commission rate charged to delegators, as a fraction
	Rate *big.Int
	// maximum commission rate which validator can ever charge, as a fraction
	MaxRate *big.Int
	// maximum increase of the validator commission every epoch, as a fraction
	MaxChangeRate *big.Int
}

type CreateValidator struct {
	ValidatorAddress common.Address
	Description
	CommissionRates
	MinSelfDelegation  *big.Int
	MaxTotalDelegation *big.Int
	SlotPubKeys        [][]byte
	SlotKeySigs        [][]byte
	Amount             *big.Int
}

type EditValidator struct {
	ValidatorAddress common.Address
	Description
	CommissionRate     *big.Int
	MinSelfDelegation  *big.Int
	MaxTotalDelegation *big.Int
	SlotKeyToRemove    []byte
	SlotKeyToAdd       []byte
	SlotKeyToAddSig    []byte
	EPOSStatus         byte
}

type Delegate types.Delegate
type CollectRewards types.CollectRewards
type Undelegate types.Undelegate

type Delegator common.Address
type Deletaion struct {
	Validator common.Address
	staking.Delegation
}
type Delegations []Deletaion

func (d Delegator) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	return types.DirectiveDelegate, nil, errors.New("not stake msg")
}

func (d Delegate) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	return types.DirectiveDelegate, types.Delegate(d), nil
}

func (c CollectRewards) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	return types.DirectiveCollectRewards, types.CollectRewards(c), nil
}

func (u Undelegate) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	return types.DirectiveUndelegate, types.Undelegate(u), nil
}

func (e EditValidator) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	typ := types.DirectiveEditValidator
	var CommissionRate = numeric.NewDecFromBigInt(e.CommissionRate)
	var SlotKeyToRemove bls.SerializedPublicKey
	var SlotKeyToAdd bls.SerializedPublicKey
	var SlotKeyToAddSig bls.SerializedSignature
	if copy(SlotKeyToRemove[:], e.SlotKeyToRemove) != len(SlotKeyToRemove) {
		return typ, nil, errors.New("invalid BLS Public key (SlotKeyToRemove)")
	}
	if copy(SlotKeyToAdd[:], e.SlotKeyToAdd) != len(SlotKeyToAdd) {
		return typ, nil, errors.New("invalid BLS Public key (SlotKeyToAdd)")
	}
	if copy(SlotKeyToAddSig[:], e.SlotKeyToAddSig) != len(SlotKeyToAddSig) {
		return typ, nil, errors.New("invalid BLS Public key (SlotKeyToAddSig)")
	}
	return types.DirectiveEditValidator, types.EditValidator{
		ValidatorAddress:   e.ValidatorAddress,
		Description:        types.Description(e.Description),
		CommissionRate:     &CommissionRate,
		MinSelfDelegation:  e.MinSelfDelegation,
		MaxTotalDelegation: e.MaxTotalDelegation,
		SlotKeyToRemove:    &SlotKeyToRemove,
		SlotKeyToAdd:       &SlotKeyToAdd,
		SlotKeyToAddSig:    &SlotKeyToAddSig,
		EPOSStatus:         effective.Eligibility(e.EPOSStatus),
	}, nil
}
func (c CreateValidator) toNativeStakeMsg() (types.Directive, types.StakeMsg, error) {
	typ := types.DirectiveCreateValidator
	SlotPubKeys := make([]bls.SerializedPublicKey, len(c.SlotPubKeys))
	SlotKeySigs := make([]bls.SerializedSignature, len(c.SlotKeySigs))
	for i, src := range c.SlotPubKeys {
		dst := SlotPubKeys[i][:]
		if copy(dst, src) != len(dst) {
			return typ, nil, errors.New("invalid BLS Public key")
		}
	}
	for i, src := range c.SlotKeySigs {
		dst := SlotKeySigs[i][:]
		if copy(dst, src) != len(dst) {
			return typ, nil, errors.New("invalid BLS signature")
		}
	}
	return types.DirectiveCreateValidator, types.CreateValidator{
		ValidatorAddress: c.ValidatorAddress,
		Description:      types.Description(c.Description),
		CommissionRates: types.CommissionRates{
			Rate:          numeric.NewDecFromBigInt(c.CommissionRates.Rate),
			MaxRate:       numeric.NewDecFromBigInt(c.CommissionRates.MaxRate),
			MaxChangeRate: numeric.NewDecFromBigInt(c.CommissionRates.MaxChangeRate),
		},
		MinSelfDelegation:  c.MinSelfDelegation,
		MaxTotalDelegation: c.MaxTotalDelegation,
		SlotPubKeys:        SlotPubKeys,
		SlotKeySigs:        SlotKeySigs,
		Amount:             c.Amount,
	}, nil
}

type solStakeMsg interface {
	toNativeStakeMsg() (types.Directive, types.StakeMsg, error)
}

type solStakeMsgAlloctor func() solStakeMsg

func emptyCreateValidator() solStakeMsg {
	return &CreateValidator{}
}
func emptyEditValidator() solStakeMsg {
	return &EditValidator{}
}
func emptyDelegate() solStakeMsg {
	return &Delegate{}
}
func emptyCollectRewards() solStakeMsg {
	return &CollectRewards{}
}
func emptyUndelegate() solStakeMsg {
	return &Undelegate{}
}

func emptyDelegator() solStakeMsg {
	return &Delegator{}
}

type methodID [4]byte
type convertor struct {
	solABI abi.Method
	goABI  struct {
		input solStakeMsgAlloctor
	}
	readOnly bool
}

func (c convertor) toGoStruct(data []byte) (solStakeMsg, error) {
	solStkMsg := c.goABI.input()
	args := c.solABI.Inputs // refers to accounts/abi/abi.go:UnpackIntoInterface(...)
	unpacked, err := args.Unpack(data)
	if err != nil {
		return nil, err
	}
	if err := args.Copy(solStkMsg, unpacked); err != nil {
		return nil, err
	}
	return solStkMsg, nil
}

// convert solidity calldata to go struct
func (c convertor) toGoStakeMsg(data []byte) (types.Directive, types.StakeMsg, error) {
	solStkMsg, err := c.toGoStruct(data)
	if err != nil {
		return types.DirectiveCreateValidator, nil, err
	}
	return solStkMsg.toNativeStakeMsg()
}

// convert go structs to solidity return data
func (c convertor) toSol(args ...interface{}) ([]byte, error) {
	return c.solABI.Outputs.Pack(args...)
}

// EVMStake provides a sets of APIs of staking.
type evmStake struct {
	convertors map[methodID]convertor
}

func stakeViewFunc(evm *EVM, param interface{}) (interface{}, error) {
	switch v := param.(type) {
	case Delegator:
		validators, delegations, err := evm.GetDelegationsByDelegator(evm.StateDB, common.Address(v))
		if err != nil {
			return nil, err
		}
		ret := make(Delegations, len(validators))
		for i, validator := range validators {
			ret[i].Validator = validator
			ret[i].Delegation = *delegations[i]
		}
		return ret, nil
	default:
		return nil, errors.New("unknow type")
	}
}

func newEvmStake() *evmStake {
	_abi, err := abi.JSON(strings.NewReader(stakingJsonABI))
	if err != nil {
		panic("invalid staking ABI") // abi must be valid
	}
	es := &evmStake{
		convertors: make(map[methodID]convertor, 5),
	}
	addConvertor := func(method string, input solStakeMsgAlloctor, readOnly bool) {
		var id methodID
		var convertor convertor
		solMethodABI, exist := _abi.Methods[method]
		if !exist {
			panic(fmt.Sprintf("%s is not included in staking ABI", method)) // abi must be valid
		}
		copy(id[:], solMethodABI.ID)
		convertor.solABI = solMethodABI
		convertor.goABI.input = input
		es.convertors[id] = convertor
	}
	addConvertor("createValidator", emptyCreateValidator, false)
	addConvertor("editValidator", emptyEditValidator, false)
	addConvertor("delegate", emptyDelegate, false)
	addConvertor("undelegate", emptyUndelegate, false)
	addConvertor("collectRewards", emptyCollectRewards, false)
	addConvertor("getDelegationsByDelegator", emptyDelegator, true)
	return es
}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *evmStake) RequiredGas(input []byte) uint64 {
	return 0
}

func (c *evmStake) RunRW(evm *EVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	var id methodID
	copy(id[:], input)
	convertor, exist := c.convertors[id]
	if !exist {
		return nil, errors.New("invalid staking api")
	}
	if !convertor.readOnly && readOnly {
		return nil, errWriteProtection
	}
	endata := input[4:]
	if convertor.readOnly {
		arg, err := convertor.toGoStruct(endata)
		if err != nil {
			return nil, err
		}
		response, err := stakeViewFunc(evm, arg)
		if err != nil {
			return nil, err
		}
		return convertor.toSol(response)
	}
	typ, stkMsg, err := convertor.toGoStakeMsg(endata)
	if err != nil {
		return nil, err
	}
	payload, err := rlp.EncodeToBytes(stkMsg)
	if err != nil {
		return nil, err
	}
	err = evm.Stake(contract, coreTypes.StakingTypeMap[typ], payload)
	if err != nil {
		return nil, err
	}
	return convertor.toSol()
}
