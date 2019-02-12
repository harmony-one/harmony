package core

import (
	"fmt"
	"testing"
)

func TestFakeGetInitShardState(t *testing.T) {
	ss := fakeGetInitShardState()
	for i := range ss {
		fmt.Printf("ShardID: %v, NodeList: %v\n", ss[i].ShardID, ss[i].NodeList)
	}
}

func TestFakeNewNodeList(t *testing.T) {
	nodeList := fakeNewNodeList(42)
	fmt.Println("newNodeList: ", nodeList)
}
