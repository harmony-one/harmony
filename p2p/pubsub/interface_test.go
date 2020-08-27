package pubsub

import (
	"context"
	"encoding/binary"
	"fmt"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type (
	deliverFunc  func(ctx context.Context, rawData []byte, cache ValidateCache)
	validateFunc func(rawData []byte) ValidateResult
)

type fakePubSubHandler struct {
	topic    Topic
	index    int
	validate validateFunc
	deliver  deliverFunc
}

type (
	testStruct struct {
		field1 testMsg
		field2 string
	}
)

func makeFakeHandlers(topic Topic, num int, validates []validateFunc, delivers []deliverFunc) []Handler {
	handlers := make([]Handler, 0, num)
	for i := 0; i != num; i++ {
		handler := &fakePubSubHandler{
			topic: topic,
			index: i,
		}
		if i < len(delivers) {
			handler.deliver = delivers[i]
		}
		if i < len(validates) {
			handler.validate = validates[i]
		}
		handlers = append(handlers, handler)
	}
	return handlers
}

func (handler *fakePubSubHandler) Topic() Topic {
	return handler.topic
}

func (handler *fakePubSubHandler) Specifier() HandlerSpecifier {
	return makeSpecifier(handler.index)
}

func makeSpecifier(index int) HandlerSpecifier {
	return HandlerSpecifier(fmt.Sprintf("testHandler [%v]", index))
}

func (handler *fakePubSubHandler) ValidateMsg(ctx context.Context, peer PeerID, rawData []byte) ValidateResult {
	if handler.validate == nil {
		return ValidateResult{
			ValidateCache: ValidateCache{},
			Action:        MsgAccept,
			Err:           nil,
		}
	}
	return handler.validate(rawData)
}

func (handler *fakePubSubHandler) DeliverMsg(ctx context.Context, rawData []byte, cache ValidateCache) {
	if handler.deliver != nil {
		handler.deliver(ctx, rawData, cache)
	}
	return
}

type testMsg uint64

func (msg testMsg) encode() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(msg))
	return b
}

func decodeTestMsg(b []byte) testMsg {
	return testMsg(binary.LittleEndian.Uint64(b))
}

type emptyValidateOptionProvider struct{}

func (vop *emptyValidateOptionProvider) getValidateOptions(topic Topic) []libp2p_pubsub.ValidatorOpt {
	return nil
}
