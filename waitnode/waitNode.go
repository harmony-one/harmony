package waitnode

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto/identity"
	"github.com/simple-rules/harmony-benchmark/utils"
)

//WaitNode is for nodes waiting to join consensus
type WaitNode struct {
	Peer p2p.Peer
	Log  log.Logger
	ID   []byte
}

// StartServer a server and process the request by a handler.
func (node *WaitNode) StartServer() {
	node.Log.Debug("Starting waitnode on server %d", "node", node.Peer.Ip, "port", node.Peer.Port)
}

func (node *WaitNode) connectIdentityChain(peer p2p.Peer) {
	// replace by p2p peer
	p2p.SendMessage(peer, identity.ConstructIdentityMessage(identity.REGISTER, node.SerializeWaitNode()))

}
func calculateHash(num string) []byte {
	var hashes [][]byte
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(num))
	hash := sha256.Sum256(bytes.Join(hashes, []byte{}))
	return hash[:]
}

// Serialize serializes the block
func (node *WaitNode) SerializeWaitNode() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(node)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeWaitNode(d []byte) *WaitNode {
	var block WaitNode
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&WaitNode)
	if err != nil {
		log.Panic(err)
	}
	return &WaitNode
}

// New Create a new Node
func New(Peer p2p.Peer) *WaitNode {
	node := WaitNode{}
	node.Peer = Peer
	node.ID = calculateHash(Peer.Ip)
	node.Log = log.New()
	return &node
}
