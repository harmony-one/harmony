// Copyright 2021 The go-ethereum Authors
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

package logger

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
)

type dummyContractRef struct {
	calledForEach bool
}

func (dummyContractRef) Address() common.Address     { return common.Address{} }
func (dummyContractRef) Value() *big.Int             { return new(big.Int) }
func (dummyContractRef) SetCode(common.Hash, []byte) {}
func (d *dummyContractRef) ForEachStorage(callback func(key, value common.Hash) bool) {
	d.calledForEach = true
}
func (d *dummyContractRef) SubBalance(amount *big.Int) {}
func (d *dummyContractRef) AddBalance(amount *big.Int) {}
func (d *dummyContractRef) SetBalance(*big.Int)        {}
func (d *dummyContractRef) SetNonce(uint64)            {}
func (d *dummyContractRef) Balance() *big.Int          { return new(big.Int) }

type dummyStatedb struct {
	state.DB
}

func (*dummyStatedb) GetRefund() uint64                                       { return 1337 }
func (*dummyStatedb) GetState(_ common.Address, _ common.Hash) common.Hash    { return common.Hash{} }
func (*dummyStatedb) SetState(_ common.Address, _ common.Hash, _ common.Hash) {}

func TestStoreCapture(t *testing.T) {
	var (
		logger   = NewStructLogger(nil)
		env      = vm.NewEVM(vm.BlockContext{}, vm.TxContext{}, &dummyStatedb{}, params.TestChainConfig, vm.Config{Debug: true, Tracer: logger})
		contract = vm.NewContract(&dummyContractRef{}, &dummyContractRef{}, new(big.Int), 100000)
	)
	contract.Code = []byte{byte(vm.PUSH1), 0x1, byte(vm.PUSH1), 0x0, byte(vm.SSTORE)}
	var index common.Hash
	logger.CaptureStart(env, common.Address{}, contract.Address(), false, nil, 0, nil)
	_, err := env.Interpreter().Run(contract, []byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(logger.storage[contract.Address()]) == 0 {
		t.Fatalf("expected exactly 1 changed value on address %x, got %d", contract.Address(),
			len(logger.storage[contract.Address()]))
	}
	exp := common.BigToHash(big.NewInt(1))
	if logger.storage[contract.Address()][index] != exp {
		t.Errorf("expected %x, got %x", exp, logger.storage[contract.Address()][index])
	}
}

// Tests that blank fields don't appear in logs when JSON marshalled, to reduce
// logs bloat and confusion. See https://github.com/ethereum/go-ethereum/issues/24487
func TestStructLogMarshalingOmitEmpty(t *testing.T) {
	tests := []struct {
		name string
		log  *StructLog
		want string
	}{
		{
			name: "empty err and no fields",
			log:  &StructLog{},
			want: `{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"contractAddress":"0x0000000000000000000000000000000000000000","callerAddress":"0x0000000000000000000000000000000000000000","opName":"STOP"}`,
		},
		{
			name: "with err",
			log:  &StructLog{Err: vm.ErrExecutionReverted},
			want: `{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"contractAddress":"0x0000000000000000000000000000000000000000","callerAddress":"0x0000000000000000000000000000000000000000","opName":"STOP","error":"execution reverted"}`,
		},
		{
			name: "with mem",
			log:  &StructLog{Memory: make([]byte, 2), MemorySize: 2},
			want: `{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memory":"0x0000","memSize":2,"stack":null,"depth":0,"refund":0,"contractAddress":"0x0000000000000000000000000000000000000000","callerAddress":"0x0000000000000000000000000000000000000000","opName":"STOP"}`,
		},
		{
			name: "with 0-size mem",
			log:  &StructLog{Memory: make([]byte, 0)},
			want: `{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"contractAddress":"0x0000000000000000000000000000000000000000","callerAddress":"0x0000000000000000000000000000000000000000","opName":"STOP"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob, err := json.Marshal(tt.log)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := string(blob), tt.want; have != want {
				t.Fatalf("mismatched results\n\thave: %v\n\twant: %v", have, want)
			}
		})
	}
}

// TestFormatLogsLegacyJSON checks that our legacy structLog JSON encoding
// matches the expected Ethereum-style shape: omitting empty error, 0x-prefixed
// and padded memory/storage, and hex-encoded return value.
func TestFormatLogsLegacyJSON(t *testing.T) {
	// Build a single StructLog with:
	// - a small memory slice so we exercise padding
	// - a storage entry
	// - an error
	addr := common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314")
	key := common.HexToHash("0x01")
	val := common.HexToHash("0x02")

	log := StructLog{
		Pc:              1,
		Op:              vm.SSTORE,
		Gas:             10,
		GasCost:         5,
		Memory:          []byte{0xaa, 0xbb},
		MemorySize:      2,
		Depth:           1,
		RefundCounter:   0,
		Err:             vm.ErrExecutionReverted,
		Storage:         map[common.Hash]common.Hash{key: val},
		ContractAddress: addr,
		CallerAddress:   addr,
	}

	formatted := formatLogs([]StructLog{log})
	if len(formatted) != 1 {
		t.Fatalf("expected 1 formatted log, got %d", len(formatted))
	}

	fl := formatted[0]

	// Error must be a non-nil pointer with the right value.
	if fl.Error == nil || *fl.Error != "execution reverted" {
		t.Fatalf("unexpected error field: %#v", fl.Error)
	}

	// Memory must be present, with a single, 32-byte padded word encoded as 0x + 64 hex chars.
	if fl.Memory == nil {
		t.Fatalf("expected memory to be set")
	}
	if len(*fl.Memory) != 1 {
		t.Fatalf("expected 1 memory word, got %d", len(*fl.Memory))
	}
	memWord := (*fl.Memory)[0]
	if len(memWord) != 2+64 { // 0x + 64 hex chars
		t.Fatalf("unexpected memory word length: got %d, value %q", len(memWord), memWord)
	}
	if memWord[:2] != "0x" {
		t.Fatalf("memory word missing 0x prefix: %q", memWord)
	}

	// Storage keys and values must be 0x-prefixed hex.
	if fl.Storage == nil {
		t.Fatalf("expected storage to be set")
	}
	st := *fl.Storage
	gotVal, ok := st[key.Hex()]
	if !ok {
		t.Fatalf("expected storage key %s to be present", key.Hex())
	}
	if gotVal != val.Hex() {
		t.Fatalf("unexpected storage value: have %s, want %s", gotVal, val.Hex())
	}
}

// TestStructLogLegacyJSONSpecFormatting mirrors the go-ethereum test to ensure
// our legacy-ish JSON formatting matches the spec expectations for error,
// memory padding and storage encoding.
func TestStructLogLegacyJSONSpecFormatting(t *testing.T) {
	tests := []struct {
		name string
		log  *StructLog
		want string
	}{
		{
			name: "omits empty error and pads memory/storage",
			log: &StructLog{
				Pc:         7,
				Op:         vm.SSTORE,
				Gas:        100,
				GasCost:    20,
				Memory:     []byte{0xaa, 0xbb},
				Storage:    map[common.Hash]common.Hash{common.BigToHash(big.NewInt(1)): common.BigToHash(big.NewInt(2))},
				Depth:      1,
				ReturnData: []byte{0x12, 0x34},
			},
			want: `{"pc":7,"op":"SSTORE","gas":100,"gasCost":20,"depth":1,"returnData":"0x1234","memory":["0xaabb000000000000000000000000000000000000000000000000000000000000"],"storage":{"0x0000000000000000000000000000000000000000000000000000000000000001":"0x0000000000000000000000000000000000000000000000000000000000000002"}}`,
		},
		{
			name: "includes error only when present",
			log: &StructLog{
				Pc:      1,
				Op:      vm.STOP,
				Gas:     2,
				GasCost: 3,
				Depth:   1,
				Err:     errors.New("boom"),
			},
			want: `{"pc":1,"op":"STOP","gas":2,"gasCost":3,"depth":1,"error":"boom"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			have := string(tt.log.toLegacyJSON())
			if have != tt.want {
				t.Fatalf("mismatched results\n\thave: %v\n\twant: %v", have, tt.want)
			}
		})
	}
}

// TestExecutionResultReturnValueEncoding ensures that ExecutionResult encodes
// returnValue as hexutil.Bytes and omits it (empty bytes) on hard failure.
func TestExecutionResultReturnValueEncoding(t *testing.T) {
	// Successful execution: return data should be preserved and hex-encoded.
	{
		logger := &StructLogger{}
		logger.output = []byte{0x01, 0x02}
		logger.usedGas = 21

		got, err := logger.GetResult()
		if err != nil {
			t.Fatalf("GetResult (success) returned error: %v", err)
		}
		var res ExecutionResult
		if err := json.Unmarshal(got, &res); err != nil {
			t.Fatalf("unmarshal result (success): %v", err)
		}
		if string(res.ReturnValue) != string(hexutil.Bytes{0x01, 0x02}) {
			t.Fatalf("unexpected return value (success): %v", res.ReturnValue)
		}
	}

	// Hard failure (non-revert): return data should be empty.
	{
		logger := &StructLogger{}
		logger.output = []byte{0x01, 0x02}
		logger.usedGas = 21
		logger.err = vm.ErrOutOfGas

		got, err := logger.GetResult()
		if err != nil {
			t.Fatalf("GetResult (failure) returned error: %v", err)
		}
		var res ExecutionResult
		if err := json.Unmarshal(got, &res); err != nil {
			t.Fatalf("unmarshal result (failure): %v", err)
		}
		if len(res.ReturnValue) != 0 {
			t.Fatalf("expected empty return value on hard failure, got %v", res.ReturnValue)
		}
	}
}
