package waitnode

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"

	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto/identity"
	"github.com/simple-rules/harmony-benchmark/utils"
)

//WaitNode is for nodes waiting to join consensus
type WaitNode struct {
	Peer p2p.Peer
	ID   uint16
}

// StartServer a server and process the request by a handler.
func (node *WaitNode) StartServer() {
	log.Printf("Starting waitnode on server %s and port %s", node.Peer.Ip, node.Peer.Port)
}

//ConnectIdentityChain connects to identity chain
func (node *WaitNode) ConnectIdentityChain(peer p2p.Peer) {
	p2p.SendMessage(peer, identity.ConstructIdentityMessage(identity.REGISTER, node.SerializeWaitNode()))
}

//Constructs node-id by hashing the IP.
func calculateHash(num string) []byte {
	var hashes [][]byte
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(num))
	hash := sha256.Sum256(bytes.Join(hashes, []byte{}))
	return hash[:]
}

//SerializeWaitNode serializes the node
func (node *WaitNode) SerializeWaitNode() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(node)
	if err != nil {
		log.Panic(err.Error())
	}
	return result.Bytes()
}

// DeserializeWaitNode deserializes the node
func DeserializeWaitNode(d []byte) *WaitNode {
	var wn WaitNode
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&wn)
	if err != nil {
		log.Panic(err)
	}
	return &wn
}

// New Create a new Node
func New(Peer p2p.Peer) *WaitNode {
	node := WaitNode{}
	node.Peer = Peer
	node.ID = utils.GetUniqueIdFromPeer(Peer)
	return &node
}
