package pubsub

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
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

var (
	testKey1 = "key1"
	testKey2 = "key2"

	testSpec1 = HandlerSpecifier("spec1")
	testSpec2 = HandlerSpecifier("spec2")

	testVal1 = &testStruct{1, "test1"}
	testVal2 = &testStruct{2, "test2"}

	testTopic = Topic("test topic")

	errIntended = errors.New("intended error")
)

func TestMergeValidateResults(t *testing.T) {
	tests := []struct {
		handlers []Handler
		vrs      []ValidateResult

		expCache  vData
		expAction ValidateAction
		expErr    error
	}{
		{
			handlers: nil,
			vrs:      nil,

			expCache:  newVData(),
			expAction: MsgAccept,
			expErr:    nil,
		},
		{
			handlers: makeFakeHandlers(testTopic, 2, nil, nil),
			vrs: []ValidateResult{
				{
					ValidateCache: ValidateCache{
						GlobalCache:  map[string]interface{}{testKey1: testVal1},
						HandlerCache: testVal1,
					},
					Action: MsgAccept,
				},
				{
					ValidateCache: ValidateCache{
						GlobalCache:  map[string]interface{}{testKey2: testVal2},
						HandlerCache: testVal2,
					},
					Action: MsgAccept,
				},
			},

			expCache: vData{
				globals:     map[string]interface{}{testKey1: testVal1, testKey2: testVal2},
				handlerData: map[HandlerSpecifier]interface{}{makeSpecifier(0): testVal1, makeSpecifier(1): testVal2},
			},
			expAction: MsgAccept,
			expErr:    nil,
		},
		{
			handlers: makeFakeHandlers(testTopic, 3, nil, nil),
			vrs: []ValidateResult{
				{
					ValidateCache: ValidateCache{
						GlobalCache:  map[string]interface{}{testKey1: testVal1},
						HandlerCache: testVal1,
					},
					Action: MsgAccept,
				},
				{
					ValidateCache: ValidateCache{
						GlobalCache:  map[string]interface{}{testKey2: testVal2},
						HandlerCache: testVal2,
					},
					Action: MsgReject,
					Err:    errIntended,
				},
			},

			expCache: vData{
				globals:     map[string]interface{}{testKey1: testVal1, testKey2: testVal2},
				handlerData: map[HandlerSpecifier]interface{}{makeSpecifier(0): testVal1, makeSpecifier(1): testVal2},
			},
			expAction: MsgReject,
			expErr:    errIntended,
		},
	}

	for i, test := range tests {
		cache, action, gotErr := mergeValidateResults(test.handlers, test.vrs)

		if err := assertVDataEqual(cache, test.expCache); err != nil {
			t.Errorf("Test %v: unexpected cache:\n\tgot:  \t%v\n\texpect: %v", i, cache, test.expCache)
		}
		if action != test.expAction {
			t.Errorf("Test %v: unexpected action: %v / %v", i, action, test.expAction)
		}
		if err := assertError(gotErr, test.expErr); err != nil {
			t.Errorf("Test %v: unexpected error:\n\tgot:  \t%v\n\texpect: %v", i, err, test.expErr)
		}
	}
}

func TestMessage_Cache(t *testing.T) {
	tests := []struct {
		cache           vData
		handlerSpec     HandlerSpecifier
		expHandlerCache ValidateCache
	}{
		{
			cache: vData{
				globals: getTestGlobals(),
				handlerData: map[HandlerSpecifier]interface{}{
					testSpec1: testVal1,
					testSpec2: testVal2,
				},
			},
			handlerSpec: testSpec1,
			expHandlerCache: ValidateCache{
				GlobalCache:  getTestGlobals(),
				HandlerCache: testVal1,
			},
		},
	}
	for i, test := range tests {
		m := newMessage(&libp2p_pubsub.Message{})

		m.setValidateCache(test.cache)

		got := m.getHandlerCache(test.handlerSpec)

		if err := assertValidateCacheEqual(got, test.expHandlerCache); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func getTestGlobals() map[string]interface{} {
	return map[string]interface{}{
		"key1": "value1",
		"key2": 2,
	}
}

var errNotEqual = errors.New("not equal")

func assertValidateCacheEqual(vc1, vc2 ValidateCache) error {
	if err := assertMapEqual(vc1.GlobalCache, vc2.GlobalCache); err != nil {
		return errors.Wrapf(err, "global cache %v", err)
	}
	if !reflect.DeepEqual(vc1.HandlerCache, vc2.HandlerCache) {
		return errors.Wrapf(errNotEqual, "handler cache:")
	}
	return nil
}

func assertVDataEqual(vd1, vd2 vData) error {
	if err := assertMapEqual(vd1.globals, vd2.globals); err != nil {
		return errors.Wrapf(err, "globals:")
	}
	if err := assertHandlerMapEqual(vd1.handlerData, vd2.handlerData); err != nil {
		return errors.Wrapf(err, "handler data:")
	}
	return nil
}

func assertMapEqual(m1, m2 map[string]interface{}) error {
	if len(m1) != len(m2) {
		return errNotEqual
	}
	for k1, v1 := range m1 {
		v2, exist := m2[k1]
		if !exist {
			return errNotEqual
		}
		if !reflect.DeepEqual(v1, v2) {
			return errNotEqual
		}
	}
	return nil
}

func assertHandlerMapEqual(m1, m2 map[HandlerSpecifier]interface{}) error {
	if len(m1) != len(m2) {
		return errNotEqual
	}
	for k1, v1 := range m1 {
		v2, exist := m2[k1]
		if !exist {
			return errNotEqual
		}
		if !reflect.DeepEqual(v1, v2) {
			return errNotEqual
		}
	}
	return nil
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

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	if (got == nil) || (expect == nil) {
		return nil
	}
	if !errors.Is(got, expect) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	return nil
}
