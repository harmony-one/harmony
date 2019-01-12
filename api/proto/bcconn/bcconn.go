package bcconn

import (
	"bytes"
	"encoding/gob"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/p2p"
)

//NodeInfo struct exists to share information on the node
type NodeInfo struct { //TODO: to be merged with Leo's nodeinfo.
	Self p2p.Peer
	PubK []byte
}

//ResponseRandomNumber struct for exchanging random information
type ResponseRandomNumber struct {
	NumberOfShards     int
	NumberOfNodesAdded int
	Leaders            []*NodeInfo
}

// SerializeNodeInfo is for serializing nodeinfo
func SerializeNodeInfo(nodeinfo *NodeInfo) []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(nodeinfo)
	if err != nil {
		log.Error("Could not serialize node info", err)
	}
	return result.Bytes()
}

// DeserializeNodeInfo deserializes the nodeinfo
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

// SerializeRandomInfo serializes random number informations
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

// DeserializeRandomInfo deserializes the random informations
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
