package waitnode

import (
	"net"
	"os"
	"github.com/simple-rules/harmony-benchmark/log"
)

type address struct {
	IP   string
	Port string
}

//WaitNode is for nodes waiting to join consensus
type WaitNode struct {
	Address address
	Worker  string
	ID      int
	Log     log.new()
}

func (node *WaitNode) doPoW(pow string) {
	return "pow"
}

// StartServer a server and process the request by a handler.
func (node *WaitNode) StartServer(add address) {
	node.log.Debug("Starting waitnode on server", "node", node.Id, "port", add.IP)
	node.listenOnPort(add.Port)
}

func (node *WaitNode) listenOnPort(port string) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		node.log.Crit("Socket listen port failed", "port", port, "err", err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			node.log.Crit("Error listening on port. Exiting.", "port", port)
			continue
		}
		go node.NodeHandler(conn)
	}
}

// New Create a new Node
func New(address *address) *WaitNode {
	node := WaitNode{}

	// Consensus and associated channel to communicate blocks
	node.Address = *address
	node.ID = 1
	node.Worker = "pow"
	return &node
}
