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
	badger "github.com/ipfs/go-ds-badger"
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

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9876", "port of the node.")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	logMaxSize := flag.Int("log_max_size", 100, "the max size in megabytes of the log file before it gets rotated")
	keyFile := flag.String("key", "./.bnkey", "the private key file of the bootnode")
	versionFlag := flag.Bool("version", false, "Output version info")
	verbosity := flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")
	logConn := flag.Bool("log_conn", false, "log incoming/outgoing connections")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Logging setup
	utils.SetLogContext(*port, *ip)
	utils.SetLogVerbosity(log.Lvl(*verbosity))
	utils.AddLogFile(fmt.Sprintf("%v/bootnode-%v-%v.log", *logFolder, *ip, *port), *logMaxSize)

	privKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		utils.FatalErrMsg(err, "cannot load key from %s", *keyFile)
	}

	selfPeer := p2p.Peer{IP: *ip, Port: *port}

	host, err := p2pimpl.NewHost(&selfPeer, privKey)
	if err != nil {
		utils.FatalErrMsg(err, "cannot initialize network")
	}

	fmt.Printf("bootnode BN_MA=%s", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, host.GetID().Pretty()))

	if *logConn {
		host.GetP2PHost().Network().Notify(utils.NewConnLogger(utils.GetLogInstance()))
	}

	// set the KValue to 50 for DHT
	// 50 is the size of every bucket in the DHT
	kaddht.KValue = 50
	dataStorePath := fmt.Sprintf(".dht-%s-%s", *ip, *port)
	dataStore, err := badger.NewDatastore(dataStorePath, nil)
	if err != nil {
		utils.FatalErrMsg(err, "cannot initialize DHT cache at %s", dataStorePath)
	}
	dht := kaddht.NewDHT(context.Background(), host.GetP2PHost(), dataStore)

	if err := dht.Bootstrap(context.Background()); err != nil {
		utils.FatalErrMsg(err, "cannot bootstrap DHT")
	}

	select {}
}
