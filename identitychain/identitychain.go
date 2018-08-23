package identitychain

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

var mutex sync.Mutex
var identityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
	log               log.Logger
	Peer              p2p.Peer
}

func (IDC *IdentityChain) shard() {
	return
}

//IdentityChainHandler handles registration of new Identities
// This could have been its seperate package like consensus, but am avoiding creating a lot of packages.
func (IDC *IdentityChain) IdentityChainHandler(conn net.Conn) {
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed")
		return
	}
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		IDC.log.Error("Read message category failed", "err", err)
		return
	}
	if msgCategory != proto.IDENTITY {
		IDC.log.Error("Identity Chain Recieved incorrect protocol message")
	}
	msgType, err := proto_identity.GetIdentityMessageType(content)
	if err != nil {
		IDC.log.Error("Read action type failed")
		return
	}
	fmt.Println(msgType)
	msgPayload, err := proto_identity.GetIdentityMessagePayload(content)
	if err != nil {
		IDC.log.Error("Read message payload failed")
		return
	}
	fmt.Println(msgPayload)
	NewWaitNode := waitnode.DeserializeWaitNode(msgPayload)
	IDC.PendingIdentities = append(IDC.PendingIdentities, NewWaitNode)
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
		NewIdentities := IDC.PendingIdentities[:identityPerBlock]
		IDC.PendingIdentities = []*waitnode.WaitNode{}
		//All other blocks are dropped, we need to inform them that they are dropped?
		IDBlock := NewBlock(NewIdentities, prevBlockHash)
		IDC.Identities = append(IDC.Identities, IDBlock)
	}

}

//StartServer a server and process the request by a handler.
func (IDC *IdentityChain) StartServer() {
	fmt.Println("Starting server...")
	IDC.log.Info("Starting IDC server...") //log.Info does nothing for me! (ak)
	IDC.listenOnPort()
}

func (IDC *IdentityChain) listenOnPort() {
	listen, err := net.Listen("tcp4", ":"+IDC.Peer.Port)
	if err != nil {
		IDC.log.Crit("Socket listen port failed")
		os.Exit(1)
	} else {
		fmt.Println("Starting server...now listening")
		IDC.log.Info("Identity chain is now listening ..") //log.Info does nothing for me! (ak) remove this
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			IDC.log.Crit("Error listening on port. Exiting", IDC.Peer.Port)
			continue
		} else {
			fmt.Println("I am accepting connections now")
		}
		go IDC.IdentityChainHandler(conn)
	}
}

// New Create a new Node
func New(Peer p2p.Peer) *IdentityChain {
	IDC := IdentityChain{}
	IDC.Peer = Peer
	IDC.log = log.New()
	return &IDC
}
