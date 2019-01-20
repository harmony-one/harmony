package bcconn

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/internal/utils"
)

func TestSerializeDeserializeNodeInfo(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	_, pk := utils.GenKey(ip, port)
	pkb, err := pk.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo := &node.Info{IP: ip, Port: port, PubKey: pkb}
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
	_, pk := utils.GenKey(ip, port)
	pkb, err := pk.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo1 := &node.Info{IP: ip, Port: port, PubKey: pkb}

	ip = "127.0.0.1"
	port = "9080"
	_, pk2 := utils.GenKey(ip, port)
	pkb2, err := pk2.MarshalBinary()
	if err != nil {
		fmt.Println("problem marshalling binary from public key")
	}
	nodeInfo2 := &node.Info{IP: ip, Port: port, PubKey: pkb2}

	leaders := make([]*node.Info, 2)
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
