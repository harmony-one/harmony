// Copyright 2017 The go-ethereum Authors
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
	"errors"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
)

var (
	ErrMalformedAATransaction = errors.New("AA transaction malformed")
)

func Validate(tx *types.Transaction, s types.Signer, evm *vm.EVM, gasLimit uint64) error {
	msg, err := tx.AsMessage(s)
	if err != nil {
		return err
	} else if !msg.IsAA() {
		return ErrMalformedAATransaction
	}

	evm.TxGasLimit = tx.GasLimit()
	if gasLimit > tx.GasLimit() {
		gasLimit = tx.GasLimit()
	}
	msg.SetGas(gasLimit)
	gp := new(GasPool).AddGas(gasLimit)

	evm.PaygasMode = vm.PaygasHalt
	_, err = ApplyMessage(evm, msg, gp)
	if err != nil {
		return err
	}
	tx.SetAAGasPrice(evm.GasPrice)
	return nil
}
