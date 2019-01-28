// bootnode provides peer discovery service to new node to connect to the p2p network

package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	libp2p "github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"
)

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

func loggingInit(logFolder, ip, port string) {
	// Setup a logger to stdout and log file.
	if err := os.MkdirAll(logFolder, 0755); err != nil {
		panic(err)
	}
	logFileName := fmt.Sprintf("./%v/bootnode-%v-%v.log", logFolder, ip, port)
	h := log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
	)
	log.Root().SetHandler(h)
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9876", "port of the node.")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	keyFile := flag.String("key", "./.bnkey", "the private key file of the bootnode")
	versionFlag := flag.Bool("version", false, "Output version info")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Logging setup
	utils.SetPortAndIP(*port, *ip)

	// Init logging.
	loggingInit(*logFolder, *ip, *port)

	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", *port))
	if err != nil {
		panic(err)
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrs(listen),
	}
	privKey, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	var selfPeer = p2p.Peer{IP: *ip, Port: *port}

	host, err := p2pimpl.NewHost(&selfPeer, privKey, opts...)
	if err != nil {
		panic(err)
	}

	log.Info("bootnode", "BN_MA", fmt.Sprintf("/ipv/%s/tcp/%s/p2p/%s", *ip, *port, host.GetID().Pretty()))
}
