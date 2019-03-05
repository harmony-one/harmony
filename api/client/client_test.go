package client

import (
	"testing"
)

func TestClient(t *testing.T) {
	shardIDs := []uint32{0}
	client := NewClient(nil, shardIDs)

	if len(client.ShardIDs) != 1 || client.ShardIDs[0] != 0 {
		t.Errorf("client initiate incorrect")
	}
}
