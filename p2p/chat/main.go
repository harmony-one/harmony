package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	log "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-discovery"
	libhost "github.com/libp2p/go-libp2p-host"
	libp2pdht "github.com/libp2p/go-libp2p-kad-dht"
	inet "github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-protocol"
	multiaddr "github.com/multiformats/go-multiaddr"
	logging "github.com/whyrusleeping/go-logging"
)

var ProtocolID = "22222"
var rend = "123123"
var logger = log.Logger("rendezvous")

func catchError(err error) {
	if err != nil {
		logger.Error("catchError", "err", err)
		panic(err)
	}
}

func handleStream(stream inet.Stream) {
	logger.Info("Got a new stream!", "local", stream.Conn().LocalMultiaddr(), "remote", stream.Conn().RemoteMultiaddr())

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
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

func writeData(rw *bufio.ReadWriter) {
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

var (
	cb []string
)

func main() {
	log.SetAllLoggers(logging.WARNING)
	log.SetLogLevel("rendezvous", "info")
	b := flag.String("b", "", "addr of bootnode")
	p := flag.String("p", "3000", "port of the node.")
	flag.Parse()
	cb = []string{*b}
	cp := *p

	ctx := context.Background()

	sourceAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", cp))
	// libp2p.New constructs a new libp2p Host. Other options can be added
	// here.
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs(sourceAddr),
	)
	if err != nil {
		panic(err)
	}
	logger.Info("Host created. I'm ", sourceAddr.String()+"/ipfs/"+host.ID().Pretty())

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	host.SetStreamHandler(protocol.ID(ProtocolID), handleStream)

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := libp2pdht.New(ctx, host)
	catchError(err)

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	logger.Debug("Bootstrapping the DHT")
	err = kademliaDHT.Bootstrap(ctx)
	catchError(err)

	contactBootnode(ctx, host)

	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	announceSelf(ctx, routingDiscovery)
	findPeers(ctx, host, routingDiscovery)
}

func contactBootnode(ctx context.Context, host libhost.Host) {
	var wg sync.WaitGroup
	for _, peerAddrS := range cb {
		if peerAddrS == "" {
			continue
		}
		logger.Debug(peerAddrS)
		peerAddr, _ := multiaddr.NewMultiaddr(peerAddrS)
		peerinfo, err := peerstore.InfoFromP2pAddr(peerAddr)
		if err != nil {
			panic(err)
		}
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
}

func announceSelf(ctx context.Context, routingDiscovery *discovery.RoutingDiscovery) {
	logger.Info("Announcing ourselves...")
	discovery.Advertise(ctx, routingDiscovery, rend)
	logger.Debug("Successfully announced!")
}

func findPeers(ctx context.Context, host libhost.Host, routingDiscovery *discovery.RoutingDiscovery) {
	// Now, look for others who have announced
	// This is like your friend telling you the location to meet you.
	logger.Info("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rend)
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}
		logger.Debug("Found peer:", peer)

		logger.Debug("Connecting to:", peer)
		stream, err := host.NewStream(ctx, peer.ID, protocol.ID(ProtocolID))

		if err != nil {
			logger.Warning("Connection failed:", err)
			continue
		} else {
			rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

			go writeData(rw)
			go readData(rw)
		}

		logger.Info("Connected to:", "local", host.Addrs(), "remote", peer)
	}

	select {}
}
