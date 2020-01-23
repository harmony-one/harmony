package node

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/hmyapi"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv1"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv2"
	"github.com/harmony-one/harmony/internal/hmyapi/filters"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	rpcHTTPPortOffset = 500
	rpcWSPortOffset   = 800
)

var (
	// HTTP RPC
	rpcAPIs          []rpc.API
	httpListener     net.Listener
	httpHandler      *rpc.Server
	wsListener       net.Listener
	wsHandler        *rpc.Server
	httpEndpoint     = ""
	wsEndpoint       = ""
	httpModules      = []string{"hmy", "hmy_v2", "net", "net_v2", "explorer"}
	httpVirtualHosts = []string{"*"}
	httpTimeouts     = rpc.DefaultHTTPTimeouts
	httpOrigins      = []string{"*"}
	wsModules        = []string{"hmy", "hmy_v2", "net", "net_v2", "web3"}
	wsOrigins        = []string{"*"}
	harmony          *hmy.Harmony
)

// IsCurrentlyLeader exposes if node is currently the leader node
func (node *Node) IsCurrentlyLeader() bool {
	return node.Consensus.IsLeader()
}

// PendingCXReceipts returns node.pendingCXReceiptsProof
func (node *Node) PendingCXReceipts() []*types.CXReceiptsProof {
	cxReceipts := make([]*types.CXReceiptsProof, len(node.pendingCXReceipts))
	i := 0
	for _, cxReceipt := range node.pendingCXReceipts {
		cxReceipts[i] = cxReceipt
		i++
	}
	return cxReceipts
}

// ErroredStakingTransactionSink is the inmemory failed staking transactions this node has
func (node *Node) ErroredStakingTransactionSink() []staking.RPCTransactionError {
	node.errorSink.Lock()
	defer node.errorSink.Unlock()
	result := []staking.RPCTransactionError{}
	node.errorSink.failedStakingTxns.Do(func(d interface{}) {
		if d != nil {
			result = append(result, d.(staking.RPCTransactionError))
		}
	})
	return result
}

// ErroredTransactionSink is the inmemory failed transactions this node has
func (node *Node) ErroredTransactionSink() []types.RPCTransactionError {
	node.errorSink.Lock()
	defer node.errorSink.Unlock()
	result := []types.RPCTransactionError{}
	node.errorSink.failedTxns.Do(func(d interface{}) {
		if d != nil {
			result = append(result, d.(types.RPCTransactionError))
		}
	})
	return result
}

// IsBeaconChainExplorerNode ..
func (node *Node) IsBeaconChainExplorerNode() bool {
	return node.NodeConfig.Role() == nodeconfig.ExplorerNode &&
		node.Consensus.ShardID == shard.BeaconChainShardID
}

// StartRPC start RPC service
func (node *Node) StartRPC(nodePort string) error {
	// Gather all the possible APIs to surface
	harmony, _ = hmy.New(
		node, node.TxPool, node.CxPool, new(event.TypeMux), node.Consensus.ShardID,
	)

	apis := node.APIs()

	for _, service := range node.serviceManager.GetServices() {
		apis = append(apis, service.APIs()...)
	}

	port, _ := strconv.Atoi(nodePort)

	ip := ""
	if !nodeconfig.GetPublicRPC() {
		ip = "127.0.0.1"
	}
	httpEndpoint = fmt.Sprintf("%v:%v", ip, port+rpcHTTPPortOffset)

	if err := node.startHTTP(httpEndpoint, apis, httpModules, httpOrigins, httpVirtualHosts, httpTimeouts); err != nil {
		return err
	}
	wsEndpoint = fmt.Sprintf("%v:%v", ip, port+rpcWSPortOffset)
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

	utils.Logger().Info().
		Str("url", fmt.Sprintf("http://%s", endpoint)).
		Str("cors", strings.Join(cors, ",")).
		Str("vhosts", strings.Join(vhosts, ",")).
		Msg("HTTP endpoint opened")
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
		utils.Logger().Info().Str("url", fmt.Sprintf("http://%s", httpEndpoint)).Msg("HTTP endpoint closed")
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
	utils.Logger().Info().Str("url", fmt.Sprintf("ws://%s", listener.Addr())).Msg("WebSocket endpoint opened")
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
		utils.Logger().Info().Str("url", fmt.Sprintf("ws://%s", wsEndpoint)).Msg("WebSocket endpoint closed")
	}
	if wsHandler != nil {
		wsHandler.Stop()
		wsHandler = nil
	}
}

// APIs return the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (node *Node) APIs() []rpc.API {
	// Gather all the possible APIs to surface
	apis := hmyapi.GetAPIs(harmony.APIBackend)
	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(harmony.APIBackend, false),
			Public:    true,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   apiv1.NewPublicNetAPI(node.host, harmony.APIBackend.NetVersion()),
			Public:    true,
		},
		{
			Namespace: "net_v2",
			Version:   "1.0",
			Service:   apiv2.NewPublicNetAPI(node.host, harmony.APIBackend.NetVersion()),
			Public:    true,
		},
	}...)
}
