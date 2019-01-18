package beaconchain

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/harmony-one/harmony/api/proto/bcconn"
	"github.com/harmony-one/harmony/api/proto/node"
	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/rpc"
	"github.com/stretchr/testify/assert"
)

var (
	leader1        = &node.Info{IP: "127.0.0.1", Port: "9981"}
	leader2        = &node.Info{IP: "127.0.0.1", Port: "9982"}
	leaders        = []*node.Info{leader1, leader2}
	shardLeaderMap = map[int]*node.Info{
		0: leader1,
		1: leader2,
	}
)

func TestNewNode(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "7523"
	numshards := 2
	bc := New(numshards, ip, port)

	if bc.PubKey == nil {
		t.Error("beacon chain public key not initialized")
	}

	if bc.BCInfo.NumberOfNodesAdded != 0 {
		t.Error("beacon chain number of nodes starting with is not zero! (should be zero)")
	}

	if bc.BCInfo.NumberOfShards != numshards {
		t.Error("beacon chain number of shards not initialized to given number of desired shards")
	}
}

func TestShardLeaderMap(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "7523"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.BCInfo.Leaders = leaders
	if !reflect.DeepEqual(bc.GetShardLeaderMap(), shardLeaderMap) {
		t.Error("The function GetShardLeaderMap doesn't work well")
	}

}

func TestFetchLeaders(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "7523"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.BCInfo.Leaders = leaders
	bc.rpcServer = beaconchain.NewServer(bc.GetShardLeaderMap)
	bc.StartRPCServer()
	port, _ := strconv.Atoi(beaconport)
	bcClient := beaconchain.NewClient("127.0.0.1", strconv.Itoa(port+BeaconchainServicePortDiff))
	response := bcClient.GetLeaders()
	retleaders := response.GetLeaders()
	if !(retleaders[0].GetIp() == leaders[0].IP || retleaders[0].GetPort() == leaders[0].Port || retleaders[1].GetPort() == leaders[1].Port) {
		t.Error("Fetch leaders response is not as expected")
	}

}

func TestAcceptNodeInfo(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "7523"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	b := bcconn.SerializeNodeInfo(leader1)
	node := bc.AcceptNodeInfo(b)
	if !reflect.DeepEqual(node, leader1) {
		t.Error("Beaconchain is unable to deserialize incoming node info")
	}
	if len(bc.BCInfo.Leaders) != 1 {
		t.Error("Beaconchain was unable to update the leader array")
	}

}

func TestRespondRandomness(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "7523"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	bc.RespondRandomness(leader1)
	assert.Equal(t, RandomInfoSent, bc.state)
}

func TestAcceptConnections(t *testing.T) {
	var ip string
	ip = "127.0.0.1"
	beaconport := "7523"
	numshards := 1
	bc := New(numshards, ip, beaconport)
	b := bcconn.SerializeNodeInfo(leader1)
	bc.AcceptConnections(b)
	assert.Equal(t, RandomInfoSent, bc.state)
}

func TestSaveBC(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "7523"
	numshards := 2
	bci := &BCInfo{IP: ip, Port: port, NumberOfShards: numshards}
	bc := &BeaconChain{BCInfo: *bci}
	err := SaveBeaconChainInfo("test.json", bc)
	if err != nil {
		log.Fatalln(err)
	}
	bc2, err2 := LoadBeaconChainInfo("test.json")
	if err2 != nil {
		log.Fatalln(err2)
	}
	if !reflect.DeepEqual(bc, bc2) {
		t.Error("beacon chain info objects are not same")
	}
	os.Remove("test.json")
}

func TestSaveFile(t *testing.T) {
	filepath := "test"
	SetSaveFile(filepath)
	if !reflect.DeepEqual(filepath, SaveFile) {
		t.Error("Could not set savefile")
	}
}
