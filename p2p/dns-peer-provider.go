package p2p

import (
	"fmt"
	"net"
	"sync/atomic"
)

// DNSPeerProvider ..
type DNSPeerProvider struct {
	zone  string
	port  string
	peers atomic.Value
}

func (p *DNSPeerProvider) lookupHost(host string) ([]string, error) {
	return net.LookupHost(host)
}

// Peers ..
func (p *DNSPeerProvider) Peers() []Peer {
	return append([]Peer{}, p.peers.Load().([]Peer)...)
}

// ResolveAndUpdatePeers ..
func (p *DNSPeerProvider) ResolveAndUpdatePeers(shards []uint32) error {
	for _, shardID := range shards {
		dns := fmt.Sprintf("s%d.%s", shardID, p.zone)
		addrs, err := p.lookupHost(dns)
		if err != nil {
			return err
		}
		for _, addr := range addrs {
			fmt.Println("peer resolved to addr->", addr)
		}
	}

	return nil
}

// NewDNS ..
func NewDNS(zone, port string) *DNSPeerProvider {
	var cachedPeers atomic.Value
	cachedPeers.Store([]Peer{})

	return &DNSPeerProvider{
		zone:  zone,
		port:  port,
		peers: cachedPeers,
	}
}
