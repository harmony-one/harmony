package core

import (
	"fmt"
	"testing"

	"github.com/harmony-one/harmony/core/types"
	"github.com/stretchr/testify/assert"
)

func TestFakeNewNodeList(t *testing.T) {
	nodeList := fakeNewNodeList(42)
	fmt.Println("newNodeList: ", nodeList)
}

func TestShuffle(t *testing.T) {
	nodeList := []types.NodeID{"node1", "node2", "node3", "node4", "node5", "node6", "node7", "node8", "node9", "node10"}
	cpList := []types.NodeID{}
	cpList = append(cpList, nodeList...)
	Shuffle(nodeList)
	cnt := 0
	for i := 0; i < 10; i++ {
		if cpList[i] == nodeList[i] {
			cnt++
		}
	}
	if cnt == 10 {
		t.Error("Shuffle list is the same as original list")
	}
	return
}

func TestSortCommitteeBySize(t *testing.T) {
	shardState := fakeGetInitShardState()
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	ss.sortCommitteeBySize()
	for i := 0; i < ss.numShards-1; i++ {
		assert.Equal(t, true, len(ss.shardState[i].NodeList) >= len(ss.shardState[i+1].NodeList))
	}

}

func TestCalculateKickoutRate(t *testing.T) {
	shardState := fakeGetInitShardState()
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodeList := fakeNewNodeList(42)
	percent := ss.calculateKickoutRate(newNodeList)
	assert.Equal(t, 0.2, percent)
}
