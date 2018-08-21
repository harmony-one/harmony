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
var IdentityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
	log               log.Logger
	Peer              p2p.Peer
}

//IdentityChainHandler handles registration of new Identities
func (IDC *IdentityChain) IdentityChainHandler(conn net.Conn) {
	// Read p2p message payload
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed")
		return
	}
	fmt.Printf("content is %b", content)

	// msgCategory, err := proto.GetMessageCategory(content)
	// if err != nil {
	// 	IDC.log.Error("Read node type failed", "err", err, "node", node)
	// 	return
	// }

	// msgType, err := proto.GetMessageType(content)
	// if err != nil {
	// 	IDC.log.Error("Read action type failed", "err", err, "node", node)
	// 	return
	// }

	// msgPayload, err := proto.GetMessagePayload(content)
	// if err != nil {
	// 	IDC.log.Error("Read message payload failed", "err", err, "node", node)
	// 	return
	// }
	// NewWaitNode := *waitnode.DeserializeWaitNode(msgPayload)
	// IDC.PendingIdentities = append(IDC.PendingIdentities, NewWaitNode)
}

// GetLatestBlock gests the latest block at the end of the chain
func (IDC *IdentityChain) GetLatestBlock() *IdentityBlock {
	if len(IDC.Identities) == 0 {
		return nil
	}
	return IDC.Identities[len(IDC.Identities)-1]
}

//UpdateIdentityChain is to create the Blocks to be added to the chain
func (IDC *IdentityChain) UpdateIdentityChain() {

	//If there are no more Identities registring the blockchain is dead
	if len(IDC.PendingIdentities) == 0 {
		// This is abd, because previous block might not be alive
		return
	}
	if len(IDC.Identities) == 0 {
		block := NewGenesisBlock()
		IDC.Identities = append(IDC.Identities, block)
	} else {
		prevBlock := IDC.GetLatestBlock()
		prevBlockHash := prevBlock.CalculateBlockHash()
		NewIdentities := IDC.PendingIdentities[:IdentityPerBlock]
		IDC.PendingIdentities = IDC.PendingIdentities[IdentityPerBlock:]
		//All other blocks are dropped.
		IDBlock := NewBlock(NewIdentities, prevBlockHash)
		IDC.Identities = append(IDC.Identities, IDBlock)
	}

}

func (IDC *IdentityChain) listenOnPort() {
	listen, err := net.Listen("tcp4", IDC.Peer.Ip+":"+IDC.Peer.Port)
	defer listen.Close()
	if err != nil {
		IDC.log.Crit("Socket listen port failed", "port", IDC.Peer.Port, "err", err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			IDC.log.Crit("Error listening on port. Exiting.", "port", IDC.Peer.Port)
			continue
		}
		go IDC.IdentityChainHandler(conn)
	}
}

func main() {
	fmt.Print("Hi")
}
