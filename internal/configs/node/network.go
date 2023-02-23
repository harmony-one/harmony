package nodeconfig

var (
	mainnetBootNodes = []string{
		"/dnsaddr/bootstrap.t.hmny.io",
	}

	testnetBootNodes = []string{
		"/dnsaddr/bootstrap.b.hmny.io",
	}

	pangaeaBootNodes = []string{
		"/ip4/52.40.84.2/tcp/9800/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
		"/ip4/54.86.126.90/tcp/9800/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
	}

	partnerBootNodes = []string{
		"/dnsaddr/bootstrap.ps.hmny.io",
	}

	stressBootNodes = []string{
		"/dnsaddr/bootstrap.stn.hmny.io",
	}

	devnetBootNodes = []string{}
)

const (
	mainnetDNSZone   = "t.hmny.io"
	testnetDNSZone   = "b.hmny.io"
	pangaeaDNSZone   = "os.hmny.io"
	partnerDNSZone   = "ps.hmny.io"
	stressnetDNSZone = "stn.hmny.io"
)

const (
	// DefaultLocalListenIP is the IP used for local hosting
	DefaultLocalListenIP = "127.0.0.1"
	// DefaultPublicListenIP is the IP used for public hosting
	DefaultPublicListenIP = "0.0.0.0"
	// DefaultP2PPort is the key to be used for p2p communication
	DefaultP2PPort = 9000
	// DefaultLegacyDNSPort is the default legacy DNS port. The actual port used is DNSPort - 3000. This is a
	// very bad design. Refactored to DefaultDNSPort
	DefaultLegacyDNSPort = 9000
	// DefaultDNSPort is the default DNS port for both remote node and local server.
	DefaultDNSPort = 6000
	// DefaultRPCPort is the default rpc port. The actual port used is 9000+500
	DefaultRPCPort = 9500
	// DefaultAuthRPCPort is the default rpc auth port. The actual port used is 9000+501
	DefaultAuthRPCPort = 9501
	// DefaultRosettaPort is the default rosetta port. The actual port used is 9000+700
	DefaultRosettaPort = 9700
	// DefaultHTTP timeouts - read, write, and idle
	DefaultHTTPTimeoutRead  = "30s"
	DefaultHTTPTimeoutWrite = "30s"
	DefaultHTTPTimeoutIdle  = "120s"
	// DefaultEvmCallTimeout is the default timeout for evm call
	DefaultEvmCallTimeout = "5s"
	// DefaultWSPort is the default port for web socket endpoint. The actual port used is
	DefaultWSPort = 9800
	// DefaultAuthWSPort is the default port for web socket auth endpoint. The actual port used is
	DefaultAuthWSPort = 9801
	// DefaultPrometheusPort is the default prometheus port. The actual port used is 9000+900
	DefaultPrometheusPort = 9900
	// DefaultP2PConcurrency is the default P2P concurrency, 0 means is set the default value of P2P Discovery, the actual value is 10
	DefaultP2PConcurrency = 0
	// DefaultMaxConnPerIP is the maximum number of connections to/from a remote IP
	DefaultMaxConnPerIP = 10
	// DefaultMaxPeers is the maximum number of remote peers, with 0 representing no limit
	DefaultMaxPeers = 0
	// DefaultConnManagerLowWatermark is the lowest number of connections that'll be maintained in connection manager
	DefaultConnManagerLowWatermark = 160
	// DefaultConnManagerHighWatermark is the highest number of connections that'll be maintained in connection manager
	// When the peer count exceeds the 'high watermark', as many peers will be pruned (and
	// their connections terminated) until 'low watermark' peers remain.
	DefaultConnManagerHighWatermark = 192
	// DefaultWaitForEachPeerToConnect sets the sync configs to connect to neighbor peers one by one and waits for each peer to connect.
	DefaultWaitForEachPeerToConnect = false
)

const (
	// DefaultRateLimit for RPC, the number of requests per second
	DefaultRPCRateLimit = 1000
)

const (
	// rpcHTTPPortOffset is the port offset for RPC HTTP requests
	rpcHTTPPortOffset = 500

	// rpcHTTPAuthPortOffset is the port offset for RPC Auth HTTP requests
	rpcHTTPAuthPortOffset = 501

	// rpcHTTPPortOffset is the port offset for rosetta HTTP requests
	rosettaHTTPPortOffset = 700

	// rpcWSPortOffSet is the port offset for RPC websocket requests
	rpcWSPortOffSet = 800

	// rpcWSAuthPortOffSet is the port offset for RPC Auth websocket requests
	rpcWSAuthPortOffSet = 801

	// prometheusHTTPPortOffset is the port offset for prometheus HTTP requests
	prometheusHTTPPortOffset = 900
)

// GetDefaultBootNodes get the default bootnode with the given network type
func GetDefaultBootNodes(networkType NetworkType) []string {
	switch networkType {
	case Mainnet:
		return mainnetBootNodes
	case Testnet:
		return testnetBootNodes
	case Pangaea:
		return pangaeaBootNodes
	case Partner:
		return partnerBootNodes
	case Stressnet:
		return stressBootNodes
	case Devnet:
		return devnetBootNodes
	}
	return nil
}

// GetDefaultDNSZone get the default DNS zone with the given network type
func GetDefaultDNSZone(networkType NetworkType) string {
	switch networkType {
	case Mainnet:
		return mainnetDNSZone
	case Testnet:
		return testnetDNSZone
	case Pangaea:
		return pangaeaDNSZone
	case Partner:
		return partnerDNSZone
	case Stressnet:
		return stressnetDNSZone
	}
	return ""
}

// GetDefaultDNSPort get the default DNS port for the given network type
func GetDefaultDNSPort(NetworkType) int {
	return DefaultDNSPort
}

// GetRPCHTTPPortFromBase return the rpc HTTP port from base port
func GetRPCHTTPPortFromBase(basePort int) int {
	return basePort + rpcHTTPPortOffset
}

// GetRPCAuthHTTPPortFromBase return the rpc HTTP port from base port
func GetRPCAuthHTTPPortFromBase(basePort int) int {
	return basePort + rpcHTTPAuthPortOffset
}

// GetRosettaHTTPPortFromBase return the rosetta HTTP port from base port
func GetRosettaHTTPPortFromBase(basePort int) int {
	return basePort + rosettaHTTPPortOffset
}

// GetWSPortFromBase return the Websocket port from the base port
func GetWSPortFromBase(basePort int) int {
	return basePort + rpcWSPortOffSet
}

// GetWSAuthPortFromBase return the Websocket port from the base auth port
func GetWSAuthPortFromBase(basePort int) int {
	return basePort + rpcWSAuthPortOffSet
}

// GetPrometheusHTTPPortFromBase return the prometheus HTTP port from base port
func GetPrometheusHTTPPortFromBase(basePort int) int {
	return basePort + prometheusHTTPPortOffset
}
