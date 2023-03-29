package rpc

import (
	"fmt"
	"net"
	"strings"

	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/configs/harmony"
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
	httpAuthEndpoint = ""
	wsEndpoint       = ""
	wsAuthEndpoint   = ""
	httpVirtualHosts = []string{"*"}
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
func StartServers(hmy *hmy.Harmony, apis []rpc.API, config nodeconfig.RPCServerConfig, rpcOpt harmony.RpcOptConfig) error {
	apis = append(apis, getAPIs(hmy, config)...)
	authApis := append(apis, getAuthAPIs(hmy, config.DebugEnabled, config.RateLimiterEnabled, config.RequestsPerSecond)...)
	// load method filter from file (if exist)
	var rmf rpc.RpcMethodFilter
	rpcFilterFilePath := strings.TrimSpace(rpcOpt.RpcFilterFile)
	if len(rpcFilterFilePath) > 0 {
		if err := rmf.LoadRpcMethodFiltersFromFile(rpcFilterFilePath); err != nil {
			return err
		}
	} else {
		rmf.ExposeAll()
	}
	if config.HTTPEnabled {
		timeouts := rpc.HTTPTimeouts{
			ReadTimeout:  config.HTTPTimeoutRead,
			WriteTimeout: config.HTTPTimeoutWrite,
			IdleTimeout:  config.HTTPTimeoutIdle,
		}
		httpEndpoint = fmt.Sprintf("%v:%v", config.HTTPIp, config.HTTPPort)
		if err := startHTTP(apis, &rmf, timeouts); err != nil {
			return err
		}

		httpAuthEndpoint = fmt.Sprintf("%v:%v", config.HTTPIp, config.HTTPAuthPort)
		if err := startAuthHTTP(authApis, &rmf, timeouts); err != nil {
			return err
		}
	}

	if config.WSEnabled {
		wsEndpoint = fmt.Sprintf("%v:%v", config.WSIp, config.WSPort)
		if err := startWS(apis, &rmf); err != nil {
			return err
		}

		wsAuthEndpoint = fmt.Sprintf("%v:%v", config.WSIp, config.WSAuthPort)
		if err := startAuthWS(authApis, &rmf); err != nil {
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

func getAuthAPIs(hmy *hmy.Harmony, debugEnable bool, rateLimiterEnable bool, ratelimit int) []rpc.API {
	return []rpc.API{
		NewPublicTraceAPI(hmy, Debug), // Debug version means geth trace rpc
		NewPublicTraceAPI(hmy, Trace), // Trace version means parity trace rpc
	}
}

// getAPIs returns all the API methods for the RPC interface
func getAPIs(hmy *hmy.Harmony, config nodeconfig.RPCServerConfig) []rpc.API {
	publicAPIs := []rpc.API{
		// Public methods
		NewPublicHarmonyAPI(hmy, V1),
		NewPublicHarmonyAPI(hmy, V2),
		NewPublicBlockchainAPI(hmy, V1, config.RateLimiterEnabled, config.RequestsPerSecond),
		NewPublicBlockchainAPI(hmy, V2, config.RateLimiterEnabled, config.RequestsPerSecond),
		NewPublicContractAPI(hmy, V1, config.RateLimiterEnabled, config.RequestsPerSecond, config.EvmCallTimeout),
		NewPublicContractAPI(hmy, V2, config.RateLimiterEnabled, config.RequestsPerSecond, config.EvmCallTimeout),
		NewPublicTransactionAPI(hmy, V1),
		NewPublicTransactionAPI(hmy, V2),
		NewPublicPoolAPI(hmy, V1, config.RateLimiterEnabled, config.RequestsPerSecond),
		NewPublicPoolAPI(hmy, V2, config.RateLimiterEnabled, config.RequestsPerSecond),
	}

	// Legacy methods (subject to removal)
	if config.LegacyRPCsEnabled {
		publicAPIs = append(publicAPIs,
			v1.NewPublicLegacyAPI(hmy, "hmy"),
			v2.NewPublicLegacyAPI(hmy, "hmyv2"),
		)
	}

	if config.StakingRPCsEnabled {
		publicAPIs = append(publicAPIs,
			NewPublicStakingAPI(hmy, V1),
			NewPublicStakingAPI(hmy, V2),
		)
	}

	if config.EthRPCsEnabled {
		publicAPIs = append(publicAPIs,
			NewPublicHarmonyAPI(hmy, Eth),
			NewPublicBlockchainAPI(hmy, Eth, config.RateLimiterEnabled, config.RequestsPerSecond),
			NewPublicContractAPI(hmy, Eth, config.RateLimiterEnabled, config.RequestsPerSecond, config.EvmCallTimeout),
			NewPublicTransactionAPI(hmy, Eth),
			NewPublicPoolAPI(hmy, Eth, config.RateLimiterEnabled, config.RequestsPerSecond),
			eth.NewPublicEthService(hmy, "eth"),
		)
	}

	publicDebugAPIs := []rpc.API{
		//Public debug API
		NewPublicDebugAPI(hmy, V1),
		NewPublicDebugAPI(hmy, V2),
	}

	privateAPIs := []rpc.API{
		NewPrivateDebugAPI(hmy, V1),
		NewPrivateDebugAPI(hmy, V2),
	}

	if config.DebugEnabled {
		apis := append(publicAPIs, publicDebugAPIs...)
		return append(apis, privateAPIs...)
	}
	return publicAPIs
}

func startHTTP(apis []rpc.API, rmf *rpc.RpcMethodFilter, httpTimeouts rpc.HTTPTimeouts) (err error) {
	httpListener, httpHandler, err = rpc.StartHTTPEndpoint(
		httpEndpoint, apis, HTTPModules, rmf, httpOrigins, httpVirtualHosts, httpTimeouts,
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

func startAuthHTTP(apis []rpc.API, rmf *rpc.RpcMethodFilter, httpTimeouts rpc.HTTPTimeouts) (err error) {
	httpListener, httpHandler, err = rpc.StartHTTPEndpoint(
		httpAuthEndpoint, apis, HTTPModules, rmf, httpOrigins, httpVirtualHosts, httpTimeouts,
	)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("http://%s", httpAuthEndpoint)).
		Str("cors", strings.Join(httpOrigins, ",")).
		Str("vhosts", strings.Join(httpVirtualHosts, ",")).
		Msg("HTTP endpoint opened")
	fmt.Printf("Started Auth-RPC server at: %v\n", httpAuthEndpoint)
	return nil
}

func startWS(apis []rpc.API, rmf *rpc.RpcMethodFilter) (err error) {
	wsListener, wsHandler, err = rpc.StartWSEndpoint(wsEndpoint, apis, WSModules, rmf, wsOrigins, true)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("ws://%s", wsListener.Addr())).
		Msg("WebSocket endpoint opened")
	fmt.Printf("Started WS server at: %v\n", wsEndpoint)
	return nil
}

func startAuthWS(apis []rpc.API, rmf *rpc.RpcMethodFilter) (err error) {
	wsListener, wsHandler, err = rpc.StartWSEndpoint(wsAuthEndpoint, apis, WSModules, rmf, wsOrigins, true)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("ws://%s", wsListener.Addr())).
		Msg("WebSocket endpoint opened")
	fmt.Printf("Started Auth-WS server at: %v\n", wsAuthEndpoint)
	return nil
}
