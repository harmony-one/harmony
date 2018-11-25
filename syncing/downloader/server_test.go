package downloader

import (
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
	TestAddressOne = pki.GetAddressFromInt(PriIntOne)
)

func setupServer() *Server {
	bc := blockchain.CreateBlockchain(TestAddressOne, 0)
	node := &node.Node{}
	node.SetBlockchain(bc)
	server := NewServer(node)
	return server
}

func TestGetBlockHashes(t *testing.T) {
	server := setupServer()
	server.Start(serverIP)

	// client := ClientSetUp(serverIP, serverPort)
	// client.GetHeaders()
}
