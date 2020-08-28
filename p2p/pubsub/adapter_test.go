package pubsub

import (
	"context"
	"errors"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_pb "github.com/libp2p/go-libp2p-pubsub/pb"
)

type fakePubSub struct {
	topicValidators map[Topic]interface{}
	topicHandles    map[Topic]*fakeTopic
}

func newFakePubSub() *fakePubSub {
	return &fakePubSub{
		topicValidators: make(map[Topic]interface{}),
		topicHandles:    make(map[Topic]*fakeTopic),
	}
}

func (ps *fakePubSub) Join(topic Topic) (topicHandle, error) {
	if _, joined := ps.topicHandles[topic]; joined {
		return nil, errors.New("topic already joined")
	}
	ft := &fakeTopic{
		ps:         ps,
		topic:      topic,
		msgCh:      make(chan *libp2p_pubsub.Message, 100),
		subscribed: false,
	}
	ps.topicHandles[topic] = ft
	return ft, nil
}

func (ps *fakePubSub) RegisterTopicValidator(topic Topic, val interface{}, opts ...libp2p_pubsub.ValidatorOpt) error {
	ps.topicValidators[topic] = val
	return nil
}

func (ps *fakePubSub) UnregisterTopicValidator(topic Topic) error {
	if _, exist := ps.topicValidators[topic]; !exist {
		return errors.New("topic validator not registered")
	}
	delete(ps.topicValidators, topic)
	return nil
}

func (ps *fakePubSub) addMessage(topic Topic, msg testMsg) error {
	ft, exist := ps.topicHandles[topic]
	if !exist {
		return errors.New("topic not joined")
	}
	ft.Publish(context.Background(), msg.encode())
	return nil
}

type fakeTopic struct {
	ps         *fakePubSub
	topic      Topic
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

func (ft *fakeTopic) Publish(ctx context.Context, data []byte) error {
	p2pMsg := &libp2p_pubsub.Message{Message: &libp2p_pb.Message{Data: data}}

	val, exist := ft.ps.topicValidators[ft.topic]
	if exist {
		valFunc := val.(func(ctx context.Context, peer PeerID, raw *libp2p_pubsub.Message) libp2p_pubsub.ValidationResult)
		result := valFunc(context.Background(), "", p2pMsg)
		if result != libp2p_pubsub.ValidationAccept {
			return nil
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ft.msgCh <- p2pMsg:
	}
	return nil
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
