package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

type fakePubSubHandler struct {
	topic       string
	index       int
	numHandlers int
	delay       time.Duration
}

type (
	testStruct struct {
		field1 int
		field2 string
	}
)

func makeFakeHandlers(topic string, num int, delay time.Duration) []PubSubHandler {
	handlers := make([]PubSubHandler, 0, num)
	for i := 0; i != num; i++ {
		handler := &fakePubSubHandler{
			topic:       topic,
			index:       i,
			numHandlers: num,
			delay:       delay,
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
	if len(rawData)%handler.numHandlers == handler.index {
		action = MsgReject
		err = errors.New("rejecting message")
	}

	select {
	case <-time.After(handler.delay):
	case <-ctx.Done():
		return ValidateResult{
			Action: MsgReject,
			Err:    ctx.Err(),
		}
	}

	return ValidateResult{
		ValidateCache: ValidateCache{
			GlobalCache: map[string]interface{}{
				fmt.Sprintf("%v", handler.index): handler.topic,
			},
			HandlerCache: testStruct{
				field1: len(rawData),
				field2: string(rawData),
			},
		},
		Action: action,
		Err:    err,
	}
}

func (handler *fakePubSubHandler) DeliverMsg(ctx context.Context, rawData []byte, cache ValidateCache) {
	select {
	case <-time.After(handler.delay):
	case <-ctx.Done():
		return
	}
	return
}
