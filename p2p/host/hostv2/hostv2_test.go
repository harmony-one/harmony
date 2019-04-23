package hostv2

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	libp2p_pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/harmony-one/harmony/p2p"
	mock "github.com/harmony-one/harmony/p2p/host/hostv2/mock"
)

func TestHostV2_SendMessageToGroups(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()
		groups := []p2p.GroupID{"ABC", "DEF"}
		data := []byte{1, 2, 3}
		pubsub := mock.NewMockpubsub(mc)
		gomock.InOrder(
			pubsub.EXPECT().Publish("ABC", data),
			pubsub.EXPECT().Publish("DEF", data),
		)
		host := &HostV2{pubsub: pubsub}
		if err := host.SendMessageToGroups(groups, data); err != nil {
			t.Errorf("expected no error; got %v", err)
		}
	})
	t.Run("Error", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()
		groups := []p2p.GroupID{"ABC", "DEF"}
		data := []byte{1, 2, 3}
		pubsub := mock.NewMockpubsub(mc)
		gomock.InOrder(
			pubsub.EXPECT().Publish("ABC", data).Return(errors.New("FIAL")),
			pubsub.EXPECT().Publish("DEF", data), // Should not early-return
		)
		host := &HostV2{pubsub: pubsub}
		if err := host.SendMessageToGroups(groups, data); err == nil {
			t.Error("expected an error but got none")
		}
	})
}

func TestGroupReceiver_Close(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()
	sub := mock.NewMocksubscription(mc)
	sub.EXPECT().Cancel()
	receiver := GroupReceiverImpl{sub: sub}
	if err := receiver.Close(); err != nil {
		t.Errorf("expected no error but got %v", err)
	}
}

func pubsubMessage(from libp2p_peer.ID, data []byte) *libp2p_pubsub.Message {
	m := libp2p_pubsub_pb.Message{From: []byte(from), Data: data}
	return &libp2p_pubsub.Message{Message: &m}
}

func TestGroupReceiver_Receive(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()
	sub := mock.NewMocksubscription(mc)
	ctx := context.Background()
	gomock.InOrder(
		sub.EXPECT().Next(ctx).Return(pubsubMessage("ABC", []byte{1, 2, 3}), nil),
		sub.EXPECT().Next(ctx).Return(pubsubMessage("DEF", []byte{4, 5, 6}), nil),
		sub.EXPECT().Next(ctx).Return(nil, errors.New("FIAL")),
	)
	receiver := GroupReceiverImpl{sub: sub}
	verify := func(sender libp2p_peer.ID, msg []byte, shouldError bool) {
		gotMsg, gotSender, err := receiver.Receive(ctx)
		if (err != nil) != shouldError {
			if shouldError {
				t.Error("expected an error but got none")
			} else {
				t.Errorf("expected no error but got %v", err)
			}
		}
		if gotSender != sender {
			t.Errorf("expected sender %v but got %v", sender, gotSender)
		}
		if !reflect.DeepEqual(gotMsg, msg) {
			t.Errorf("expected message %v but got %v", msg, gotMsg)
		}
	}
	verify("ABC", []byte{1, 2, 3}, false)
	verify("DEF", []byte{4, 5, 6}, false)
	verify("", nil, true)
}

func TestHostV2_GroupReceiver(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()
		sub := &libp2p_pubsub.Subscription{}
		pubsub := mock.NewMockpubsub(mc)
		pubsub.EXPECT().Subscribe("ABC").Return(sub, nil)
		host := &HostV2{pubsub: pubsub}
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
	t.Run("Error", func(t *testing.T) {
		mc := gomock.NewController(t)
		defer mc.Finish()
		pubsub := mock.NewMockpubsub(mc)
		pubsub.EXPECT().Subscribe("ABC").Return(nil, errors.New("FIAL"))
		host := &HostV2{pubsub: pubsub}
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
