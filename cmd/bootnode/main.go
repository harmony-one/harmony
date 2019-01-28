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
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	ma "github.com/multiformats/go-multiaddr"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

type privKey struct {
	Key string `json:"key"`
}

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
	privKey, err := loadKey(*keyFile)
	if err != nil {
		panic(err)
	}

	opts = append(opts, libp2p.Identity(privKey))

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		panic(err)
	}

	log.Info("bootnode", "BN_MA", fmt.Sprintf("/ipv/%s/tcp/%s/p2p/%s", *ip, *port, host.ID().Pretty()))
}

// saveKey save private key to keyfile
func saveKey(keyfile string, key crypto.PrivKey) (err error) {
	str, err := utils.SavePrivateKey(key)
	if err != nil {
		return
	}

	keyStruct := privKey{Key: str}

	err = utils.Save(keyfile, &keyStruct)
	return
}

// loadKey load private key from keyfile
func loadKey(keyfile string) (key crypto.PrivKey, err error) {
	var keyStruct privKey
	err = utils.Load(keyfile, &keyStruct)
	if err != nil {
		log.Warn("No priviate key can be loaded from file", "keyfile", keyfile)
		log.Warn("Using random private key")
		key, _, err = utils.GenKeyP2PRand()
		if err != nil {
			panic(err)
		}
		err = saveKey(keyfile, key)
		if err != nil {
			log.Warn("bootnode", "failed to save key to keyfile", err)
		}
		return key, nil
	}
	key, err = utils.LoadPrivateKey(keyStruct.Key)
	return key, err
}
