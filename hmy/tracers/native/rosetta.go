package native

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy/tracers"
)

func init() {
	register("RosettaBlockTracer", newRosettaTracer)
}

func newRosettaTracer(_ *tracers.Context, _ json.RawMessage) (tracers.Tracer, error) {
	return &RosettaBlockTracer{ParityBlockTracer: &ParityBlockTracer{}}, nil
}

// RosettaBlockTracer converts Parity-style call traces and Harmony staking
// balance movements into the operation format consumed by the Rosetta service.
type RosettaBlockTracer struct {
	*ParityBlockTracer

	logs       []*tracers.RosettaLogItem
	frameStart []int
}

func (rbt *RosettaBlockTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	rbt.logs = nil
	rbt.frameStart = []int{0}
	rbt.ParityBlockTracer.CaptureStart(env, from, to, create, input, gas, value)
}

func (rbt *RosettaBlockTracer) CaptureEnd(output []byte, gasUsed uint64, duration time.Duration, err error) {
	rbt.markCurrentFrame(err)
	rbt.ParityBlockTracer.CaptureEnd(output, gasUsed, duration, err)
}

func (rbt *RosettaBlockTracer) CaptureEnter(_ vm.OpCode, _ common.Address, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
	rbt.frameStart = append(rbt.frameStart, len(rbt.logs))
}

func (rbt *RosettaBlockTracer) CaptureExit(_ []byte, _ uint64, err error) {
	rbt.markCurrentFrame(err)
}

func (rbt *RosettaBlockTracer) markCurrentFrame(err error) {
	if len(rbt.frameStart) == 0 {
		return
	}
	last := len(rbt.frameStart) - 1
	start := rbt.frameStart[last]
	rbt.frameStart = rbt.frameStart[:last]
	if err == nil {
		return
	}
	for _, log := range rbt.logs[start:] {
		log.IsSuccess = false
		log.Reverted = true
	}
}

func (rbt *RosettaBlockTracer) formatAction(depth []int, parentErr error, ac *action) *tracers.RosettaLogItem {
	value := ac.value
	if value == nil {
		value = new(big.Int)
	}
	return &tracers.RosettaLogItem{
		IsSuccess: ac.err == nil,
		Reverted:  parentErr != nil || ac.err != nil,
		OP:        ac.op,
		Depth:     depth,
		From:      &vm.RosettaLogAddressItem{Account: &ac.from},
		To:        &vm.RosettaLogAddressItem{Account: &ac.to},
		Value:     value,
	}
}

// AddRosettaLog records Harmony-specific balance movements that are not EVM
// calls, such as staking operations.
func (rbt *RosettaBlockTracer) AddRosettaLog(op vm.OpCode, from, to *vm.RosettaLogAddressItem, value *big.Int) {
	if value == nil {
		value = new(big.Int)
	} else {
		value = new(big.Int).Set(value)
	}
	rbt.logs = append(rbt.logs, &tracers.RosettaLogItem{
		IsSuccess: true,
		OP:        op,
		Depth:     []int{},
		From:      from,
		To:        to,
		Value:     value,
	})
}

// GetRosettaResult returns the typed trace used internally by the Rosetta API.
func (rbt *RosettaBlockTracer) GetRosettaResult() ([]*tracers.RosettaLogItem, error) {
	results := make([]*tracers.RosettaLogItem, 0, len(rbt.logs))
	if rbt.cur != nil {
		root := &rbt.cur.action
		var finalize func(*action, error, []int)
		finalize = func(ac *action, parentErr error, depth []int) {
			results = append(results, rbt.formatAction(depth, parentErr, ac))
			nextErr := parentErr
			if ac.err != nil {
				nextErr = ac.err
			}
			for i, subAction := range ac.subCalls {
				subDepth := append(append([]int(nil), depth...), i)
				finalize(subAction, nextErr, subDepth)
			}
		}
		for i, subAction := range root.subCalls {
			finalize(subAction, root.err, []int{i})
		}
	}
	return append(results, rbt.logs...), nil
}

// GetResult implements tracers.Tracer. Rosetta callers use GetRosettaResult to
// retain the typed internal representation.
func (rbt *RosettaBlockTracer) GetResult() (json.RawMessage, error) {
	result, err := rbt.GetRosettaResult()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
