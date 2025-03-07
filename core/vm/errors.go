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

import "errors"

// List evm execution errors
var (
	// ErrInvalidSubroutineEntry means that a BEGINSUB was reached via iteration,
	// as opposed to from a JUMPSUB instruction
	ErrInvalidSubroutineEntry   = errors.New("invalid subroutine entry")
	ErrOutOfGas                 = errors.New("out of gas")
	ErrCodeStoreOutOfGas        = errors.New("contract creation code storage out of gas")
	ErrDepth                    = errors.New("max call depth exceeded")
	ErrInsufficientBalance      = errors.New("insufficient balance for transfer")
	ErrContractAddressCollision = errors.New("contract address collision")
	ErrExecutionReverted        = errors.New("execution reverted")
	ErrMaxCodeSizeExceeded      = errors.New("max code size exceeded")
	ErrInvalidJump              = errors.New("invalid jump destination")
	ErrWriteProtection          = errors.New("write protection")
	ErrReturnDataOutOfBounds    = errors.New("return data out of bounds")
	ErrGasUintOverflow          = errors.New("gas uint64 overflow")
	ErrInvalidRetsub            = errors.New("invalid retsub")
	ErrReturnStackExceeded      = errors.New("return stack limit reached")
)
