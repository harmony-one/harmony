package beaconchain

import (
	"fmt"
	"log"
	"reflect"
	"testing"
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
func TestSaveBC(t *testing.T) {

	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	numshards := 2
	bc := &BeaconChain{IP: ip, Port: port, NumberOfShards: numshards}
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
	//os.Remove("test.json")
	fmt.Println(bc2)
	fmt.Println(bc)

}
