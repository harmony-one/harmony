package p2p

import (
	"context"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/internal/herrors"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func TestValidateAction_Compare(t *testing.T) {
	tests := []struct {
		v1, v2 ValidateAction
		expRes int
	}{
		{MsgAccept, MsgIgnore, -1},
		{MsgIgnore, MsgIgnore, 0},
		{MsgReject, MsgIgnore, 1},
		{ValidateAction(100), MsgAccept, -1},
		{MsgAccept, ValidateAction(100), 1},
	}
	for i, test := range tests {
		res := test.v1.Compare(test.v2)
		if res != test.expRes {
			t.Errorf("Test %v: Unexpected result: %v / %v", i, res, test.expRes)
		}
	}
}

func TestMergeValidateResult(t *testing.T) {
	tests := []struct {
		vrs []ValidateResult
		exp ValidateResult
	}{
		{
			vrs: []ValidateResult{
				{Reason: "accepted1", Action: MsgAccept},
				{Reason: "rejected1", Action: MsgReject},
				{Reason: "ignored1", Action: MsgIgnore},
			},
			exp: ValidateResult{
				Reason: "accepted: accepted1; rejected: rejected1; ignored: ignored1",
				Action: MsgReject,
			},
		},
		{
			vrs: []ValidateResult{
				{Reason: "ignored1", Action: MsgIgnore},
				{Reason: "accepted1", Action: MsgAccept},
			},
			exp: ValidateResult{
				Reason: "ignored: ignored1; accepted: accepted1",
				Action: MsgIgnore,
			},
		},
		{
			vrs: []ValidateResult{
				{Reason: "", Action: MsgAccept},
				{Reason: "", Action: MsgIgnore},
			},
			exp: ValidateResult{
				Reason: "",
				Action: MsgIgnore,
			},
		},
	}
	for i, test := range tests {
		res := mergeValidateResults(test.vrs)
		if !reflect.DeepEqual(res, test.exp) {
			t.Errorf("Test %v: Unexpected result \n\t[%+v]\n\t[%+v]", i, res, test.exp)
		}
	}
}

type (
	testStruct struct {
		field1 int
		field2 string
	}
)

var (
	testKey1 = "key1"
	testKey2 = "key2"

	testSpec1 = "spec1"
	testSpec2 = "spec2"

	testVal1 = &testStruct{1, "test1"}
	testVal2 = &testStruct{2, "test2"}
)

func TestMessage_VDataGlobal(t *testing.T) {
	tests := []struct {
		editMessage func(*message) error
		getKey      string
		expRes      interface{}
		expErr      error
	}{
		{
			editMessage: func(msg *message) error {
				return msg.setVDataGlobal(testKey1, testVal1)
			},
			getKey: testKey1,
			expRes: testVal1,
		},
		{
			editMessage: func(msg *message) error {
				return msg.setVDataGlobal(testKey1, testVal1)
			},
			getKey: testKey2,
			expRes: nil,
		},
		{
			editMessage: func(msg *message) error {
				if err := msg.setVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				return msg.setVDataGlobal(testKey2, testVal2)
			},
			getKey: testKey2,
			expRes: testVal2,
		},
		{
			editMessage: func(msg *message) error {
				if err := msg.setVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				return msg.setVDataGlobal(testKey1, testVal2)
			},
			expErr: errGlobalValueOverwrite,
		},
		{
			editMessage: func(msg *message) error {
				if err := msg.setVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				msg.mustSetVDataGlobal(testKey1, testVal2)
				return nil
			},
			getKey: testKey1,
			expRes: testVal2,
		},
	}
	for i, test := range tests {
		msg := &message{&libp2p_pubsub.Message{}}
		err := test.editMessage(msg)

		if assErr := herrors.AssertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: unexpected error %v", i, assErr)
		}
		if err != nil {
			continue
		}
		gotVal := msg.getVDataGlobal(test.getKey)
		if !reflect.DeepEqual(gotVal, test.expRes) {
			t.Errorf("Test %v: unexpected val: [%+v] / [%+v]", i, gotVal, test.expRes)
		}
	}
}

func TestMessage_HandlerData(t *testing.T) {
	tests := []struct {
		editMessage func(*message)
		getSpec     string
		expRes      interface{}
	}{
		{
			editMessage: func(msg *message) {
				msg.setValidatorDataByHandler(testSpec1, testVal1)
			},
			getSpec: testSpec1,
			expRes:  testVal1,
		},
		{
			editMessage: func(msg *message) {
				msg.setValidatorDataByHandler(testSpec2, testVal2)
			},
			getSpec: testSpec1,
			expRes:  nil,
		},
	}
	for i, test := range tests {
		msg := &message{&libp2p_pubsub.Message{}}
		test.editMessage(msg)

		got := msg.getHandlerCache(test.getSpec)
		if !reflect.DeepEqual(got, test.expRes) {
			t.Errorf("Test %v: unexpected val: [%+v] / [%+v]", i, got, test.expRes)
		}
	}
}

// TestContextChildCancel is a learning test for context cancel.
// Canceling a child context will not cancel parent context.
func TestContextChildCancel(t *testing.T) {
	parent, _ := context.WithCancel(context.Background())
	child, childCancel := context.WithCancel(parent)

	childCancel()

	select {
	case <-child.Done():
	default:
		t.Fatalf("child context should be cancelled")
	}

	select {
	case <-parent.Done():
		t.Fatalf("canceling child context should not cancel parent context")
	default:
	}
}

// TestContextParentCancel is a learning test showing that cancelling a parent
// context will also cancel the child context.
func TestContextParentCancel(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	child, _ := context.WithCancel(parent)

	parentCancel()

	select {
	case <-parent.Done():
	default:
		t.Fatalf("parent context should be canceled")
	}

	select {
	case <-child.Done():
	default:
		t.Fatalf("child context should also be canceled")
	}
}
