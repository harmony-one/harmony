package main

import (
	"flag"
	"fmt"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

func main() {

	ip := flag.String("ip", "127.0.0.0", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	flag.Parse()
	peer := p2p.Peer{Ip: *ip, Port: *port}
	node := node.Node{}
	node.Self = peer
	msg := node.SerializeNode()
	fmt.Print(msg)
	// fmt.Println(ip)
	// fmt.Println(peer)

}
