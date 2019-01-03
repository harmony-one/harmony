package beaconchain

import (
	"strconv"
	"testing"

	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/rpc"
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

func TestInitRPC(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	numshards := 2
	bc := New(numshards, ip, port)
	bc.InitRPCServer()
	if bc.rpcServer == nil {
		t.Error("beacon chain rpc server not initialized")
	}
}

func TestRPCServer(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "8080"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.SupportRPC()
	port, _ := strconv.Atoi(beaconport)
	bcClient := beaconchain.NewClient("127.0.0.1", strconv.Itoa(port+BeaconchainServicePortDiff))
	response := bcClient.GetHello()
	if response.GetMessage() != "hello" {
		t.Error("beaconchain rpc service is not responding")
	}
}
