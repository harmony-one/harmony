package hostv2

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

func TestHostV2_SendMessageToGroups(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		okTopic := NewMocktopicHandle(mc)
		newTopic := NewMocktopicHandle(mc)
		groups := []nodeconfig.GroupID{"OK", "New"}
		data := []byte{1, 2, 3}
		joined := map[string]topicHandle{"OK": okTopic}
		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: joined}

		gomock.InOrder(
			// okTopic is already in joined map, JoinTopic shouldn't be called
			joiner.EXPECT().JoinTopic("OK").Times(0),
			okTopic.EXPECT().Publish(context.TODO(), data).Return(nil),
			// newTopic is not in joined map, JoinTopic should be called
			joiner.EXPECT().JoinTopic("New").Return(newTopic, nil),
			newTopic.EXPECT().Publish(context.TODO(), data).Return(nil),
		)

		err := host.SendMessageToGroups(groups, data)

		if err != nil {
			t.Errorf("expected no error; got %v", err)
		}
	})
	t.Run("JoinError", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		okTopic := NewMocktopicHandle(mc)
		groups := []nodeconfig.GroupID{"Error", "OK"}
		data := []byte{1, 2, 3}
		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		gomock.InOrder(
			// Make first join return an error
			joiner.EXPECT().JoinTopic("Error").Return(nil, errors.New("join error")),
			// Subsequent topics should still be processed after an error
			joiner.EXPECT().JoinTopic("OK").Return(okTopic, nil),
			okTopic.EXPECT().Publish(context.TODO(), data).Return(nil),
		)

		err := host.SendMessageToGroups(groups, data)

		if err == nil {
			t.Error("expected an error; got nil")
		}
	})
	t.Run("PublishError", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		okTopic := NewMocktopicHandle(mc)
		erringTopic := NewMocktopicHandle(mc)
		groups := []nodeconfig.GroupID{"Error", "OK"}
		data := []byte{1, 2, 3}
		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		gomock.InOrder(
			// Make first publish return an error
			joiner.EXPECT().JoinTopic("Error").Return(erringTopic, nil),
			erringTopic.EXPECT().Publish(context.TODO(), data).Return(errors.New("publish error")),
			// Subsequent topics should still be processed after an error
			joiner.EXPECT().JoinTopic("OK").Return(okTopic, nil),
			okTopic.EXPECT().Publish(context.TODO(), data).Return(nil),
		)

		if err := host.SendMessageToGroups(groups, data); err == nil {
			t.Error("expected an error; got nil")
		}
	})
}

func TestGroupReceiver_Close(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	sub := NewMocksubscription(mc)
	sub.EXPECT().Cancel()
	receiver := GroupReceiverImpl{sub: sub}

	err := receiver.Close()

	if err != nil {
		t.Errorf("expected no error but got %v", err)
	}
}

func pubsubMessage(from libp2p_peer.ID, data []byte) *libp2p_pubsub.Message {
	m := libp2p_pubsub_pb.Message{From: []byte(from), Data: data}
	return &libp2p_pubsub.Message{Message: &m}
}

func TestGroupReceiver_Receive(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		ctx := context.Background()
		sub := NewMocksubscription(mc)
		receiver := GroupReceiverImpl{sub: sub}
		wantSender := libp2p_peer.ID("OK")
		wantMsg := []byte{1, 2, 3}

		sub.EXPECT().Next(ctx).Return(pubsubMessage(wantSender, wantMsg), nil)

		gotMsg, gotSender, err := receiver.Receive(ctx)

		if err != nil {
			t.Errorf("expected no error; got %v", err)
		}
		if gotSender != wantSender {
			t.Errorf("expected sender %v; got %v", wantSender, gotSender)
		}
		if !reflect.DeepEqual(gotMsg, wantMsg) {
			t.Errorf("expected message %v; got %v", wantMsg, gotMsg)
		}
	})
	t.Run("Error", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		ctx := context.Background()
		sub := NewMocksubscription(mc)
		receiver := GroupReceiverImpl{sub: sub}

		sub.EXPECT().Next(ctx).Return(nil, errors.New("receive error"))

		msg, sender, err := receiver.Receive(ctx)

		if err == nil {
			t.Error("expected an error; got nil")
		}
		if sender != "" {
			t.Errorf("expected empty sender; got %v", sender)
		}
		if len(msg) > 0 {
			t.Errorf("expected empty message; got %v", msg)
		}
	})
}

