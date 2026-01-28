package vm

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// SimpleTracer
type SimpleTracer struct {
	Block uint64
}

func (t *SimpleTracer) CaptureTxStart(gasLimit uint64) {

}

func (t *SimpleTracer) CaptureTxEnd(restGas uint64) {

}

func (t *SimpleTracer) CaptureStart(env *EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {

}

func (t *SimpleTracer) CaptureEnd(output []byte, gasUsed uint64, _ time.Duration, err error) {
}

func (t *SimpleTracer) CaptureEnter(typ OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {

}

func (t *SimpleTracer) CaptureExit(output []byte, gasUsed uint64, err error) {

}

// CaptureState
func (t *SimpleTracer) CaptureState(evm *EVM, pc uint64, op OpCode, gas uint64, cost uint64, scope *ScopeContext, ret []byte, depth int, err error) {
	if evm.Context.BlockNumber.Uint64() != t.Block {
		return
	}
	var stackLen int
	if scope != nil && scope.Stack != nil {
		stackLen = scope.Stack.len()
	}
	fmt.Printf("vm_trace(%d) pc=%d op=%v gas=%d cost=%d stack_len=%d depth=%d ret_len=%d err=%v\n",
		evm.Context.BlockNumber.Uint64(), pc, op, gas, cost, stackLen, depth, len(ret), err)
}

// CaptureFault
func (t *SimpleTracer) CaptureFault(pc uint64, op OpCode, gas uint64, cost uint64, scope *ScopeContext, depth int, err error) {

}
