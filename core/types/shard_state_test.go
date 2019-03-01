package types

import (
	"bytes"
	"testing"
)

func TestGetHashFromNodeList(t *testing.T) {
	l1 := []NodeInfo{
		NodeInfo{NodeID: "node1", IsLeader: true},
		NodeInfo{NodeID: "node2", IsLeader: false},
		NodeInfo{NodeID: "node3", IsLeader: false},
	}
	l2 := []NodeInfo{
		NodeInfo{NodeID: "node2", IsLeader: false},
		NodeInfo{NodeID: "node1", IsLeader: true},
		NodeInfo{NodeID: "node3", IsLeader: false},
	}
	h1 := GetHashFromNodeList(l1)
	h2 := GetHashFromNodeList(l2)

	if bytes.Compare(h1, h2) != 0 {
		t.Error("node list l1 and l2 should have equal hash")
	}
}

func TestHash(t *testing.T) {
	com1 := Committee{
		ShardID: 22,
		NodeList: []NodeInfo{
			NodeInfo{NodeID: "node11", IsLeader: true},
			NodeInfo{NodeID: "node22", IsLeader: false},
			NodeInfo{NodeID: "node1", IsLeader: false},
		},
	}
	com2 := Committee{
		ShardID: 2,
		NodeList: []NodeInfo{
			NodeInfo{NodeID: "node4", IsLeader: true},
			NodeInfo{NodeID: "node5", IsLeader: false},
			NodeInfo{NodeID: "node6", IsLeader: false},
		},
	}
	shardState1 := ShardState{com1, com2}
	h1 := shardState1.Hash()

	com3 := Committee{
		ShardID: 2,
		NodeList: []NodeInfo{
			NodeInfo{NodeID: "node6", IsLeader: false},
			NodeInfo{NodeID: "node5", IsLeader: false},
			NodeInfo{NodeID: "node4", IsLeader: true},
		},
	}
	com4 := Committee{
		ShardID: 22,
		NodeList: []NodeInfo{
			NodeInfo{NodeID: "node1", IsLeader: false},
			NodeInfo{NodeID: "node11", IsLeader: true},
			NodeInfo{NodeID: "node22", IsLeader: false},
		},
	}

	shardState2 := ShardState{com3, com4}
	h2 := shardState2.Hash()

	if bytes.Compare(h1[:], h2[:]) != 0 {
		t.Error("shardState1 and shardState2 should have equal hash")
	}
}
