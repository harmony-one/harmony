package beaconchain

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/harmony-one/harmony/api/proto/bcconn"
	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/rpc"
	"github.com/harmony-one/harmony/p2p"
)

var (
	leader1        = &bcconn.NodeInfo{Self: p2p.Peer{IP: "127.0.0.1", Port: "1"}}
	leader2        = &bcconn.NodeInfo{Self: p2p.Peer{IP: "127.0.0.1", Port: "2"}}
	leaders        = []*bcconn.NodeInfo{leader1, leader2}
	shardLeaderMap = map[int]*bcconn.NodeInfo{
		0: leader1,
		1: leader2,
	}
)

func TestNewNode(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	numshards := 2
	bc := New(numshards, ip, port)

	if bc.PubKey == nil {
		t.Error("beacon chain public key not initialized")
	}

	if bc.NumberOfNodesAdded != 0 {
		t.Error("beacon chain number of nodes starting with is not zero! (should be zero)")
	}

	if bc.NumberOfShards != numshards {
		t.Error("beacon chain number of shards not initialized to given number of desired shards")
	}
}

func TestShardLeaderMap(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "8080"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.Leaders = leaders
	if !reflect.DeepEqual(bc.GetShardLeaderMap(), shardLeaderMap) {
		t.Error("The function GetShardLeaderMap doesn't work well")
	}

}

func TestFetchLeaders(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "8080"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.Leaders = leaders
	bc.rpcServer = beaconchain.NewServer(bc.GetShardLeaderMap)
	bc.StartRPCServer()
	port, _ := strconv.Atoi(beaconport)
	bcClient := beaconchain.NewClient("127.0.0.1", strconv.Itoa(port+BeaconchainServicePortDiff))
	response := bcClient.GetLeaders()
	retleaders := response.GetLeaders()
	if !(retleaders[0].GetIp() == leaders[0].Self.IP || retleaders[0].GetPort() == leaders[0].Self.Port || retleaders[1].GetPort() == leaders[1].Self.Port) {
		t.Error("Fetch leaders response is not as expected")
	}

}
