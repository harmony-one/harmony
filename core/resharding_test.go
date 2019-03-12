package core

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/harmony-one/harmony/core/types"
	"github.com/stretchr/testify/assert"
)

func fakeGetInitShardState(numberOfShards, numOfNodes int) types.ShardState {
	rand.Seed(int64(InitialSeed))
	shardState := types.ShardState{}
	for i := 0; i < numberOfShards; i++ {
		sid := uint32(i)
		com := types.Committee{ShardID: sid}
		for j := 0; j < numOfNodes; j++ {
			nid := strconv.Itoa(int(rand.Int63()))
			com.NodeList = append(com.NodeList, types.NodeID(nid))
		}
		shardState = append(shardState, com)
	}
	return shardState
}

func fakeNewNodeList(seed int64) []types.NodeID {
	rand.Seed(seed)
	numNewNodes := rand.Intn(10)
	nodeList := []types.NodeID{}
	for i := 0; i < numNewNodes; i++ {
		nid := strconv.Itoa(int(rand.Int63()))
		nodeList = append(nodeList, types.NodeID(nid))
	}
	return nodeList
}

func TestFakeNewNodeList(t *testing.T) {
	nodeList := fakeNewNodeList(42)
	fmt.Println("newNodeList: ", nodeList)
}

func TestShuffle(t *testing.T) {
	nodeList := []types.NodeID{
		"node1",
		"node2",
		"node3",
		"node4",
		"node5",
		"node6",
		"node7",
		"node8",
		"node9",
		"node10",
	}

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
	shardState := fakeGetInitShardState(6, 10)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	ss.sortCommitteeBySize()
	for i := 0; i < ss.numShards-1; i++ {
		assert.Equal(t, true, len(ss.shardState[i].NodeList) >= len(ss.shardState[i+1].NodeList))
	}
}

func TestUpdateShardState(t *testing.T) {
	shardState := fakeGetInitShardState(6, 10)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodeList := []types.NodeID{
		"node1",
		"node2",
		"node3",
		"node4",
		"node5",
		"node6",
	}

	ss.Reshard(newNodeList, 0.2)
	assert.Equal(t, 6, ss.numShards)
	for _, shard := range ss.shardState {
		assert.Equal(t, string(shard.Leader), string(shard.NodeList[0]))
	}
}

func TestAssignNewNodes(t *testing.T) {
	shardState := fakeGetInitShardState(2, 2)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodes := []types.NodeID{
		"node1",
		"node2",
		"node3",
	}

	ss.assignNewNodes(newNodes)
	assert.Equal(t, 2, ss.numShards)
	assert.Equal(t, 5, len(ss.shardState[0].NodeList))
}

func TestCalculateKickoutRate(t *testing.T) {
	shardState := fakeGetInitShardState(6, 10)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodeList := fakeNewNodeList(42)
	percent := ss.calculateKickoutRate(newNodeList)
	assert.Equal(t, 0.2, percent)
}
