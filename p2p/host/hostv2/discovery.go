package hostv2

import (
	"context"
	"sync"

	"github.com/harmony-one/harmony/log"
	discovery "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	maddr "github.com/multiformats/go-multiaddr"
)

var bootstrapPeersString = []string{
	"/ip4/127.0.0.1/tcp/3000/ipfs/QmcWJ2WV8WJVhtt1w2cZ7iryC8vRL7QxDm3P9vGNFFfg8F", // Copy the Addr of a chat node
}
var bootstrapPeers []maddr.Multiaddr

func init() {
	var err error
	bootstrapPeers, err = stringsToAddrs(bootstrapPeersString)
	if err != nil {
		panic(err)
	}
	log.Info("Bootstrap nodes", "peers", bootstrapPeers)
}

// SetRendezvousString -
func (h *HostV2) SetRendezvousString(str string) {
	h.rendezvousString = str
}

func (h *HostV2) setupDiscovery() {
	h.SetRendezvousString("meet me here")
	ctx := context.Background()
	var err error
	h.dht, err = libp2pdht.New(ctx, h.h)
	if err != nil {
		panic(err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Debug("Bootstrapping the DHT")
	if err = h.dht.Bootstrap(ctx); err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		log.Info("Trying to connect bootnode", "addr", peerAddr.String())
		peerinfo, err := peerstore.InfoFromP2pAddr(peerAddr)
		if err != nil {
			panic(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.h.Connect(ctx, *peerinfo); err != nil {
				log.Error("Failed to set up connection to bootstrap node", "err", err)
			} else {
				log.Info("Connection established with bootstrap node:", "peerInfo", *peerinfo)
			}
		}()
	}
	wg.Wait()

	routingDiscovery := discovery.NewRoutingDiscovery(h.dht)
	h.announceSelf(ctx, routingDiscovery)
	go h.searchPeer(ctx, routingDiscovery)
}

func (h *HostV2) announceSelf(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	log.Info("Announcing ourselves...", "rendezvousString", h.rendezvousString)
	discovery.Advertise(ctx, routingDiscovery, h.rendezvousString)
	log.Debug("Successfully announced!")
}

func (h *HostV2) searchPeer(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	log.Debug("Searching for other peers...", "rendezvousString", h.rendezvousString)
	peerChan, err := routingDiscovery.FindPeers(ctx, h.rendezvousString)
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == h.h.ID() {
			continue
		}
		log.Debug("Found peer", "peer", peer)
		h.h.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
	}

	// hang forever
	select {}
}
