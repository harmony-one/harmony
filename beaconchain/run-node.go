package main

import (
	"flag"
	"time"

	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
)

func main() {

	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	flag.Parse()
	peer := p2p.Peer{Ip: *ip, Port: *port}
	// c, err := net.Dial("tcp4", "127.0.0.1:8081")
	// fmt.Println(c)
	// fmt.Println(err)
	node := node.Node{}
	node.SelfPeer = peer
	node.IDCPeer = p2p.Peer{Ip: *ip, Port: "8081"}
	// node.SetLog()
	// go node.StartServer(*port)
	// time.Sleep(5 * time.Second)
	node.ConnectBeaconChain()
	time.Sleep(5 * time.Second)

}