func TestHostV2_GroupReceiver(t *testing.T) {
	t.Run("New", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		sub := &libp2p_pubsub.Subscription{}
		topic := NewMocktopicHandle(mc)
		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		gomock.InOrder(
			joiner.EXPECT().JoinTopic("ABC").Return(topic, nil),
			topic.EXPECT().Subscribe().Return(sub, nil),
		)

		gotReceiver, err := host.GroupReceiver("ABC")

		if r, ok := gotReceiver.(*GroupReceiverImpl); !ok {
			t.Errorf("expected a hostv2 GroupReceiverImpl; got %v", gotReceiver)
		} else if r.sub != sub {
			t.Errorf("unexpected subscriber %v", r.sub)
		}
		if err != nil {
			t.Errorf("expected no error; got %v", err)
		}
	})
	t.Run("JoinError", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		joiner.EXPECT().JoinTopic("ABC").Return(nil, errors.New("join error"))

		gotReceiver, err := host.GroupReceiver("ABC")

		if gotReceiver != nil {
			t.Errorf("expected a nil hostv2 GroupReceiverImpl; got %v", gotReceiver)
		}
		if err == nil {
			t.Error("expected an error; got none")
		}
	})
	t.Run("SubscribeError", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		topic := NewMocktopicHandle(mc)
		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		gomock.InOrder(
			joiner.EXPECT().JoinTopic("ABC").Return(topic, nil),
			topic.EXPECT().Subscribe().Return(nil, errors.New("subscription error")),
		)

		gotReceiver, err := host.GroupReceiver("ABC")

		if gotReceiver != nil {
			t.Errorf("expected a nil hostv2 GroupReceiverImpl; got %v", gotReceiver)
		}
		if err == nil {
			t.Error("expected an error; got none")
		}
	})
	t.Run("Closed", func(t *testing.T) {
		var emptyReceiver GroupReceiverImpl
		_, _, err := emptyReceiver.Receive(context.Background())
		if err == nil {
			t.Errorf("Receive() from nil/closed receiver did not return error")
		}
	})
}

func TestHostV2_getTopic(t *testing.T) {
	t.Run("NewOK", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		joiner := NewMocktopicJoiner(mc)
		want := NewMocktopicHandle(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		joiner.EXPECT().JoinTopic("ABC").Return(want, nil)

		got, err := host.getTopic("ABC")

		if err != nil {
			t.Errorf("want nil error; got %v", err)
		}
		if got != want {
			t.Errorf("want topic handle %v; got %v", want, got)
		}
		if _, ok := host.joined["ABC"]; !ok {
			t.Error("topic not found in joined map")
		}
	})
	t.Run("NewError", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		joiner := NewMocktopicJoiner(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{}}

		joiner.EXPECT().JoinTopic("ABC").Return(nil, errors.New("OMG"))

		got, err := host.getTopic("ABC")

		if err == nil {
			t.Error("want non-nil error; got nil")
		}
		if got != nil {
			t.Errorf("want nil handle; got %v", got)
		}
	})
	t.Run("Existing", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()

		joiner := NewMocktopicJoiner(mc)
		want := NewMocktopicHandle(mc)
		host := &HostV2{joiner: joiner, joined: map[string]topicHandle{"ABC": want}}

		joiner.EXPECT().JoinTopic("ABC").Times(0)

		got, err := host.getTopic("ABC")

		if err != nil {
			t.Errorf("want nil error; got %v", err)
		}
		if got != want {
			t.Errorf("want topic handle %v; got %v", want, got)
		}
	})
}
