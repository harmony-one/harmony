package main

import (
	"flag"
	"fmt"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

func main() {
	ip := flag.String("ip", "localhost", "IP of the node")
	port := flag.String("port", "8080", "port of the node")
	flag.Parse()
	peer := p2p.Peer{Ip: *ip, Port: *port}
	idcpeer := p2p.Peer{Ip: "localhost", Port: "9000"} //Hardcoded here.
	node := node.NewWaitNode(peer, idcpeer)
	go func() {
		node.ConnectIdentityChain()
	}()
	fmt.Println("control is back with me")
	node.StartServer(*port)
	fmt.Println("starting the server")

}
