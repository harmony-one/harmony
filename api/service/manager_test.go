package service

import (
	"fmt"
	"testing"
	"time"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
)

type SupportSyncingTest struct {
	msgChan chan *msg_pb.Message
	status  *int
}

func (s *SupportSyncingTest) StartService() {
	fmt.Println("SupportSyncingTest starting")
	if s.status != nil {
		*s.status = 1
	}
}

func (s *SupportSyncingTest) StopService() {
	fmt.Println("SupportSyncingTest stopping")
	if s.status != nil {
		*s.status = 2
	}
}

func (s *SupportSyncingTest) NotifyService(data map[string]interface{}) {
	t := data["test"].(*testing.T)

	if s.msgChan != data["chan"].(chan *msg_pb.Message) {
		t.Error("message chan is not equal to the one from the passing params")
	}
}
func (s *SupportSyncingTest) SetMessageChan(msgChan chan *msg_pb.Message) {
	s.msgChan = msgChan
}

// Test TakeAction.
func TestTakeAction(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()
	m.RegisterService(SupportSyncing, &SupportSyncingTest{})

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			m.SendAction(&Action{Action: Start, ServiceType: SupportSyncing})
		}
	}

	m.SendAction(&Action{ServiceType: Done})
}

func TestMessageChan(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()
	m.RegisterService(SupportSyncing, &SupportSyncingTest{})
	msgChans := make(map[Type]chan *msg_pb.Message)
	m.SetupServiceMessageChan(msgChans)

	m.SendAction(&Action{
		Action:      Notify,
		ServiceType: SupportSyncing,
		Params: map[string]interface{}{
			"chan": msgChans[SupportSyncing],
			"test": t,
		},
	})

	m.SendAction(&Action{ServiceType: Done})
}

func TestStopServices(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()
	status := 0
	m.RegisterService(SupportSyncing, &SupportSyncingTest{status: &status})
	msgChans := make(map[Type]chan *msg_pb.Message)
	m.SetupServiceMessageChan(msgChans)

	m.SendAction(&Action{
		Action:      Notify,
		ServiceType: SupportSyncing,
		Params: map[string]interface{}{
			"chan": msgChans[SupportSyncing],
			"test": t,
		},
	})

	m.StopServicesByRole([]Type{})
	if status != 2 {
		t.Error("Service did not stop")
	}
}

func TestInit(t *testing.T) {
	if GroupIDShards[p2p.ShardID(0)] != p2p.GroupIDBeacon {
		t.Errorf("GroupIDShards[0]: %v != GroupIDBeacon: %v", GroupIDShards[p2p.ShardID(0)], p2p.GroupIDBeacon)
	}
	if len(GroupIDShards) != nodeconfig.MaxShards {
		t.Errorf("len(GroupIDShards): %v != TotalShards: %v", len(GroupIDShards), nodeconfig.MaxShards)
	}
}
