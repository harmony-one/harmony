package downloader

import (
	"fmt"
	"testing"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/node"
)

const (
	serverPort = "9997"
	serverIP   = "127.0.0.1"
	clientPort = "9999"
)

var (
	PriIntOne      = 111
	PriIntTwo      = 222
	TestAddressOne = pki.GetAddressFromInt(PriIntOne)
	TestAddressTwo = pki.GetAddressFromInt(PriIntTwo)
)

func setupServer() *Server {
	bc := blockchain.CreateBlockchainWithMoreBlocks([][20]byte{TestAddressOne, TestAddressTwo}, 0)
	node := &node.Node{}
	node.SetBlockchain(bc)
	server := NewServer(node)
	fmt.Println("minh ", len(bc.Blocks))
	return server
}

func TestGetBlockHashes(t *testing.T) {
	server := setupServer()
	server.Start(serverIP)

	// client := ClientSetUp(serverIP, serverPort)
	// client.GetHeaders()
}
