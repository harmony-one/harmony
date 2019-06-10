package utils

import (
	"fmt"
	"strings"

	p2p "github.com/harmony-one/harmony/p2p"
	ma "github.com/multiformats/go-multiaddr"
)

// AddrList is a list of multiaddress
type AddrList []ma.Multiaddr

// String is a function to print a string representation of the AddrList
func (al *AddrList) String() string {
	strs := make([]string, len(*al))
	for i, addr := range *al {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ",")
}

// Set is a function to set the value of AddrList based on a string
func (al *AddrList) Set(value string) error {
	if len(*al) > 0 {
		return fmt.Errorf("AddrList is already set")
	}
	for _, a := range strings.Split(value, ",") {
		addr, err := ma.NewMultiaddr(a)
		if err != nil {
			return err
		}
		*al = append(*al, addr)
	}
	return nil
}

// StringsToAddrs convert a list of strings to a list of multiaddresses
func StringsToAddrs(addrStrings []string) (maddrs []ma.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := ma.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

// StringsToPeers converts a string to a list of Peers
// addr is a string of format "ip:port,ip:port"
func StringsToPeers(input string) []p2p.Peer {
	addrs := strings.Split(input, ",")
	peers := make([]p2p.Peer, 0)
	for _, addr := range addrs {
		data := strings.Split(addr, ":")
		if len(data) >= 2 {
			peer := p2p.Peer{}
			peer.IP = data[0]
			peer.Port = data[1]
			peers = append(peers, peer)
		}
	}
	return peers
}

// DefaultBootNodeAddrStrings is a list of Harmony bootnodes address. Used to find other peers in the network.
var DefaultBootNodeAddrStrings = []string{
   "/ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv",
}

// BootNodes is a list of boot nodes. It is populated either from default or from user CLI input.
var BootNodes AddrList
