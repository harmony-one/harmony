package hostv2

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	discovery "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	maddr "github.com/multiformats/go-multiaddr"
)

var bootstrapPeersString = []string{
	// "/ip4/127.0.0.1/tcp/3000/ipfs/QmStdwSAjkMCQLgwuMFGuHhFNWYjFDMW5eu3wTNacPsLK5", // Copy the Addr of a chat node
}
var bootstrapPeers []maddr.Multiaddr

func init() {
	var err error
	bootstrapPeers, err = stringsToAddrs(bootstrapPeersString)
	catchError(err)
	log.Info("Default Boot Nodes", "peers", bootstrapPeers)
}

// SetRendezvousString sets the rendezvous string
func (h *HostV2) SetRendezvousString(str string) {
	h.rendezvousString = str
}

func (h *HostV2) startDiscovery() {
	ctx := context.Background()
	var err error
	h.dht, err = libp2pdht.New(ctx, h.h)
	catchError(err)

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Debug("Bootstrapping the DHT")

	err = h.dht.Bootstrap(ctx)
	catchError(err)

	h.contactBootnode(ctx)
	routingDiscovery := discovery.NewRoutingDiscovery(h.dht)
	h.announceSelf(ctx, routingDiscovery)
	h.searchPeer(ctx, routingDiscovery)
}

func (h *HostV2) contactBootnode(ctx context.Context) {
	log.Info("BootNodes", "nodes", h.bootNodes)
	var wg sync.WaitGroup
	for _, peerAddr := range h.bootNodes {
		log.Info("Trying to connect bootnode", "addr", peerAddr)
		peerinfo, err := peerstore.InfoFromP2pAddr(peerAddr)
		catchError(err)
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
}

func (h *HostV2) announceSelf(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	log.Info("Announcing ourselves...", "rendezvousString", h.rendezvousString)
	discovery.Advertise(ctx, routingDiscovery, h.rendezvousString)
	log.Debug("Successfully announced!")
}

func (h *HostV2) searchPeer(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	log.Debug("Searching for other peers...", "rendezvousString", h.rendezvousString)
	peerChan, err := routingDiscovery.FindPeers(ctx, h.rendezvousString)
	catchError(err)

	for peer := range peerChan {
		if peer.ID == h.h.ID() {
			continue
		}
		log.Debug("Found peer", "peer", peer, "self", h.GetSelfPeer())
		h.h.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
	}
}

// AddBootNode add an address as bootnode
func (h *HostV2) AddBootNode(addr string) {
	maddr, err := stringToAddr(addr)
	catchError(err)
	h.bootNodes = append(h.bootNodes, maddr)
}
