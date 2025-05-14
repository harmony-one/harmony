package bootnode

import (
	"reflect"
	"strings"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
)

// BootNodeConfig contains all the configs user can set for running harmony binary. Served as the bridge
// from user set flags to internal node configs. Also user can persist this structure to a toml file
// to avoid inputting all arguments.
type BootNodeConfig struct {
	Version string
	General GeneralConfig
	P2P     P2pConfig
	HTTP    HttpConfig
	WS      WsConfig
	RPCOpt  RpcOptConfig
	Pprof   PprofConfig
	Log     LogConfig
	Sys     *SysConfig `toml:",omitempty"`
}

func (bnc BootNodeConfig) ToRPCServerConfig() nodeconfig.RPCServerConfig {
	readTimeout, err := time.ParseDuration(bnc.HTTP.ReadTimeout)
	if err != nil {
		readTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutRead)
		utils.Logger().Warn().
			Str("provided", bnc.HTTP.ReadTimeout).
			Dur("updated", readTimeout).
			Msg("Sanitizing invalid http read timeout")
	}
	writeTimeout, err := time.ParseDuration(bnc.HTTP.WriteTimeout)
	if err != nil {
		writeTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutWrite)
		utils.Logger().Warn().
			Str("provided", bnc.HTTP.WriteTimeout).
			Dur("updated", writeTimeout).
			Msg("Sanitizing invalid http write timeout")
	}
	idleTimeout, err := time.ParseDuration(bnc.HTTP.IdleTimeout)
	if err != nil {
		idleTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutIdle)
		utils.Logger().Warn().
			Str("provided", bnc.HTTP.IdleTimeout).
			Dur("updated", idleTimeout).
			Msg("Sanitizing invalid http idle timeout")
	}
	return nodeconfig.RPCServerConfig{
		HTTPEnabled:        bnc.HTTP.Enabled,
		HTTPIp:             bnc.HTTP.IP,
		HTTPPort:           bnc.HTTP.Port,
		HTTPTimeoutRead:    readTimeout,
		HTTPTimeoutWrite:   writeTimeout,
		HTTPTimeoutIdle:    idleTimeout,
		WSEnabled:          bnc.WS.Enabled,
		WSIp:               bnc.WS.IP,
		WSPort:             bnc.WS.Port,
		DebugEnabled:       bnc.RPCOpt.DebugEnabled,
		EthRPCsEnabled:     bnc.RPCOpt.EthRPCsEnabled,
		LegacyRPCsEnabled:  bnc.RPCOpt.LegacyRPCsEnabled,
		RpcFilterFile:      bnc.RPCOpt.RpcFilterFile,
		RateLimiterEnabled: bnc.RPCOpt.RateLimterEnabled,
		RequestsPerSecond:  bnc.RPCOpt.RequestsPerSecond,
	}
}

type P2pConfig struct {
	Port                 int
	IP                   string
	KeyFile              string
	DHTDataStore         *string `toml:",omitempty"`
	DiscConcurrency      int     // Discovery Concurrency value
	MaxConnsPerIP        int
	DisablePrivateIPScan bool
	MaxPeers             int64
	// In order to disable Connection Manager, it only needs to
	// set both the high and low watermarks to zero. In this way,
	// using Connection Manager will be an optional feature.
	ConnManagerLowWatermark  int
	ConnManagerHighWatermark int
	WaitForEachPeerToConnect bool
	// to disable p2p security (tls and noise)
	NoTransportSecurity bool
	// enable p2p NAT. NAT Manager takes care of setting NAT port mappings, and discovering external addresses
	NAT bool
	// custom user agent; explicitly set the user-agent, so we can differentiate from other Go libp2p users
	UserAgent string
	// p2p dial timeout
	DialTimeout time.Duration
	// P2P multiplexer type, should be comma separated (mplex, mplexC6, yamux)
	Muxer string
	// No relay services, direct connections between peers only
	NoRelay bool
}

type GeneralConfig struct {
	NodeType    string
	ShardID     int
	TraceEnable bool
}

type PprofConfig struct {
	Enabled            bool
	ListenAddr         string
	Folder             string
	ProfileNames       []string
	ProfileIntervals   []int
	ProfileDebugValues []int
}

type LogConfig struct {
	Console       bool
	Folder        string
	FileName      string
	RotateSize    int
	RotateCount   int
	RotateMaxAge  int
	Verbosity     int
	VerbosePrints LogVerbosePrints
	Context       *LogContext `toml:",omitempty"`
}

type LogVerbosePrints struct {
	Config bool
}

func FlagSliceToLogVerbosePrints(verbosePrintsFlagSlice []string) LogVerbosePrints {
	verbosePrints := LogVerbosePrints{}
	verbosePrintsReflect := reflect.Indirect(reflect.ValueOf(&verbosePrints))
	for _, verbosePrint := range verbosePrintsFlagSlice {
		verbosePrint = strings.Title(verbosePrint)
		field := verbosePrintsReflect.FieldByName(verbosePrint)
		if field.IsValid() && field.CanSet() {
			field.SetBool(true)
		}
	}

	return verbosePrints
}

type LogContext struct {
	IP   string
	Port int
}

type SysConfig struct {
	NtpServer string
}

type HttpConfig struct {
	Enabled      bool
	IP           string
	Port         int
	AuthPort     int
	ReadTimeout  string
	WriteTimeout string
	IdleTimeout  string
}

type WsConfig struct {
	Enabled  bool
	IP       string
	Port     int
	AuthPort int
}

type RpcOptConfig struct {
	DebugEnabled      bool   // Enables PrivateDebugService APIs, including the EVM tracer
	EthRPCsEnabled    bool   // Expose Eth RPCs
	LegacyRPCsEnabled bool   // Expose Legacy RPCs
	RpcFilterFile     string // Define filters to enable/disable RPC exposure
	RateLimterEnabled bool   // Enable Rate limiter for RPC
	RequestsPerSecond int    // for RPC rate limiter
}

type PrometheusConfig struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool
	Gateway    string
}
