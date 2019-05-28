package node

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/hmyapi"
)

const (
	rpcHTTPPortOffset = 500
	rpcWSPortOffset   = 800
)

var (
	// HTTP RPC
	rpcAPIs      []rpc.API
	httpListener net.Listener
	httpHandler  *rpc.Server

	wsListener net.Listener
	wsHandler  *rpc.Server

	httpEndpoint = ""
	wsEndpoint   = ""

	httpModules      = []string{"hmy"}
	httpVirtualHosts = []string{"*"}
	httpTimeouts     = rpc.DefaultHTTPTimeouts

	wsModules = []string{"net", "web3"}
	wsOrigins = []string{"*"}

	apiBackend *core.HmyAPIBackend
)

// StartRPC start RPC service
func (node *Node) StartRPC(nodePort string) error {
	// Gather all the possible APIs to surface
	apiBackend = core.NewBackend(node.Blockchain(), node.TxPool, node.accountManager, node.addPendingTransaction)

	apis := hmyapi.GetAPIs(apiBackend)
	for _, service := range node.serviceManager.GetServices() {
		apis = append(apis, service.APIs()...)
	}

	port, _ := strconv.Atoi(nodePort)

	httpEndpoint = fmt.Sprintf(":%v", port+rpcHTTPPortOffset)
	if err := node.startHTTP(httpEndpoint, apis, httpModules, nil, httpVirtualHosts, httpTimeouts); err != nil {
		return err
	}

	wsEndpoint = fmt.Sprintf(":%v", port+rpcWSPortOffset)
	if err := node.startWS(wsEndpoint, apis, wsModules, wsOrigins, true); err != nil {
		node.stopHTTP()
		return err
	}

	rpcAPIs = apis
	return nil
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (node *Node) startHTTP(endpoint string, apis []rpc.API, modules []string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts) error {
	// Short circuit if the HTTP endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}

	listener, handler, err := rpc.StartHTTPEndpoint(endpoint, apis, modules, cors, vhosts, timeouts)
	if err != nil {
		return err
	}

	log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))
	// All listeners booted successfully
	httpListener = listener
	httpHandler = handler

	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (node *Node) stopHTTP() {
	if httpListener != nil {
		httpListener.Close()
		httpListener = nil

		log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", httpEndpoint))
	}
	if httpHandler != nil {
		httpHandler.Stop()
		httpHandler = nil
	}
}

// startWS initializes and starts the websocket RPC endpoint.
func (node *Node) startWS(endpoint string, apis []rpc.API, modules []string, wsOrigins []string, exposeAll bool) error {
	// Short circuit if the WS endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}
	listener, handler, err := rpc.StartWSEndpoint(endpoint, apis, modules, wsOrigins, exposeAll)
	if err != nil {
		return err
	}
	log.Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s", listener.Addr()))
	// All listeners booted successfully
	wsListener = listener
	wsHandler = handler

	return nil
}

// stopWS terminates the websocket RPC endpoint.
func (node *Node) stopWS() {
	if wsListener != nil {
		wsListener.Close()
		wsListener = nil

		log.Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", wsEndpoint))
	}
	if wsHandler != nil {
		wsHandler.Stop()
		wsHandler = nil
	}
}
