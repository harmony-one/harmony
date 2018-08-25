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
var identityPerBlock = 100000

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
	log               log.Logger
	Peer              p2p.Peer
}

//GlobalBlockchainConfig stores global level blockchain configurations.
type GlobalBlockchainConfig struct {
	NumberOfShards int
	EpochTimeSecs  int16
}

func (IDC *IdentityChain) shard() {
	return
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
