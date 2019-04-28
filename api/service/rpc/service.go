package rpcservice

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/rpc"
	"github.com/harmony-one/harmony/rpc/hmyapi"
)

const (
	rpcPortDiff = 123
)

// Service is the struct for rpc service.
type Service struct {
	messageChan chan *msg_pb.Message
	server      *http.Server
	// Util
	peer       *p2p.Peer
	blockchain *core.BlockChain
	txPool     *core.TxPool
	// HTTP RPC
	rpcAPIs       []rpc.API    // List of APIs currently provided by the node
	httpEndpoint  string       // HTTP endpoint (interface + port) to listen at (empty = HTTP disabled)
	httpWhitelist []string     // HTTP RPC modules to allow through this endpoint
	httpListener  net.Listener // HTTP RPC listener socket to server API requests
	httpHandler   *rpc.Server  // HTTP RPC request handler to process the API requests
}

// New returns RPC service.
func New(b *core.BlockChain, p *p2p.Peer, t *core.TxPool) *Service {
	return &Service{
		blockchain: b,
		peer:       p,
		txPool:     t,
	}
}

// StartService starts RPC service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting RPC service.")
	if err := s.startRPC(); err != nil {
		utils.GetLogInstance().Error("Failed to start RPC service.")
	} else {
		utils.GetLogInstance().Info("Started RPC service.")
	}
}

// StopService shutdowns RPC service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Shutting down RPC service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.GetLogInstance().Error("Error when shutting down RPC server", "error", err)
	} else {
		utils.GetLogInstance().Error("Shutting down RPC server successufully")
	}
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// startRPC is a helper method to start all the various RPC endpoint during node
// startup. It's not meant to be called at any time afterwards as it makes certain
// assumptions about the state of the node.
func (s *Service) startRPC() error {
	apis := hmyapi.GetAPIs(rpc.NewBackend(s.blockchain, s.txPool))

	port, _ := strconv.Atoi(s.peer.Port)
	s.httpEndpoint = fmt.Sprintf("localhost:%v", port+rpcPortDiff)
	if err := s.startHTTP(s.httpEndpoint, apis); err != nil {
		utils.GetLogInstance().Debug("Failed to start RPC HTTP")
		return err
	}
	utils.GetLogInstance().Debug("Started RPC HTTP")

	// All API endpoints started successfully
	s.rpcAPIs = apis
	return nil
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (s *Service) startHTTP(endpoint string, apis []rpc.API) error {
	utils.GetLogInstance().Debug("rpc startHTTP", "endpoint", endpoint, "apis", apis)
	// Short circuit if the HTTP endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartHTTPEndpoint(endpoint, apis)
	if err != nil {
		return err
	}
	utils.GetLogInstance().Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint))
	// All listeners booted successfully
	s.httpEndpoint = endpoint
	s.httpListener = listener
	s.httpHandler = handler

	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (s *Service) stopHTTP() {
	if s.httpListener != nil {
		s.httpListener.Close()
		s.httpListener = nil

		utils.GetLogInstance().Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", s.httpEndpoint))
	}
	if s.httpHandler != nil {
		s.httpHandler.Stop()
		s.httpHandler = nil
	}
}
