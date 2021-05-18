package rpc

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	eth "github.com/harmony-one/harmony/rpc/eth"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
)

// Version enum
const (
	V1 Version = iota
	V2
	Eth
	Debug
	Trace
)

const (
	// APIVersion used for DApp's, bumped after RPC refactor (7/2020)
	APIVersion = "1.1"
	// CallTimeout is the timeout given to all contract calls
	CallTimeout = 5 * time.Second
	// LogTag is the tag found in the log for all RPC logs
	LogTag = "[RPC]"
	// HTTPPortOffset ..
	HTTPPortOffset = 500
	// WSPortOffset ..
	WSPortOffset = 800

	netNamespace   = "net"
	netV1Namespace = "netv1"
	netV2Namespace = "netv2"
	web3Namespace  = "web3"
)

var (
	// HTTPModules ..
	HTTPModules = []string{"hmy", "hmyv2", "eth", "debug", "trace", netNamespace, netV1Namespace, netV2Namespace, web3Namespace, "explorer"}
	// WSModules ..
	WSModules = []string{"hmy", "hmyv2", "eth", "debug", "trace", netNamespace, netV1Namespace, netV2Namespace, web3Namespace, "web3"}

	httpListener     net.Listener
	httpHandler      *rpc.Server
	wsListener       net.Listener
	wsHandler        *rpc.Server
	httpEndpoint     = ""
	wsEndpoint       = ""
	httpVirtualHosts = []string{"*"}
	httpTimeouts     = rpc.DefaultHTTPTimeouts
	httpOrigins      = []string{"*"}
	wsOrigins        = []string{"*"}
)

// Version of the RPC
type Version int

// Namespace of the RPC version
func (n Version) Namespace() string {
	return HTTPModules[n]
}

// StartServers starts the http & ws servers
func StartServers(hmy *hmy.Harmony, apis []rpc.API, config nodeconfig.RPCServerConfig) error {
	apis = append(apis, getAPIs(hmy, config.DebugEnabled, config.RateLimiterEnabled, config.RequestsPerSecond)...)

	if config.HTTPEnabled {
		httpEndpoint = fmt.Sprintf("%v:%v", config.HTTPIp, config.HTTPPort)
		if err := startHTTP(apis); err != nil {
			return err
		}
	}

	if config.WSEnabled {
		wsEndpoint = fmt.Sprintf("%v:%v", config.WSIp, config.WSPort)
		if err := startWS(apis); err != nil {
			return err
		}
	}

	return nil
}

// StopServers stops the http & ws servers
func StopServers() error {
	if httpListener != nil {
		if err := httpListener.Close(); err != nil {
			return err
		}
		httpListener = nil
		utils.Logger().Info().
			Str("url", fmt.Sprintf("http://%s", httpEndpoint)).
			Msg("HTTP endpoint closed")
	}
	if httpHandler != nil {
		httpHandler.Stop()
		httpHandler = nil
	}
	if wsListener != nil {
		if err := wsListener.Close(); err != nil {
			return err
		}
		wsListener = nil
		utils.Logger().Info().
			Str("url", fmt.Sprintf("http://%s", wsEndpoint)).
			Msg("WS endpoint closed")
	}
	if wsHandler != nil {
		wsHandler.Stop()
		wsHandler = nil
	}
	return nil
}

// getAPIs returns all the API methods for the RPC interface
func getAPIs(hmy *hmy.Harmony, debugEnable bool, rateLimiterEnable bool, ratelimit int) []rpc.API {
	publicAPIs := []rpc.API{
		// Public methods
		NewPublicHarmonyAPI(hmy, V1),
		NewPublicHarmonyAPI(hmy, V2),
		NewPublicHarmonyAPI(hmy, Eth),
		NewPublicBlockchainAPI(hmy, V1, rateLimiterEnable, ratelimit),
		NewPublicBlockchainAPI(hmy, V2, rateLimiterEnable, ratelimit),
		NewPublicBlockchainAPI(hmy, Eth, rateLimiterEnable, ratelimit),
		NewPublicContractAPI(hmy, V1),
		NewPublicContractAPI(hmy, V2),
		NewPublicContractAPI(hmy, Eth),
		NewPublicTransactionAPI(hmy, V1),
		NewPublicTransactionAPI(hmy, V2),
		NewPublicTransactionAPI(hmy, Eth),
		NewPublicPoolAPI(hmy, V1),
		NewPublicPoolAPI(hmy, V2),
		NewPublicPoolAPI(hmy, Eth),
		NewPublicStakingAPI(hmy, V1),
		NewPublicStakingAPI(hmy, V2),
		NewPublicTraceAPI(hmy, Debug), // Debug version means geth trace rpc
		NewPublicTraceAPI(hmy, Trace), // Trace version means parity trace rpc
		// Legacy methods (subject to removal)
		v1.NewPublicLegacyAPI(hmy, "hmy"),
		eth.NewPublicEthService(hmy, "eth"),
		v2.NewPublicLegacyAPI(hmy, "hmyv2"),
	}

	privateAPIs := []rpc.API{
		NewPrivateDebugAPI(hmy, V1),
		NewPrivateDebugAPI(hmy, V2),
	}

	if debugEnable {
		return append(publicAPIs, privateAPIs...)
	}
	return publicAPIs
}

func startHTTP(apis []rpc.API) (err error) {
	httpListener, httpHandler, err = rpc.StartHTTPEndpoint(
		httpEndpoint, apis, HTTPModules, httpOrigins, httpVirtualHosts, httpTimeouts,
	)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("http://%s", httpEndpoint)).
		Str("cors", strings.Join(httpOrigins, ",")).
		Str("vhosts", strings.Join(httpVirtualHosts, ",")).
		Msg("HTTP endpoint opened")
	fmt.Printf("Started RPC server at: %v\n", httpEndpoint)
	return nil
}

func startWS(apis []rpc.API) (err error) {
	wsListener, wsHandler, err = rpc.StartWSEndpoint(wsEndpoint, apis, WSModules, wsOrigins, true)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("ws://%s", wsListener.Addr())).
		Msg("WebSocket endpoint opened")
	fmt.Printf("Started WS server at: %v\n", wsEndpoint)
	return nil
}
