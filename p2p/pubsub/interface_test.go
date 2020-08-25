package pubsub

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

type fakePubSubHandler struct {
	topic       string
	index       int
	numHandlers int
	deliverFunc func(ctx context.Context, rawData []byte, cache ValidateCache)
}

type (
	testStruct struct {
		field1 testMsg
		field2 string
	}
)

type testMsg uint64

func (msg testMsg) encode() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(msg))
	return b
}

func decodeTestMsg(b []byte) testMsg {
	return testMsg(binary.LittleEndian.Uint64(b))
}

func makeFakeHandlers(topic string, num int, deliver func(ctx context.Context, rawData []byte, cache ValidateCache)) []PubSubHandler {
	handlers := make([]PubSubHandler, 0, num)
	for i := 0; i != num; i++ {
		handler := &fakePubSubHandler{
			topic:       topic,
			index:       i,
			numHandlers: num,
			deliverFunc: deliver,
		}
		handlers = append(handlers, handler)
	}
	return handlers
}

func (handler *fakePubSubHandler) Topic() string {
	return handler.topic
}

func (handler *fakePubSubHandler) Specifier() string {
	return makeSpecifier(handler.index)
}

func makeSpecifier(index int) string {
	return fmt.Sprintf("testHandler [%v]", index)
}

func (handler *fakePubSubHandler) ValidateMsg(ctx context.Context, peer PeerID, rawData []byte) ValidateResult {
	var (
		action = MsgAccept
		err    error
	)
	msg := decodeTestMsg(rawData)
	if int(msg)%handler.numHandlers == handler.index {
		action = MsgReject
		err = errors.New("rejecting message")
	}

	return ValidateResult{
		ValidateCache: ValidateCache{
			GlobalCache: map[string]interface{}{
				fmt.Sprintf("%v", handler.index): handler.topic,
			},
			HandlerCache: testStruct{
				field1: msg,
				field2: string(rawData),
			},
		},
		Action: action,
		Err:    err,
	}
}

func (handler *fakePubSubHandler) DeliverMsg(ctx context.Context, rawData []byte, cache ValidateCache) {
	if handler.deliverFunc != nil {
		handler.deliverFunc(ctx, rawData, cache)
	}
	return
}
