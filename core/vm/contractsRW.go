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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/staking/types"

	coreTypes "github.com/harmony-one/harmony/core/types"
)

var PrecompiledRWContractsEVMStake = map[common.Address]PrecompiledContractRW{
	common.BytesToAddress([]byte{252}): &evmStake{},
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

func newCreateValidator(input []byte) (types.Directive, types.StakeMsg) {
	return types.DirectiveCreateValidator, types.CreateValidator{}
}

// EVMStake provides a sets of APIs of staking.
type evmStake struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *evmStake) RequiredGas(input []byte) uint64 {
	return 0
}

func (c *evmStake) RunRW(evm *EVM, contract *Contract, input []byte, readOnly bool) ([]byte, error) {
	typ, stkMsg := newCreateValidator(input)
	payload, _ := rlp.EncodeToBytes(stkMsg)
	err := evm.Stake(contract, coreTypes.StakingTypeMap[typ], payload)
	return nil, err
}

// be compatible with PrecompiledContract
func (c *evmStake) Run(input []byte) ([]byte, error) {
	return nil, nil
}
