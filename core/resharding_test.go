package core

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/shard"

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

func fakeGetInitShardState(numberOfShards, numOfNodes int) shard.ShardState {
	rand.Seed(int64(42))
	shardState := shard.ShardState{}
	for i := 0; i < numberOfShards; i++ {
		sid := uint32(i)
		com := shard.Committee{ShardID: sid}
		for j := 0; j < numOfNodes; j++ {
			nid := strconv.Itoa(int(rand.Int63()))
			blsPubKey := [48]byte{}
			copy(blsPubKey1[:], []byte(nid))
			com.NodeList = append(com.NodeList, shard.NodeID{
				EcdsaAddress: common.BytesToAddress([]byte(nid)),
				BlsPublicKey: blsPubKey,
			})
		}
		shardState = append(shardState, com)
	}
	return shardState
}

func fakeNewNodeList(seed int64) []shard.NodeID {
	rand.Seed(seed)
	numNewNodes := rand.Intn(10)
	nodeList := []shard.NodeID{}
	for i := 0; i < numNewNodes; i++ {
		nid := strconv.Itoa(int(rand.Int63()))
		blsPubKey := [48]byte{}
		copy(blsPubKey1[:], []byte(nid))
		nodeList = append(nodeList, shard.NodeID{
			EcdsaAddress: common.BytesToAddress([]byte(nid)),
			BlsPublicKey: blsPubKey,
		})
	}
	return nodeList
}

func TestFakeNewNodeList(t *testing.T) {
	nodeList := fakeNewNodeList(42)
	fmt.Println("newNodeList: ", nodeList)
}

func TestShuffle(t *testing.T) {
	nodeList := []shard.NodeID{
		{EcdsaAddress: common.Address{0x12}, BlsPublicKey: blsPubKey1},
		{EcdsaAddress: common.Address{0x22}, BlsPublicKey: blsPubKey2},
		{EcdsaAddress: common.Address{0x32}, BlsPublicKey: blsPubKey3},
		{EcdsaAddress: common.Address{0x42}, BlsPublicKey: blsPubKey4},
		{EcdsaAddress: common.Address{0x52}, BlsPublicKey: blsPubKey5},
		{EcdsaAddress: common.Address{0x62}, BlsPublicKey: blsPubKey6},
		{EcdsaAddress: common.Address{0x72}, BlsPublicKey: blsPubKey7},
		{EcdsaAddress: common.Address{0x82}, BlsPublicKey: blsPubKey8},
		{EcdsaAddress: common.Address{0x92}, BlsPublicKey: blsPubKey9},
		{EcdsaAddress: common.Address{0x02}, BlsPublicKey: blsPubKey10},
	}

	cpList := []shard.NodeID{}
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
	newNodeList := []shard.NodeID{
		{EcdsaAddress: common.Address{0x12}, BlsPublicKey: blsPubKey1},
		{EcdsaAddress: common.Address{0x22}, BlsPublicKey: blsPubKey2},
		{EcdsaAddress: common.Address{0x32}, BlsPublicKey: blsPubKey3},
		{EcdsaAddress: common.Address{0x42}, BlsPublicKey: blsPubKey4},
		{EcdsaAddress: common.Address{0x52}, BlsPublicKey: blsPubKey5},
		{EcdsaAddress: common.Address{0x62}, BlsPublicKey: blsPubKey6},
	}

	ss.Reshard(newNodeList, 0.2)
	assert.Equal(t, 6, ss.numShards)
}

func TestAssignNewNodes(t *testing.T) {
	shardState := fakeGetInitShardState(2, 2)
	ss := &ShardingState{epoch: 1, rnd: 42, shardState: shardState, numShards: len(shardState)}
	newNodes := []shard.NodeID{
		{EcdsaAddress: common.Address{0x12}, BlsPublicKey: blsPubKey1},
		{EcdsaAddress: common.Address{0x22}, BlsPublicKey: blsPubKey2},
		{EcdsaAddress: common.Address{0x32}, BlsPublicKey: blsPubKey3},
	}

	ss.assignNewNodes(newNodes)
	assert.Equal(t, 2, ss.numShards)
	assert.Equal(t, 5, len(ss.shardState[0].NodeList))
}
