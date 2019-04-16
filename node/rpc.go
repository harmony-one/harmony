package node

import (
	"fmt"
	"strconv"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/rpc"
	"github.com/harmony-one/harmony/rpc/hmyapi"
)

// apis returns the collection of RPC descriptors this node offers.
func (n *Node) apis() []rpc.API {
	return []rpc.API{
		// {
		// 	Namespace: "web3",
		// 	Version:   "1.0",
		// 	Service:   NewPublicWeb3API(n),
		// 	Public:    true,
		// },
	}
}

// startRPC is a helper method to start all the various RPC endpoint during node
// startup. It's not meant to be called at any time afterwards as it makes certain
// assumptions about the state of the node.
func (n *Node) startRPC(b *core.BlockChain) error {
	apis := hmyapi.GetAPIs(b)
	// Gather all the possible APIs to surface
	// apis := n.apis()
	// for _, service := range services {
	// 	apis = append(apis, service.APIs()...)
	// }
	// Start the various API endpoints, terminating all in case of errors
	// if err := n.startInProc(apis); err != nil {
	// 	return err
	// }
	// if err := n.startIPC(apis); err != nil {
	// 	n.stopInProc()
	// 	return err
	// }
	port, _ := strconv.Atoi(n.SelfPeer.Port)
	n.httpEndpoint = fmt.Sprintf("127.0.0.1:%v", port+123)
	if err := n.startHTTP(n.httpEndpoint, apis); err != nil {
		utils.GetLogInstance().Debug("Failed to start RPC HTTP")
		// n.stopIPC()
		// n.stopInProc()
		return err
	}
	// if err := n.startWS(n.wsEndpoint, apis, n.config.WSModules, n.config.WSOrigins, n.config.WSExposeAll); err != nil {
	// 	n.stopHTTP()
	// 	n.stopIPC()
	// 	n.stopInProc()
	// 	return err
	// }
	// All API endpoints started successfully
	n.rpcAPIs = apis
	return nil
}

// // startInProc initializes an in-process RPC endpoint.
// func (n *Node) startInProc(apis []rpc.API) error {
// 	// Register all the APIs exposed by the services
// 	handler := rpc.NewServer()
// 	for _, api := range apis {
// 		if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
// 			return err
// 		}
// 		utils.GetLogInstance().Debug("InProc registered", "namespace", api.Namespace)
// 	}
// 	n.inprocHandler = handler
// 	return nil
// }

// // stopInProc terminates the in-process RPC endpoint.
// func (n *Node) stopInProc() {
// 	if n.inprocHandler != nil {
// 		n.inprocHandler.Stop()
// 		n.inprocHandler = nil
// 	}
// }

// // startIPC initializes and starts the IPC RPC endpoint.
// func (n *Node) startIPC(apis []rpc.API) error {
// 	if n.ipcEndpoint == "" {
// 		return nil // IPC disabled.
// 	}
// 	listener, handler, err := rpc.StartIPCEndpoint(n.ipcEndpoint, apis)
// 	if err != nil {
// 		return err
// 	}
// 	n.ipcListener = listener
// 	n.ipcHandler = handler
// 	utils.GetLogInstance().Info("IPC endpoint opened", "url", n.ipcEndpoint)
// 	return nil
// }

// // stopIPC terminates the IPC RPC endpoint.
// func (n *Node) stopIPC() {
// 	if n.ipcListener != nil {
// 		n.ipcListener.Close()
// 		n.ipcListener = nil

// 		utils.GetLogInstance().Info("IPC endpoint closed", "url", n.ipcEndpoint)
// 	}
// 	if n.ipcHandler != nil {
// 		n.ipcHandler.Stop()
// 		n.ipcHandler = nil
// 	}
// }

// startHTTP initializes and starts the HTTP RPC endpoint.
func (n *Node) startHTTP(endpoint string, apis []rpc.API) error {
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
	n.httpEndpoint = endpoint
	n.httpListener = listener
	n.httpHandler = handler

	return nil
}

// stopHTTP terminates the HTTP RPC endpoint.
func (n *Node) stopHTTP() {
	if n.httpListener != nil {
		n.httpListener.Close()
		n.httpListener = nil

		utils.GetLogInstance().Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", n.httpEndpoint))
	}
	if n.httpHandler != nil {
		n.httpHandler.Stop()
		n.httpHandler = nil
	}
}

// // startWS initializes and starts the websocket RPC endpoint.
// func (n *Node) startWS(endpoint string, apis []rpc.API, modules []string, wsOrigins []string, exposeAll bool) error {
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
// 	n.wsEndpoint = endpoint
// 	n.wsListener = listener
// 	n.wsHandler = handler

// 	return nil
// }

// // stopWS terminates the websocket RPC endpoint.
// func (n *Node) stopWS() {
// 	if n.wsListener != nil {
// 		n.wsListener.Close()
// 		n.wsListener = nil

// 		utils.GetLogInstance().Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", n.wsEndpoint))
// 	}
// 	if n.wsHandler != nil {
// 		n.wsHandler.Stop()
// 		n.wsHandler = nil
// 	}
// }
