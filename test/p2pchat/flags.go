package main

import (
	"flag"
	"strings"

	maddr "github.com/multiformats/go-multiaddr"
)

// A new type we need for writing a custom flag parser
type addrList []maddr.Multiaddr

func (al *addrList) String() string {
	strs := make([]string, len(*al))
	for i, addr := range *al {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ",")
}

func (al *addrList) Set(value string) error {
	addr, err := maddr.NewMultiaddr(value)
	if err != nil {
		return err
	}
	*al = append(*al, addr)
	return nil
}

// Harmony test bootstrap nodes. Used to find other peers in the network.
var defaultBootstrapAddrStrings = []string{
	"/ip4/127.0.0.1/tcp/9876/p2p/QmayB8NwxmfGE4Usb4H61M8uwbfc7LRbmXb3ChseJgbVuf",
}

// StringsToAddrs ...
func StringsToAddrs(addrStrings []string) (maddrs []maddr.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := maddr.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

// Config ...
type Config struct {
	RendezvousString string
	BootstrapPeers   addrList
	ListenAddresses  addrList
	ProtocolID       string
	PubSubImpl       string
}

// ParseFlags ...
func ParseFlags() (Config, error) {
	config := Config{}
	flag.StringVar(&config.RendezvousString, "rendezvous", "meet me here",
		"Unique string to identify group of nodes. Share this with your friends to let them connect with you")
	flag.Var(&config.BootstrapPeers, "peer", "Adds a peer multiaddress to the bootstrap list")
	flag.Var(&config.ListenAddresses, "listen", "Adds a multiaddress to the listen list")
	flag.StringVar(&config.ProtocolID, "pid", "/chat/1.1.0", "Sets a protocol id for stream headers")
	flag.StringVar(&config.PubSubImpl, "pubsub", "gossip", "Set the pubsub implementation: gossip, flood")
	flag.Parse()

	if len(config.BootstrapPeers) == 0 {
		bootstrapPeerAddrs, err := StringsToAddrs(defaultBootstrapAddrStrings)
		if err != nil {
			return config, err
		}
		config.BootstrapPeers = bootstrapPeerAddrs
	}

	return config, nil
}
