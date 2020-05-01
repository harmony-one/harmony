package service

import (
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

func TestMessageChan(t *testing.T) {
	m := &Manager{}
	m.SetupServiceManager()
	msgChans := make(map[Type]chan *msg_pb.Message)
	m.SetupServiceMessageChan(msgChans)
}

func TestInit(t *testing.T) {
	if GroupIDShards[nodeconfig.ShardID(0)] != nodeconfig.NewGroupIDByShardID(0) {
		t.Errorf("GroupIDShards[0]: %v != GroupIDBeacon: %v",
			GroupIDShards[nodeconfig.ShardID(0)],
			nodeconfig.NewGroupIDByShardID(0),
		)
	}
	if len(GroupIDShards) != nodeconfig.MaxShards {
		t.Errorf("len(GroupIDShards): %v != TotalShards: %v",
			len(GroupIDShards),
			nodeconfig.MaxShards,
		)
	}
}
