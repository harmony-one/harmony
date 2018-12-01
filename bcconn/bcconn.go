package bcconn

import (
	"bytes"
	"encoding/gob"

	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
)

type NodeInfo struct { //TODO: to be merged with Leo's key.
	Self p2p.Peer
	PubK []byte
}
type ResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*NodeInfo
}

//SerializeNode
func SerializeNodeInfo(nodeinfo *NodeInfo) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(nodeinfo)
	if err != nil {
		log.Error("Could not serialize node info", err)
	}
	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeNodeInfo(d []byte) *NodeInfo {
	var wn NodeInfo
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Error("Could not de-serialize node info", err)
	}
	return &wn
}

//SerializeNode
func SerializeRandomInfo(response ResponseRandomNumber) []byte {
	//Needs to escape the serialization of unexported fields
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(response)
	if err != nil {
		log.Crit("Could not serialize randomn number information", "error", err)
	}

	return result.Bytes()
}

// DeserializeNode deserializes the node
func DeserializeRandomInfo(d []byte) ResponseRandomNumber {
	var wn ResponseRandomNumber
	r := bytes.NewBuffer(d)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&wn)
	if err != nil {
		log.Crit("Could not de-serialize random number information")
	}
	return wn
}
