package p2p

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/harmony-one/harmony/internal/testerr"
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
		editMessage func(*Message) error
		getKey      string
		expRes      interface{}
		expErr      error
	}{
		{
			editMessage: func(msg *Message) error {
				return msg.SetVDataGlobal(testKey1, testVal1)
			},
			getKey: testKey1,
			expRes: testVal1,
		},
		{
			editMessage: func(msg *Message) error {
				return msg.SetVDataGlobal(testKey1, testVal1)
			},
			getKey: testKey2,
			expRes: nil,
		},
		{
			editMessage: func(msg *Message) error {
				if err := msg.SetVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				return msg.SetVDataGlobal(testKey2, testVal2)
			},
			getKey: testKey2,
			expRes: testVal2,
		},
		{
			editMessage: func(msg *Message) error {
				if err := msg.SetVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				return msg.SetVDataGlobal(testKey1, testVal2)
			},
			expErr: errGlobalValueOverwrite,
		},
		{
			editMessage: func(msg *Message) error {
				if err := msg.SetVDataGlobal(testKey1, testVal1); err != nil {
					return err
				}
				msg.MustSetVDataGlobal(testKey1, testVal2)
				return nil
			},
			getKey: testKey1,
			expRes: testVal2,
		},
	}
	for i, test := range tests {
		msg := &Message{&libp2p_pubsub.Message{}}
		err := test.editMessage(msg)

		if assErr := testerr.AssertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: unexpected error %v", i, assErr)
		}
		if err != nil {
			continue
		}
		gotVal := msg.GetVDataGlobal(test.getKey)
		if !reflect.DeepEqual(gotVal, test.expRes) {
			t.Errorf("Test %v: unexpected val: [%+v] / [%+v]", i, gotVal, test.expRes)
		}
	}
}

func TestMessage_HandlerData(t *testing.T) {
	tests := []struct {
		editMessage func(*Message)
		getSpec     string
		expRes      interface{}
	}{
		{
			editMessage: func(msg *Message) {
				msg.setValidatorDataByHandler(testSpec1, testVal1)
			},
			getSpec: testSpec1,
			expRes:  testVal1,
		},
		{
			editMessage: func(msg *Message) {
				msg.setValidatorDataByHandler(testSpec2, testVal2)
			},
			getSpec: testSpec1,
			expRes:  nil,
		},
	}
	for i, test := range tests {
		msg := &Message{&libp2p_pubsub.Message{}}
		test.editMessage(msg)

		got := msg.getVDataHandler(test.getSpec)
		if !reflect.DeepEqual(got, test.expRes) {
			t.Errorf("Test %v: unexpected val: [%+v] / [%+v]", i, got, test.expRes)
		}
	}
}

// Learning test for context cancel. Canceling a child context will not cancel parent context.
func ExampleContextCancel() {
	parent, _ := context.WithCancel(context.Background())

	child, childCancel := context.WithCancel(parent)

	childCancel()

	select {
	case <-child.Done():
		fmt.Println("child already canceled")
	}

	select {
	case <-parent.Done():
		fmt.Println("parent also canceled")
	default:
		fmt.Println("parent not canceled")
	}
	// Output:
	// child already canceled
	// parent not canceled
}
