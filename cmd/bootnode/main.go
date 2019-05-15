// bootnode provides peer discovery service to new node to connect to the p2p network

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"

	kaddht "github.com/libp2p/go-libp2p-kad-dht"
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

func loggingInit(logFolder, ip, port string, verbosity log.Lvl) {
	// Setup a logger to stdout and log file.
	if err := os.MkdirAll(logFolder, 0755); err != nil {
		panic(err)
	}
	logFileName := fmt.Sprintf("./%v/bootnode-%v-%v.log", logFolder, ip, port)
	h := log.LvlFilterHandler(verbosity, log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
	))
	log.Root().SetHandler(h)
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9876", "port of the node.")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	keyFile := flag.String("key", "./.bnkey", "the private key file of the bootnode")
	versionFlag := flag.Bool("version", false, "Output version info")
	verbosity := flag.Int("verbosity", 3, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 3)")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Logging setup
	utils.SetPortAndIP(*port, *ip)
	utils.SetVerbosity(log.Lvl(*verbosity))

	// Init logging.
	loggingInit(*logFolder, *ip, *port, log.Lvl(*verbosity))

	privKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	var selfPeer = p2p.Peer{IP: *ip, Port: *port}

	host, err := p2pimpl.NewHost(&selfPeer, privKey)
	if err != nil {
		panic(err)
	}

	log.Info("bootnode", "BN_MA", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, host.GetID().Pretty()))

	dataStore := dsync.MutexWrap(ds.NewMapDatastore())
	dht := kaddht.NewDHT(context.Background(), host.GetP2PHost(), dataStore)

	if err := dht.Bootstrap(context.Background()); err != nil {
		log.Error("failed to bootstrap DHT")
		panic(err)
	}

	select {}
}
