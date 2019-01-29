package service

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	clientService "github.com/harmony-one/harmony/api/client/service"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff = 5555
)

// SupportClient ...
type SupportClient struct {
	server *clientService.Server
	port   string
}

// NewSupportClient ...
func NewSupportClient(stateReader func() (*state.DB, error), callFaucetContract func(common.Address) common.Hash, nodePort string) *SupportClient {
	port, _ := strconv.Atoi(nodePort)
	return &SupportClient{server: clientService.NewServer(stateReader, callFaucetContract), port: strconv.Itoa(port + ClientServicePortDiff)}
}

// Start ...
func (sc *SupportClient) Start() {
	sc.server.Start(sc.ip, sc.port)
}

// Stop ...
func (sc *SupportClient) Stop() {

}

// SupportClient initializes and starts the client service
func (node *Node) SupportClient() {
	node.InitClientServer()
	node.StartClientServer()
}

// InitClientServer initializes client server.
func (node *Node) InitClientServer() {
	node.clientServer = clientService.NewServer(node.blockchain.State, node.CallFaucetContract)
}

// StartClientServer starts client server.
func (node *Node) StartClientServer() {
	port, _ := strconv.Atoi(node.SelfPeer.Port)
	utils.GetLogInstance().Info("support_client: StartClientServer on port:", "port", port+ClientServicePortDiff)
	node.clientServer.Start(node.SelfPeer.IP, strconv.Itoa(port+ClientServicePortDiff))
}
