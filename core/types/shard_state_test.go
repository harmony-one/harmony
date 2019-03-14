package types

import (
	"bytes"
	"testing"
)

func TestGetHashFromNodeList(t *testing.T) {
	l1 := []NodeID{
		{"node1", "node1"},
		{"node2", "node2"},
		{"node3", "node3"},
	}
	l2 := []NodeID{
		{"node2", "node2"},
		{"node1", "node1"},
		{"node3", "node3"},
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
		Leader:  NodeID{"node11", "node11"},
		NodeList: []NodeID{
			{"node11", "node11"},
			{"node22", "node22"},
			{"node1", "node1"},
		},
	}
	com2 := Committee{
		ShardID: 2,
		Leader:  NodeID{"node4", "node4"},
		NodeList: []NodeID{
			{"node4", "node4"},
			{"node5", "node5"},
			{"node6", "node6"},
		},
	}
	shardState1 := ShardState{com1, com2}
	h1 := shardState1.Hash()

	com3 := Committee{
		ShardID: 2,
		Leader:  NodeID{"node4", "node4"},
		NodeList: []NodeID{
			{"node6", "node6"},
			{"node5", "node5"},
			{"node4", "node4"},
		},
	}
	com4 := Committee{
		ShardID: 22,
		Leader:  NodeID{"node11", "node11"},
		NodeList: []NodeID{
			{"node1", "node1"},
			{"node11", "node11"},
			{"node22", "node22"},
		},
	}

	shardState2 := ShardState{com3, com4}
	h2 := shardState2.Hash()

	if bytes.Compare(h1[:], h2[:]) != 0 {
		t.Error("shardState1 and shardState2 should have equal hash")
	}
}
