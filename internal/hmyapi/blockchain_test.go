package hmyapi

import (
	"fmt"
	"testing"
)

func TestGetShardingStructure(t *testing.T) {
	shardID := 0
	numShard := 4
	res := GenShardingStructure(uint32(numShard), uint32(shardID), "http://s%d.t.hmy.io:9500", "ws://s%d.t.hmy.io:9800")
	if len(res) != 4 || !res[0]["current"].(bool) || res[1]["current"].(bool) || res[2]["current"].(bool) || res[3]["current"].(bool) {
		t.Error("Error when generating sharding structure")
	}
	for i := 0; i < numShard; i++ {
		if res[i]["current"].(bool) != (i == shardID) {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["shardID"].(int) != i {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["http"].(string) != fmt.Sprintf("http://s%d.t.hmy.io:9500", i) {
			t.Error("Error when generating sharding structure")
		}
		if res[i]["ws"].(string) != fmt.Sprintf("ws://s%d.t.hmy.io:9800", i) {
			t.Error("Error when generating sharding structure")
		}
	}
}
