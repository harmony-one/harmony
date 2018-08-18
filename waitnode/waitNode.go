package waitnode

import (
	"fmt"
	"net"
	"os"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

//WaitNode is for nodes waiting to join consensus
type WaitNode struct {
	Peer p2p.Peer
	Log  log.Logger
}

// StartServer a server and process the request by a handler.
func (node *WaitNode) StartServer(add p2p.Peer) {
	node.Log.Debug("Starting waitnode on server %d", "node", node.Peer.Ip, "port", node.Peer.Port)
	node.connectIdentityChain(add.Port)
}

func (node *WaitNode) connectIdentityChain(port string) {
	// replace by p2p peer
	identityChainIP := "127.0.0.1"
	fmt.Println("Connecting to identity chain")
	conn, err := net.Dial("tcp4", identityChainIP+":"+port)
	defer conn.Close()
	if err != nil {
		node.Log.Crit("Socket listen port failed", "port", port, "err", err)
		os.Exit(1)
	}
	//for {
	// 	conn, err := listen.Accept()
	// 	if err != nil {
	// 		node.log.Crit("Error listening on port. Exiting.", "port", port)
	// 		continue
	// 	}
	// }
}

// New Create a new Node
func New(Peer p2p.Peer) *WaitNode {
	node := WaitNode{}
	node.Peer = Peer
	node.Log = log.New()
	return &node
}
