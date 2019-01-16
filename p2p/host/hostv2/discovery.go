package hostv2

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/harmony-one/harmony/log"
	discovery "github.com/libp2p/go-libp2p-discovery"
	libhost "github.com/libp2p/go-libp2p-host"
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

// SetRendezvousString -
func (h *HostV2) SetRendezvousString(str string) {
	rendezvousString = str
}

func (h *HostV2) setupDiscovery() {
	ctx := context.Background()
	var err error
	// h.dht, err = libp2pdht.New(ctx, h.h)
	kademliaDHT, err := libp2pdht.New(ctx, h.h)
	catchError(err)

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	log.Debug("Bootstrapping the DHT")
	// _, err = h.dht.BootstrapWithConfig(libp2pdht.BootstrapConfig{ // https://godoc.org/github.com/libp2p/go-libp2p-kad-dht#IpfsDHT.BootstrapWithConfig
	// 	Queries: 10,
	// 	Period:  3 * time.Second,
	// 	Timeout: time.Duration(10 * time.Second),
	// })

	err = kademliaDHT.Bootstrap(ctx)

	catchError(err)

	h.contactBootnode(ctx)

	// routingDiscovery := discovery.NewRoutingDiscovery(h.dht)
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	announceSelf(ctx, routingDiscovery)
	searchPeer(ctx, routingDiscovery, h.h)
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

var rendezvousString = "tt"

func announceSelf(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	log.Info("Announcing ourselves...", "rendezvousString", rendezvousString)
	discovery.Advertise(ctx, routingDiscovery, rendezvousString)
	log.Debug("Successfully announced!")
}

func searchPeer(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery, h libhost.Host) {
	log.Debug("Searching for other peers...", "rendezvousString", rendezvousString)
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
	catchError(err)

	for peer := range peerChan {
		if peer.ID == h.ID() {
			continue
		}
		// log.Debug("Found peer", "peer", peer, "self", h.GetSelfPeer())
		// h.h.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)

		// msg := fmt.Sprintf("Ping from %s", h.h.Addrs())
		// h.SendMessageToPeer(peer.ID, msg)

		log.Debug("Connecting to:", "peer", peer)
		stream, err := h.NewStream(ctx, peer.ID, ProtocolID)

		if err != nil {
			log.Warn("Connection failed:", "err", err)
			continue
		} else {
			rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

			go writeData1(rw)
			go readData1(rw)
		}

	}

	// hang forever
	select {}
}

// AddBootNode -
func (h *HostV2) AddBootNode(addr string) {
	maddr, err := stringToAddr(addr)
	catchError(err)
	h.bootNodes = append(h.bootNodes, maddr)
}

func readData1(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData1(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}
