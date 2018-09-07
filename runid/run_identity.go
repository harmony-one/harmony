package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/simple-rules/harmony-benchmark/identitychain"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

func main() {
	ip := flag.String("ip", "127.0.0.0", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	flag.Parse()
	peer := p2p.Peer{Ip: *ip, Port: *port}
	IDC := identitychain.New(peer)
	fmt.Println(IDC)
	epochTimer := time.NewTicker(10 * time.Second)
	go func() {
		for t := range epochTimer.C {

			fmt.Println("Changing epoch at ", t)
			IDC.Shard()

		}
	}()
	IDC.StartServer()

}
