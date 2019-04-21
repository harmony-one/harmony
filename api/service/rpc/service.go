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

// Service is the struct for rpc service.
type Service struct {
	messageChan chan *msg_pb.Message
	server      *http.Server
	// Util
	peer       *p2p.Peer
	blockchain *core.BlockChain
	// HTTP RPC
	rpcAPIs       []rpc.API    // List of APIs currently provided by the node
	httpEndpoint  string       // HTTP endpoint (interface + port) to listen at (empty = HTTP disabled)
	httpWhitelist []string     // HTTP RPC modules to allow through this endpoint
	httpListener  net.Listener // HTTP RPC listener socket to server API requests
	httpHandler   *rpc.Server  // HTTP RPC request handler to process the API requests
}

// New returns RPC service.
func New(b *core.BlockChain, p *p2p.Peer) *Service {
	return &Service{
		blockchain: b,
		peer:       p,
	}
}

// StartService starts RPC service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting RPC service.")
	if err := s.startRPC(s.blockchain); err != nil {
		// TODO(ricl): what if failed to start service?
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
func (s *Service) startRPC(b *core.BlockChain) error {
	apis := hmyapi.GetAPIs(b)
	// Gather all the possible APIs to surface
	// apis := s.apis()
	// for _, service := range services {
	// 	apis = append(apis, service.APIs()...)
	// }
	// Start the various API endpoints, terminating all in case of errors
	// if err := s.startInProc(apis); err != nil {
	// 	return err
	// }
	// if err := s.startIPC(apis); err != nil {
	// 	s.stopInProc()
	// 	return err
	// }
	port, _ := strconv.Atoi(s.peer.Port)
	s.httpEndpoint = fmt.Sprintf("127.0.0.1:%v", port+123)
	if err := s.startHTTP(s.httpEndpoint, apis); err != nil {
		utils.GetLogInstance().Debug("Failed to start RPC HTTP")
		// s.stopIPC()
		// s.stopInProc()
		return err
	}
	utils.GetLogInstance().Debug("Started RPC HTTP")
	// if err := s.startWS(s.wsEndpoint, apis, s.config.WSModules, s.config.WSOrigins, s.config.WSExposeAll); err != nil {
	// 	s.stopHTTP()
	// 	s.stopIPC()
	// 	s.stopInProc()
	// 	return err
	// }
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

// // startInProc initializes an in-process RPC endpoint.
// func (s *Node) startInProc(apis []rpc.API) error {
// 	// Register all the APIs exposed by the services
// 	handler := rpc.NewServer()
// 	for _, api := range apis {
// 		if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
// 			return err
// 		}
// 		utils.GetLogInstance().Debug("InProc registered", "namespace", api.Namespace)
// 	}
// 	s.inprocHandler = handler
// 	return nil
// }

// // stopInProc terminates the in-process RPC endpoint.
// func (s *Node) stopInProc() {
// 	if s.inprocHandler != nil {
// 		s.inprocHandler.Stop()
// 		s.inprocHandler = nil
// 	}
// }

// // startIPC initializes and starts the IPC RPC endpoint.
// func (s *Node) startIPC(apis []rpc.API) error {
// 	if s.ipcEndpoint == "" {
// 		return nil // IPC disabled.
// 	}
// 	listener, handler, err := rpc.StartIPCEndpoint(s.ipcEndpoint, apis)
// 	if err != nil {
// 		return err
// 	}
// 	s.ipcListener = listener
// 	s.ipcHandler = handler
// 	utils.GetLogInstance().Info("IPC endpoint opened", "url", s.ipcEndpoint)
// 	return nil
// }

// // stopIPC terminates the IPC RPC endpoint.
// func (s *Node) stopIPC() {
// 	if s.ipcListener != nil {
// 		s.ipcListener.Close()
// 		s.ipcListener = nil

// 		utils.GetLogInstance().Info("IPC endpoint closed", "url", s.ipcEndpoint)
// 	}
// 	if s.ipcHandler != nil {
// 		s.ipcHandler.Stop()
// 		s.ipcHandler = nil
// 	}
// }

// // startWS initializes and starts the websocket RPC endpoint.
// func (s *Node) startWS(endpoint string, apis []rpc.API, modules []string, wsOrigins []string, exposeAll bool) error {
// 	// Short circuit if the WS endpoint isn't being exposed
// 	if endpoint == "" {
// 		return nil
// 	}
// 	listener, handler, err := rpc.StartWSEndpoint(endpoint, apis, modules, wsOrigins, exposeAll)
// 	if err != nil {
// 		return err
// 	}
// 	utils.GetLogInstance().Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s", listener.Addr()))
// 	// All listeners booted successfully
// 	s.wsEndpoint = endpoint
// 	s.wsListener = listener
// 	s.wsHandler = handler

// 	return nil
// }

// // stopWS terminates the websocket RPC endpoint.
// func (s *Node) stopWS() {
// 	if s.wsListener != nil {
// 		s.wsListener.Close()
// 		s.wsListener = nil

// 		utils.GetLogInstance().Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", s.wsEndpoint))
// 	}
// 	if s.wsHandler != nil {
// 		s.wsHandler.Stop()
// 		s.wsHandler = nil
// 	}
// }
