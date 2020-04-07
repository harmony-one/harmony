package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
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

func (s *SupportSyncingTest) APIs() []rpc.API { return nil }

// Test TakeAction.
func TestTakeAction(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()

	for i := 0; i < 2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
			return
		}
	}

}

func TestMessageChan(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()
	msgChans := make(map[Type]chan *msg_pb.Message)
	m.SetupServiceMessageChan(msgChans)
}

func TestInit(t *testing.T) {
	if GroupIDShards[nodeconfig.ShardID(0)] != nodeconfig.NewGroupIDByShardID(0) {
		t.Errorf("GroupIDShards[0]: %v != GroupIDBeacon: %v", GroupIDShards[nodeconfig.ShardID(0)], nodeconfig.NewGroupIDByShardID(0))
	}
	if len(GroupIDShards) != nodeconfig.MaxShards {
		t.Errorf("len(GroupIDShards): %v != TotalShards: %v", len(GroupIDShards), nodeconfig.MaxShards)
	}
}
