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

package tracers

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/vm"
	"math/big"
)

type RosettaLogItem struct {
	IsSuccess bool
	Reverted  bool
	OP        vm.OpCode
	Depth     []int
	From      common.Address
	To        common.Address
	Value     *big.Int
}

type RosettaBlockTracer struct {
	*ParityBlockTracer
}

func (rbt *RosettaBlockTracer) formatAction(depth []int, parentErr error, ac *action) *RosettaLogItem {
	val := ac.value
	if val == nil {
		val = big.NewInt(0)
	}

	return &RosettaLogItem{
		IsSuccess: ac.err == nil,
		Reverted:  !(parentErr == nil && ac.err == nil),
		OP:        ac.op,
		Depth:     depth,
		From:      ac.from,
		To:        ac.to,
		Value:     val,
	}
}

func (rbt *RosettaBlockTracer) GetResult() ([]*RosettaLogItem, error) {
	root := &rbt.action

	for len(rbt.calls) > 1 {
		call := rbt.pop()
		if call.op == vm.CREATE || call.op == vm.CREATE2 {
			call.gasUsed = call.gasIn - call.gasCost
		} else {
			if call.gas != 0 {
				call.gasUsed = call.gasIn - call.gasCost + call.gas
			}
		}
		rbt.last().push(call)
	}

	var results = make([]*RosettaLogItem, 0)
	var err error
	var finalize func(ac *action, parentErr error, traceAddress []int)
	finalize = func(ac *action, parentErr error, traceAddress []int) {
		results = append(results, rbt.formatAction(traceAddress, parentErr, ac))
		nextErr := parentErr
		if ac.err != nil {
			nextErr = ac.err
		}

		for i, subAc := range ac.subCalls {
			finalize(subAc, nextErr, append(traceAddress[:], i))
		}
	}

	traceAddress := make([]int, 0)
	for i, subAc := range root.subCalls {
		finalize(subAc, root.err, append(traceAddress[:], i))
	}

	return results, err
}
