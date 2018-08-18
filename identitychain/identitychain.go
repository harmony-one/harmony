package identitychain

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

var mutex sync.Mutex

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
	log               log.Logger
}

//IdentityChainHandler handles transactions
func (IDC *IdentityChain) IdentityChainHandler(conn net.Conn) {
	fmt.Println("yay")
}
func (IDC *IdentityChain) listenOnPort(port string) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		IDC.log.Crit("Socket listen port failed", "port", port, "err", err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			IDC.log.Crit("Error listening on port. Exiting.", "port", port)
			continue
		}
		go IDC.IdentityChainHandler(conn)
	}
}

func main() {
	var IDC IdentityChain
	var nullPeer p2p.Peer
	go func() {
		genesisBlock := &IdentityBlock{nullPeer, 0}
		mutex.Lock()
		IDC.Identities = append(IDC.Identities, genesisBlock)
		mutex.Unlock()

	}()
}
