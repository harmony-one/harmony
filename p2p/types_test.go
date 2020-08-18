package p2p

import (
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/internal/testerr"
)

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
		msg := &Message{}
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
		msg := &Message{}
		test.editMessage(msg)

		got := msg.getVDataHandler(test.getSpec)
		if !reflect.DeepEqual(got, test.expRes) {
			t.Errorf("Test %v: unexpected val: [%+v] / [%+v]", i, got, test.expRes)
		}
	}
}
