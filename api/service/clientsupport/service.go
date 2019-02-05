package clientsupport

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

// Service is the client support service.
type Service struct {
	server     *clientService.Server
	grpcServer *grpc.Server
	ip         string
	port       string
}

// New returns new client support service.
func New(stateReader func() (*state.DB, error), callFaucetContract func(common.Address) common.Hash, ip, nodePort string) *Service {
	port, _ := strconv.Atoi(nodePort)
	return &Service{server: clientService.NewServer(stateReader, callFaucetContract), ip: ip, port: strconv.Itoa(port + ClientServicePortDiff)}
}

// StartService starts client support service.
func (sc *Service) StartService() {
	sc.grpcServer, _ = sc.server.Start(sc.ip, sc.port)
}

// StopService stops client support service.
func (sc *Service) StopService() {
	sc.grpcServer.Stop()
}
