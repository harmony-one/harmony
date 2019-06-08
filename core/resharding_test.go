package core

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/harmony-one/harmony/core/types"
	"github.com/stretchr/testify/assert"
)

var (
	blsPubKey1  = [48]byte{}
	blsPubKey2  = [48]byte{}
	blsPubKey3  = [48]byte{}
	blsPubKey4  = [48]byte{}
	blsPubKey5  = [48]byte{}
	blsPubKey6  = [48]byte{}
	blsPubKey7  = [48]byte{}
	blsPubKey8  = [48]byte{}
	blsPubKey9  = [48]byte{}
	blsPubKey10 = [48]byte{}
)

func init() {
	copy(blsPubKey1[:], []byte("random key 1"))
	copy(blsPubKey2[:], []byte("random key 2"))
	copy(blsPubKey3[:], []byte("random key 3"))
	copy(blsPubKey4[:], []byte("random key 4"))
	copy(blsPubKey5[:], []byte("random key 5"))
	copy(blsPubKey6[:], []byte("random key 6"))
	copy(blsPubKey7[:], []byte("random key 7"))
	copy(blsPubKey8[:], []byte("random key 8"))
	copy(blsPubKey9[:], []byte("random key 9"))
	copy(blsPubKey10[:], []byte("random key 10"))
}

func fakeGetInitShardState(numberOfShards, numOfNodes int) types.ShardState {
	rand.Seed(int64(InitialSeed))
	shardState := types.ShardState{}
	for i := 0; i < numberOfShards; i++ {
		sid := uint32(i)
		com := types.Committee{ShardID: sid}
		for j := 0; j < numOfNodes; j++ {
			nid := strconv.Itoa(int(rand.Int63()))
			blsPubKey := [48]byte{}
			copy(blsPubKey1[:], []byte(nid))
			com.NodeList = append(com.NodeList, types.NodeID{nid, blsPubKey})
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
		blsPubKey := [48]byte{}
		copy(blsPubKey1[:], []byte(nid))
		nodeList = append(nodeList, types.NodeID{nid, blsPubKey})
	}
	return nodeList
}

func TestFakeNewNodeList(t *testing.T) {
	nodeList := fakeNewNodeList(42)
	fmt.Println("newNodeList: ", nodeList)
}

func TestShuffle(t *testing.T) {
	nodeList := []types.NodeID{
		{"node1", blsPubKey1},
		{"node2", blsPubKey2},
		{"node3", blsPubKey3},
		{"node4", blsPubKey4},
		{"node5", blsPubKey5},
		{"node6", blsPubKey6},
		{"node7", blsPubKey7},
		{"node8", blsPubKey8},
		{"node9", blsPubKey9},
		{"node10", blsPubKey10},
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
		{"node1", blsPubKey1},
		{"node2", blsPubKey2},
		{"node3", blsPubKey3},
		{"node4", blsPubKey4},
		{"node5", blsPubKey5},
		{"node6", blsPubKey6},
	}

	ss.Reshard(newNodeList, 0.2)
	assert.Equal(t, 6, ss.numShards)
}

func TestAssignNewNodes(t *testing.T) {
	shardState := fakeGetInitShardState(2, 2)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodes := []types.NodeID{
		{"node1", blsPubKey1},
		{"node2", blsPubKey2},
		{"node3", blsPubKey3},
	}

	ss.assignNewNodes(newNodes)
	assert.Equal(t, 2, ss.numShards)
	assert.Equal(t, 5, len(ss.shardState[0].NodeList))
}
