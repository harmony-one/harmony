package node

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/hmyapi"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv1"
	"github.com/harmony-one/harmony/internal/hmyapi/apiv2"
	"github.com/harmony-one/harmony/internal/hmyapi/filters"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	rpcHTTPPortOffset = 500
	rpcWSPortOffset   = 800
)

var (
	// HTTP RPC
	httpListener     net.Listener
	httpHandler      *rpc.Server
	httpEndpoint     = ""
	wsEndpoint       = ""
	httpModules      = []string{"hmy", "hmyv2", "net", "netv2", "explorer"}
	httpVirtualHosts = []string{"*"}
	httpTimeouts     = rpc.DefaultHTTPTimeouts
	httpOrigins      = []string{"*"}
	wsModules        = []string{"hmy", "hmyv2", "net", "netv2", "web3"}
	wsOrigins        = []string{"*"}
	harmony          *hmy.Harmony
)

// IsCurrentlyLeader exposes if node is currently the leader node
func (node *Node) IsCurrentlyLeader() bool {
	return node.Consensus.IsLeader()
}

// PeerConnectivity ..
func (node *Node) PeerConnectivity() (int, int, int) {
	return node.host.C()
}

// ListPeer return list of peers for a certain topic
func (node *Node) ListPeer(topic string) []peer.ID {
	return node.host.ListPeer(topic)
}

// ListTopic return list of topics the node subscribed
func (node *Node) ListTopic() []string {
	return node.host.ListTopic()
}

// ListBlockedPeer return list of blocked peers
func (node *Node) ListBlockedPeer() []peer.ID {
	return node.host.ListBlockedPeer()
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

// ReportStakingErrorSink is the report of failed staking transactions this node has (held in memory only)
func (node *Node) ReportStakingErrorSink() types.TransactionErrorReports {
	return node.TransactionErrorSink.StakingReport()
}

// GetNodeBootTime ..
func (node *Node) GetNodeBootTime() int64 {
	return node.unixTimeAtNodeStart
}

// ReportPlainErrorSink is the report of failed transactions this node has (held in memory only)
func (node *Node) ReportPlainErrorSink() types.TransactionErrorReports {
	return node.TransactionErrorSink.PlainReport()
}

// StartRPC start RPC service
func (node *Node) StartRPC(nodePort string) error {
	// Gather all the possible APIs to surface
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
func (node *Node) startWS(
	endpoint string, apis []rpc.API, modules []string, wsOrigins []string, exposeAll bool,
) error {
	// Short circuit if the WS endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}
	listener, _, err := rpc.StartWSEndpoint(endpoint, apis, modules, wsOrigins, exposeAll)
	if err != nil {
		return err
	}
	utils.Logger().Info().
		Str("url", fmt.Sprintf("ws://%s", listener.Addr())).
		Msg("WebSocket endpoint opened")
	return nil
}

// APIs return the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (node *Node) APIs() []rpc.API {
	if harmony == nil {
		harmony = hmy.New(node, node.TxPool, node.CxPool, node.Consensus.ShardID)
	}
	// Gather all the possible APIs to surface
	apis := hmyapi.GetAPIs(harmony)
	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "hmy",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(harmony, false),
			Public:    true,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   apiv1.NewPublicNetAPI(node.host, harmony.ChainID),
			Public:    true,
		},
		{
			Namespace: "netv2",
			Version:   "1.0",
			Service:   apiv2.NewPublicNetAPI(node.host, harmony.ChainID),
			Public:    true,
		},
	}...)
}
