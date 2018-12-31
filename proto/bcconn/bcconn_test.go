package bcconn

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony-public/pkg/p2p"
	"github.com/harmony-one/harmony/utils"
)

func TestSerializeDeserializeNodeInfo(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	self := p2p.Peer{IP: ip, Port: port}
	_, pk := utils.GenKey(ip, port)
	pkb, err := pk.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo := &NodeInfo{Self: self, PubK: pkb}
	serializedNI := SerializeNodeInfo(nodeInfo)
	deserializedNI := DeserializeNodeInfo(serializedNI)
	if !reflect.DeepEqual(nodeInfo, deserializedNI) {
		t.Fatalf("serialized and deserializing nodeinfo does not lead to origina nodeinfo")
	}

}

func TestSerializeDeserializeRandomInfo(t *testing.T) {
	var ip, port string

	ip = "127.0.0.1"
	port = "8080"
	self := p2p.Peer{IP: ip, Port: port}
	_, pk := utils.GenKey(ip, port)
	pkb, err := pk.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo1 := &NodeInfo{Self: self, PubK: pkb}

	ip = "127.0.0.1"
	port = "9080"
	self2 := p2p.Peer{IP: ip, Port: port}
	_, pk2 := utils.GenKey(ip, port)
	pkb2, err := pk2.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo2 := &NodeInfo{Self: self2, PubK: pkb2}

	leaders := make([]*NodeInfo, 2)
	leaders[0] = nodeInfo1
	leaders[1] = nodeInfo2

	rrn := ResponseRandomNumber{NumberOfShards: 5, NumberOfNodesAdded: 10, Leaders: leaders}
	serializedrrn := SerializeRandomInfo(rrn)
	deserializedrrn := DeserializeRandomInfo(serializedrrn)
	fmt.Println(rrn)
	fmt.Println(deserializedrrn)
	if !reflect.DeepEqual(rrn, deserializedrrn) {
		t.Fatalf("serializin g and deserializing random response does not lead to original randominfo")
	}
}
