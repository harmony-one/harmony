package audit

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// fcAddress is the only write-capable staking precompile (§2.3).
var fcAddress = common.BytesToAddress([]byte{252})

// FCOp is one recorded call frame targeting 0xfc: the complete precompile
// inventory including Undelegate/CollectRewards, sufficient to classify
// reverted and top-up operations.
type FCOp struct {
	Block     uint64 `json:"block"`
	TxOrdinal int    `json:"tx_ordinal"` // message application order within the block
	Depth     int    `json:"depth"`
	Caller    string `json:"caller"`
	Kind      string `json:"kind"` // Delegate | Undelegate | CollectRewards | unparseable
	Delegator string `json:"delegator,omitempty"`
	Validator string `json:"validator,omitempty"`
	Amount    string `json:"amount,omitempty"`
	// FrameFailed: the 0xfc frame itself returned an error.
	FrameFailed bool `json:"frame_failed,omitempty"`
	// EnclosingReverted: an ancestor frame exited with an error after this
	// call (its state effect rolled back; StakeMsgs entries survive).
	EnclosingReverted bool `json:"enclosing_reverted,omitempty"`
}

// fcTracer implements vm.EVMLogger, recording every call frame that
// targets 0xfc with caller, ParseStakeMsg-decoded input, depth, frame
// success and enclosing-revert status.
type fcTracer struct {
	block     uint64
	txOrdinal int

	// frame stack: frame ids of currently open frames.
	stack   []int
	nextID  int
	ops     []FCOp
	opPaths [][]int // ops[i]'s enclosing frame-id path (excluding the fc frame itself)

	// open fc frames by frame id -> ops index (to set FrameFailed on exit).
	openFC map[int]int
}

var _ vm.EVMLogger = (*fcTracer)(nil)

func newFCTracer() *fcTracer { return &fcTracer{openFC: map[int]int{}} }

// BeginBlock resets per-block counters (the tracer is reused per block).
func (t *fcTracer) BeginBlock(number uint64) {
	t.block = number
	t.txOrdinal = -1
	t.stack = t.stack[:0]
}

// Ops returns the recorded inventory.
func (t *fcTracer) Ops() []FCOp { return t.ops }

func (t *fcTracer) CaptureTxStart(gasLimit uint64) {
	t.txOrdinal++
	t.stack = t.stack[:0]
}

func (t *fcTracer) CaptureTxEnd(restGas uint64) {}

func (t *fcTracer) push() int {
	id := t.nextID
	t.nextID++
	t.stack = append(t.stack, id)
	return id
}

func (t *fcTracer) pop(err error) {
	if len(t.stack) == 0 {
		return
	}
	id := t.stack[len(t.stack)-1]
	t.stack = t.stack[:len(t.stack)-1]
	if opIdx, ok := t.openFC[id]; ok {
		delete(t.openFC, id)
		if err != nil {
			t.ops[opIdx].FrameFailed = true
		}
	}
	if err != nil {
		// Every recorded op whose path contains this frame had its state
		// effect rolled back.
		for i, path := range t.opPaths {
			for _, fid := range path {
				if fid == id {
					t.ops[i].EnclosingReverted = true
					break
				}
			}
		}
	}
}

func (t *fcTracer) record(from, to common.Address, input []byte) {
	if to != fcAddress {
		return
	}
	op := FCOp{
		Block:     t.block,
		TxOrdinal: t.txOrdinal,
		Depth:     len(t.stack), // depth of the fc frame (stack includes it)
		Caller:    from.Hex(),
		Kind:      "unparseable",
	}
	msg, err := staking.ParseStakeMsg(from, input)
	if err == nil {
		switch m := msg.(type) {
		case *stakingTypes.Delegate:
			op.Kind = "Delegate"
			op.Delegator = m.DelegatorAddress.Hex()
			op.Validator = m.ValidatorAddress.Hex()
			op.Amount = bigString(m.Amount)
		case *stakingTypes.Undelegate:
			op.Kind = "Undelegate"
			op.Delegator = m.DelegatorAddress.Hex()
			op.Validator = m.ValidatorAddress.Hex()
			op.Amount = bigString(m.Amount)
		case *stakingTypes.CollectRewards:
			op.Kind = "CollectRewards"
			op.Delegator = m.DelegatorAddress.Hex()
		default:
			op.Kind = fmt.Sprintf("unexpected:%T", msg)
		}
	}
	// The enclosing path excludes the fc frame itself (top of stack).
	path := append([]int(nil), t.stack[:len(t.stack)-1]...)
	t.ops = append(t.ops, op)
	t.opPaths = append(t.opPaths, path)
	t.openFC[t.stack[len(t.stack)-1]] = len(t.ops) - 1
}

func (t *fcTracer) CaptureStart(env *vm.EVM, from, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	id := t.push()
	_ = id
	if !create {
		t.record(from, to, input)
	}
}

func (t *fcTracer) CaptureEnd(output []byte, gasUsed uint64, d time.Duration, err error) {
	t.pop(err)
}

func (t *fcTracer) CaptureEnter(typ vm.OpCode, from, to common.Address, input []byte, gas uint64, value *big.Int) {
	t.push()
	t.record(from, to, input)
}

func (t *fcTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	t.pop(err)
}

func (t *fcTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
}

func (t *fcTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
}

func bigString(b *big.Int) string {
	if b == nil {
		return ""
	}
	return b.String()
}
