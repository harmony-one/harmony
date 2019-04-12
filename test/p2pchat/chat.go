package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	log "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	discovery "github.com/libp2p/go-libp2p-discovery"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	multiaddr "github.com/multiformats/go-multiaddr"
	logging "github.com/whyrusleeping/go-logging"
)

var logger = log.Logger("rendezvous")

// Harmony MIT License
func writePubsub(ps *pubsub.PubSub) {
	stdReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		data, _ := stdReader.ReadString('\n')
		ps.Publish("pubsubtestchannel", []byte(data))
	}
}

// Harmony MIT License
func readPubsub(sub *pubsub.Subscription) {
	ctx := context.Background()
	for {
		m, err := sub.Next(ctx)

		if err == nil {
			msg := m.Data
			sender := peer.ID(m.From)
			fmt.Printf("Received pubsub: '%v' from: %v\n", string(msg), sender)
		}
	}
}

func main() {
	log.SetAllLoggers(logging.WARNING)
	log.SetLogLevel("rendezvous", "info")
	help := flag.Bool("h", false, "Display Help")
	config, err := ParseFlags()
	if err != nil {
		panic(err)
	}

	if *help {
		fmt.Println("This program demonstrates a simple p2p chat application using libp2p")
		fmt.Println()
		fmt.Println("Usage: Run './p2pchat in two different terminals. Let them connect to the bootstrap nodes, announce themselves and connect to the peers")
		flag.PrintDefaults()
		return
	}

	ctx := context.Background()

	// libp2p.New constructs a new libp2p Host. Other options can be added
	// here.
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs([]multiaddr.Multiaddr(config.ListenAddresses)...),
	)
	if err != nil {
		panic(err)
	}
	logger.Info("Host created. We are:", host.ID())
	logger.Info(host.Addrs())

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := libp2pdht.New(ctx, host)
	if err != nil {
		panic(err)
	}

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	logger.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	// Let's connect to the bootstrap nodes first. They will tell us about the
	// other nodes in the network.
	var wg sync.WaitGroup
	for _, peerAddr := range config.BootstrapPeers {
		peerinfo, _ := peerstore.InfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				logger.Warning(err)
			} else {
				logger.Info("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	logger.Info("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	discovery.Advertise(ctx, routingDiscovery, config.RendezvousString)
	logger.Debug("Successfully announced!")

	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	logger.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, config.RendezvousString)
	if err != nil {
		panic(err)
	}

	var ps *pubsub.PubSub

	switch config.PubSubImpl {
	case "gossip":
		ps, err = pubsub.NewGossipSub(ctx, host)
	case "flood":
		ps, err = pubsub.NewFloodSub(ctx, host)
	default:
		logger.Error("Unsupported Pubsub implementation")
		return
	}

	if err != nil {
		fmt.Printf("pub error: %v", err)
		panic(err)
	}

	sub, err := ps.Subscribe("pubsubtestchannel")

	if err != nil {
		fmt.Printf("sub error: %v", err)
		panic(err)
	}

	go writePubsub(ps)
	go readPubsub(sub)

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}
		logger.Debug("Found peer:", peer)

		if err := host.Connect(ctx, peer); err != nil {
			logger.Warning("can't connect to peer", "error", err, "peer", peer)
		} else {
			logger.Info("connected to peer host", "node", peer)
		}
	}

	select {}
}
