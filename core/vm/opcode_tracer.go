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

//func (t *SimpleTracer) CaptureTxEnd(restGas uint64) {
//
//}

func (t *SimpleTracer) CaptureStart(env *EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
	return nil
}

func (t *SimpleTracer) CaptureEnd(output []byte, gasUsed uint64, _ time.Duration, err error) error {
	return nil
}

//func (t *SimpleTracer) CaptureEnter(typ OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
//
//}

//func (t *SimpleTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
//}

// CaptureState
func (t *SimpleTracer) CaptureState(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, contract *Contract, depth int, err error) (HookAfter, error) {
	if env.Context.BlockNumber.Uint64() != t.Block {
		return nil, nil
	}
	var stackLen int
	if stack != nil {
		stackLen = stack.len()
	}
	memoryLen := int(memory.Len()) / 32
	fmt.Printf("vm_trace(%d) pc=%d op=%v gas=%d cost=%d stack_len=%d memory_len=%d depth=%d err=%v\n",
		env.Context.BlockNumber.Uint64(), pc, op, gas, cost, stackLen, memoryLen, depth, err)
	// print stack, memory

	fmt.Printf("stack: ")
	for i := 0; i < stackLen; i++ {
		fmt.Printf("%s, ", stack.data[stackLen-1-i].String())
	}
	fmt.Printf("\nmemory: ")
	for i := 0; i < memoryLen; i++ {
		fmt.Printf("%s, ", new(big.Int).SetBytes(memory.GetPtr(int64(i*32), 32)).String())
	}
	fmt.Println()

	return nil, nil
}

// CaptureFault
func (t *SimpleTracer) CaptureFault(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, stack *Stack, contract *Contract, depth int, err error) error {
	return nil
}
