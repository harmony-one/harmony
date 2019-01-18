package main

import (
	"bufio"
	"flag"
	"log"
	"time"

	elog "github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv2"
)

var (
	host *hostv2.HostV2
)

func main() {
	elog.Root().SetHandler(elog.StdoutHandler)
	port := flag.String("p", "3000", "port of the node.")
	boot := flag.String("b", "", "addr of bootnode")
	flag.Parse()

	peer := p2p.Peer{IP: "0.0.0.0", Port: *port}
	host = hostv2.New(peer)
	if *boot != "" {
		log.Printf("bootnode: %s", *boot)
		host.AddBootNode(*boot)
	}
	host.SetRendezvousString("some random text")
	go ping()
	host.BindHandlerAndServe(handleStream)
}

func handleStream(s p2p.Stream) {
	msg, err := bufio.NewReader(s).ReadString('\n')
	if err != nil {
		panic(err)
	}
	log.Printf("Received msg: %s", msg)
}

func ping() {
	for range time.Tick(3 * time.Second) {
		host.Broadcast()
	}
}
