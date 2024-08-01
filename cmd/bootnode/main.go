// bootnode provides peer discovery service to new node to connect to the p2p network

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	net "github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnLogger ..
type ConnLogger struct {
	l log.Logger
}

func netLogger(n net.Network, l log.Logger) log.Logger {
	return l.New(
		"netLocalPeer", n.LocalPeer(),
		"netListenAddresses", n.ListenAddresses())
}

func connLogger(c net.Conn, l log.Logger) log.Logger {
	return l.New(
		"connLocalPeer", c.LocalPeer(),
		"connLocalAddr", c.LocalMultiaddr(),
		"connRemotePeer", c.RemotePeer(),
		"connRemoteAddr", c.RemoteMultiaddr())
}

func streamLogger(s net.Stream, l log.Logger) log.Logger {
	return connLogger(s.Conn(), l).New("streamProtocolID", s.Protocol())
}

// Listen logs a listener starting listening on an address.
func (cl ConnLogger) Listen(n net.Network, ma ma.Multiaddr) {
	utils.WithCaller(netLogger(n, cl.l)).
		Debug("listener starting", "listenAddress", ma)
}

// ListenClose logs a listener stopping listening on an address.
func (cl ConnLogger) ListenClose(n net.Network, ma ma.Multiaddr) {
	utils.WithCaller(netLogger(n, cl.l)).
		Debug("listener closing", "listenAddress", ma)
}

// Connected logs a connection getting opened.
func (cl ConnLogger) Connected(n net.Network, c net.Conn) {
	utils.WithCaller(connLogger(c, netLogger(n, cl.l))).Debug("connected")
}

// Disconnected logs a connection getting closed.
func (cl ConnLogger) Disconnected(n net.Network, c net.Conn) {
	utils.WithCaller(connLogger(c, netLogger(n, cl.l))).Debug("disconnected")
}

// OpenedStream logs a new stream getting opened.
func (cl ConnLogger) OpenedStream(n net.Network, s net.Stream) {
	utils.WithCaller(streamLogger(s, netLogger(n, cl.l))).Debug("stream opened")
}

// ClosedStream logs a stream getting closed.
func (cl ConnLogger) ClosedStream(n net.Network, s net.Stream) {
	utils.WithCaller(streamLogger(s, netLogger(n, cl.l))).Debug("stream closed")
}

// NewConnLogger returns a new connection logger that uses the given
// Ethereum logger.  See ConnLogger for usage.
func NewConnLogger(l log.Logger) *ConnLogger {
	return &ConnLogger{l: l}
}

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2019. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func main() {
	timestamp := time.Now().Format("20060102150405")
	defUserAgent := fmt.Sprintf("bootnode-%s", timestamp)

	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9876", "port of the node.")
	console := flag.Bool("console_only", false, "Output to console only")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	logMaxSize := flag.Int("log_max_size", 100, "the max size in megabytes of the log file before it gets rotated")
	logRotateCount := flag.Int("log_rotate_count", 0, "the number of rotated logs to keep. If set to 0 rotation is disabled")
	logRotateMaxAge := flag.Int("log_rotate_max_age", 0, "the maximum number of days to retain old logs. If set to 0 rotation is disabled")
	keyFile := flag.String("key", "./.bnkey", "the private key file of the bootnode")
	versionFlag := flag.Bool("version", false, "Output version info")
	verbosity := flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")
	logConn := flag.Bool("log_conn", false, "log incoming/outgoing connections")
	maxConnPerIP := flag.Int("max_conn_per_ip", 10, "max connections number for same ip")
	forceReachabilityPublic := flag.Bool("force_public", false, "forcing the local node to believe it is reachable externally")
	noTransportSecurity := flag.Bool("no_transport_security", true, "disable TLS encrypted transport")
	muxer := flag.String("muxer", "mplex, yamux", "protocol muxer to mux per-protocol streams (mplex, yamux)")
	userAgent := flag.String("user_agent", defUserAgent, "explicitly set the user-agent, so we can differentiate from other Go libp2p users")
	noRelay := flag.Bool("no_relay", true, "no relay services, direct connections between peers only")

	pprof := flag.Bool("pprof", false, "enabled pprof")
	pprofAddr := flag.String("pprof.addr", "127.0.0.1:6060", "http pprof address")
	//keyFile := flag.String("pprof.profile.names", "", "the private key file of the bootnode")
	//keyFile := flag.String("pprof.profile.intervals", "600", "the private key file of the bootnode")
	//keyFile := flag.String("pprof.profile.intervals", "", "the private key file of the bootnode")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Logging setup
	utils.SetLogContext(*port, *ip)
	utils.SetLogVerbosity(log.Lvl(*verbosity))
	if *console != true {
		utils.AddLogFile(fmt.Sprintf("%v/bootnode-%v-%v.log", *logFolder, *ip, *port), *logMaxSize, *logRotateCount, *logRotateMaxAge)
	}

	privKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		utils.FatalErrMsg(err, "cannot load key from %s", *keyFile)
	}

	// For bootstrap nodes, we shall keep .dht file.
	dataStorePath := fmt.Sprintf(".dht-%s-%s", *ip, *port)
	selfPeer := p2p.Peer{IP: *ip, Port: *port}

	host, err := p2p.NewHost(p2p.HostConfig{
		Self:                    &selfPeer,
		BLSKey:                  privKey,
		BootNodes:               nil, // Boot nodes have no boot nodes :) Will be connected when other nodes joined
		DataStoreFile:           &dataStorePath,
		MaxConnPerIP:            *maxConnPerIP,
		ForceReachabilityPublic: *forceReachabilityPublic,
		NoTransportSecurity:     *noTransportSecurity,
		NAT:                     true,
		UserAgent:               *userAgent,
		DialTimeout:             time.Minute,
		Muxer:                   *muxer,
		NoRelay:                 *noRelay,
	})
	if err != nil {
		utils.FatalErrMsg(err, "cannot initialize network")
	}

	fmt.Printf("bootnode BN_MA=%s\n",
		fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, host.GetID().String()),
	)

	host.Start()

	if *logConn {
		host.GetP2PHost().Network().Notify(NewConnLogger(utils.GetLogInstance()))
	}

	if *pprof {
		fmt.Printf("starting pprof on http://%s/debug/pprof/\n", *pprofAddr)
		http.ListenAndServe(*pprofAddr, nil)
	}

	select {}
}
