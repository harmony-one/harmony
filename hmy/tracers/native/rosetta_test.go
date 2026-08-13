package native_test

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy/tracers"
	"github.com/harmony-one/harmony/hmy/tracers/native"
)

func TestRosettaBlockTracerRegistered(t *testing.T) {
	tracer, err := tracers.New("RosettaBlockTracer", new(tracers.Context), nil)
	if err != nil {
		t.Fatalf("RosettaBlockTracer lookup failed: %v", err)
	}
	rosettaTracer, ok := tracer.(*native.RosettaBlockTracer)
	if !ok {
		t.Fatalf("unexpected RosettaBlockTracer type: %T", tracer)
	}
	if _, ok := tracer.(vm.RosettaTracer); !ok {
		t.Fatalf("RosettaBlockTracer does not implement vm.RosettaTracer: %T", tracer)
	}

	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	value := big.NewInt(3)
	rosettaTracer.AddRosettaLog(
		vm.CALL,
		&vm.RosettaLogAddressItem{Account: &from},
		&vm.RosettaLogAddressItem{Account: &to},
		value,
	)
	result, err := rosettaTracer.GetRosettaResult()
	if err != nil {
		t.Fatalf("RosettaBlockTracer result failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("unexpected result length: got %d, want 1", len(result))
	}
	want := &tracers.RosettaLogItem{
		IsSuccess: true,
		OP:        vm.CALL,
		Depth:     []int{},
		From:      &vm.RosettaLogAddressItem{Account: &from},
		To:        &vm.RosettaLogAddressItem{Account: &to},
		Value:     value,
	}
	if !reflect.DeepEqual(result[0], want) {
		t.Fatalf("unexpected result: got %#v, want %#v", result[0], want)
	}
}

func TestRosettaBlockTracerRevertsSyntheticLogsWithTransaction(t *testing.T) {
	genericTracer, err := tracers.New("RosettaBlockTracer", new(tracers.Context), nil)
	if err != nil {
		t.Fatalf("RosettaBlockTracer lookup failed: %v", err)
	}
	tracer := genericTracer.(*native.RosettaBlockTracer)
	tracer.CaptureEnter(vm.CALL, common.Address{}, common.Address{}, nil, 0, new(big.Int))
	tracer.CaptureEnter(vm.CALL, common.Address{}, common.Address{}, nil, 0, new(big.Int))
	tracer.AddRosettaLog(vm.CALL, nil, nil, big.NewInt(1))
	tracer.CaptureExit(nil, 0, nil)
	tracer.CaptureExit(nil, 0, errors.New("execution reverted"))

	result, err := tracer.GetRosettaResult()
	if err != nil {
		t.Fatalf("RosettaBlockTracer result failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("unexpected result length: got %d, want 1", len(result))
	}
	if result[0].IsSuccess || !result[0].Reverted {
		t.Fatalf("synthetic log did not inherit transaction revert: %#v", result[0])
	}
}

func TestRosettaBlockTracerLimitsRevertToCurrentFrame(t *testing.T) {
	genericTracer, err := tracers.New("RosettaBlockTracer", new(tracers.Context), nil)
	if err != nil {
		t.Fatalf("RosettaBlockTracer lookup failed: %v", err)
	}
	tracer := genericTracer.(*native.RosettaBlockTracer)

	tracer.CaptureEnter(vm.CALL, common.Address{}, common.Address{}, nil, 0, new(big.Int))
	tracer.AddRosettaLog(vm.CALL, nil, nil, big.NewInt(1))
	tracer.CaptureExit(nil, 0, errors.New("execution reverted"))
	tracer.CaptureEnter(vm.CALL, common.Address{}, common.Address{}, nil, 0, new(big.Int))
	tracer.AddRosettaLog(vm.CALL, nil, nil, big.NewInt(2))
	tracer.CaptureExit(nil, 0, nil)

	result, err := tracer.GetRosettaResult()
	if err != nil {
		t.Fatalf("RosettaBlockTracer result failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("unexpected result length: got %d, want 2", len(result))
	}
	if result[0].IsSuccess || !result[0].Reverted {
		t.Fatalf("failed frame log was not reverted: %#v", result[0])
	}
	if !result[1].IsSuccess || result[1].Reverted {
		t.Fatalf("successful sibling log inherited revert: %#v", result[1])
	}
}
