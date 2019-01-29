package service

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	clientService "github.com/harmony-one/harmony/api/client/service"
	"github.com/harmony-one/harmony/core/state"
	"google.golang.org/grpc"
)

const (
	// ClientServicePortDiff is the positive port diff for client service
	ClientServicePortDiff = 5555
)

// SupportClient ...
type SupportClient struct {
	server     *clientService.Server
	grpcServer *grpc.Server
	ip         string
	port       string
}

// NewSupportClient ...
func NewSupportClient(stateReader func() (*state.DB, error), callFaucetContract func(common.Address) common.Hash, ip, nodePort string) *SupportClient {
	port, _ := strconv.Atoi(nodePort)
	return &SupportClient{server: clientService.NewServer(stateReader, callFaucetContract), ip: ip, port: strconv.Itoa(port + ClientServicePortDiff)}
}

// Start ...
func (sc *SupportClient) Start() {
	sc.grpcServer, _ = sc.server.Start(sc.ip, sc.port)
}

// Stop ...
func (sc *SupportClient) Stop() {
	sc.grpcServer.Stop()
}
