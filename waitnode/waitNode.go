package waitnode

import (
	"fmt"

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
	log     log.Logger
}

func (node *WaitNode) doPoW() {
	node.log.Debug("Node with ID %d and IP %s is doing POW", node.ID, node.Address.IP)
}

// StartServer a server and process the request by a handler.
func (node *WaitNode) StartServer(add address) {
	node.log.Debug("Starting waitnode on server %d", "node", node.ID, "port", add.IP)
	node.connectIdentityChain(add.Port)
}

func (node *WaitNode) connectIdentityChain(port string) {
	fmt.Println("Connecting to identity chain")
	// listen, err := net.Listen("tcp4", ":"+port)
	// defer listen.Close()
	// if err != nil {
	// 	node.log.Crit("Socket listen port failed", "port", port, "err", err)
	// 	os.Exit(1)
	// }
	// for {
	// 	conn, err := listen.Accept()
	// 	if err != nil {
	// 		node.log.Crit("Error listening on port. Exiting.", "port", port)
	// 		continue
	// 	}
	// }
}

// New Create a new Node
func New(address *address, id int) *WaitNode {
	node := WaitNode{}
	node.Address = *address
	node.ID = id
	node.Worker = "pow"
	node.log = log.New()
	return &node
}
