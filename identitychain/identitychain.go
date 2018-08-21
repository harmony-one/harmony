package identitychain

import (
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

var mutex sync.Mutex
var IdentityPerBlock := 100000
// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
	log               log.Logger
	Peer			   p2p.Peer
}

//IdentityChainHandler handles registration of new Identities
func (IDC *IdentityChain) IdentityChainHandler(conn net.Conn) {
	// Read p2p message payload
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}
	
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		IDC.log.Error("Read node type failed", "err", err, "node", node)
		return
	}

	msgType, err := proto.GetMessageType(content)
	if err != nil {
		IDC.log.Error("Read action type failed", "err", err, "node", node)
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		IDC.log.Error("Read message payload failed", "err", err, "node", node)
		return
	}
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}
}

// GetLatestBlock gests the latest block at the end of the chain
func (IDC *IdentityChain) GetLatestBlock() *IdentityBlock {
	if len(IDC.Identities) == 0 {
		return nil
	}
	return IDC.Identities[len(IDC.Identities)-1]
}

//CreateNewBlock is to create the Blocks to be added to the chain
func (IDC *IdentityChain) MakeNewBlock() *IdentityBlock {
	
	if len(IDC.Identities) == 0 {
		return NewGenesisBlock()
	}
	//If there are no more Identities registring the blockchain is dead
	if len(IDC.PendingIdentities) == 0 {
		// This is abd, because previous block might not be alive
		return IDC.GetLatestBlock()
	}
	prevBlock := IDC.GetLatestBlock()
	NewIdentities  := IDC.PendingIdentities[:IdentityPerBlock]
	IDC.PendingIdentities = IDC.PendingIdentities[IdentityPerBlock]:
	//All other blocks are dropped.
	IDBlock = NewBlock(NewIdentities,prevBlock.CalculateBlockHash())
	IDC.Identities = append(IDBlock,IDC.Identities)
	}
}

func (IDC *IdentityChain) listenOnPort() {
	listen, err := net.Listen("tcp4", IDC.Peer.Ip + ":" IDC.Peer.Port)
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
