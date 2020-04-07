package client

import (
	"testing"
)

func TestClient(t *testing.T) {
	shardID := uint32(0)
	client := NewClient(nil, shardID)
	if client.ShardID != uint32(0) {
		t.Errorf("client initiate incorrect")
	}
}
