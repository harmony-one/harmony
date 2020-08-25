package pubsub

import (
	"context"
	"errors"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_pb "github.com/libp2p/go-libp2p-pubsub/pb"
)

type fakePubSub struct {
	topicValidators map[string]interface{}
	topicMsg        map[string]chan *libp2p_pubsub.Message
}

func newFakePubSub() *fakePubSub {
	return &fakePubSub{
		topicValidators: make(map[string]interface{}),
		topicMsg:        make(map[string]chan *libp2p_pubsub.Message),
	}
}

func (ps *fakePubSub) Join(topic string) (psTopic, error) {
	if _, joined := ps.topicMsg[topic]; joined {
		return nil, errors.New("topic already joined")
	}
	msgCh := make(chan *libp2p_pubsub.Message, 100)
	ps.topicMsg[topic] = msgCh
	return &fakeTopic{
		msgCh:      msgCh,
		subscribed: false,
	}, nil
}

func (ps *fakePubSub) RegisterTopicValidator(topic string, val interface{}, opts ...libp2p_pubsub.ValidatorOpt) error {
	ps.topicValidators[topic] = val
	return nil
}

func (ps *fakePubSub) UnregisterTopicValidator(topic string) error {
	if _, exist := ps.topicValidators[topic]; !exist {
		return errors.New("topic validator not registered")
	}
	delete(ps.topicValidators, topic)
	return nil
}

func (ps *fakePubSub) addMessage(topic string, msg testMsg) error {
	msgCh, exist := ps.topicMsg[topic]
	if !exist {
		return errors.New("topic not joined")
	}

	p2pMsg := &libp2p_pubsub.Message{Message: &libp2p_pb.Message{Data: msg.encode()}}

	val, exist := ps.topicValidators[topic]
	if exist {
		valFunc := val.(func(ctx context.Context, peer PeerID, raw *libp2p_pubsub.Message) libp2p_pubsub.ValidationResult)
		result := valFunc(context.Background(), "", p2pMsg)
		if result != libp2p_pubsub.ValidationAccept {
			return nil
		}
	}

	msgCh <- p2pMsg
	return nil
}

type fakeTopic struct {
	msgCh      chan *libp2p_pubsub.Message
	subscribed bool
}

func (ft *fakeTopic) Subscribe() (subscription, error) {
	if ft.subscribed {
		return nil, errors.New("already subscribed")
	}
	ft.subscribed = true
	for {
		haveMsg := false
		select {
		case <-ft.msgCh:
			haveMsg = true
		default:
		}
		if !haveMsg {
			break
		}
	}
	return &fakeSubscription{ft, ft.msgCh, false}, nil
}

type fakeSubscription struct {
	topic    *fakeTopic
	msgCh    chan *libp2p_pubsub.Message
	Canceled bool
}

func (sub *fakeSubscription) Next(ctx context.Context) (*libp2p_pubsub.Message, error) {
	if sub.Canceled {
		return nil, errors.New("subscription already canceled")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-sub.msgCh:
		return msg, nil
	}
}

func (sub *fakeSubscription) Cancel() {
	sub.Canceled = true
	sub.topic.subscribed = false
}
