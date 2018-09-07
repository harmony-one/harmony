package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

func main() {
	ip := flag.String("ip", "localhost", "IP of the node")
	port := flag.String("port", "8080", "port of the node")
	flag.Parse()

	i := 0
	peer := p2p.Peer{Ip: *ip, Port: *port}
	fmt.Println("Now onto node i", i)
	idcpeer := p2p.Peer{Ip: "localhost", Port: "9000"} //Hardcoded here.
	node := node.NewWaitNode(peer, idcpeer)
	go func() {
		node.ConnectIdentityChain()
	}()
	node.StartServer(*port)

	time.Sleep(5 * time.Second)
	//}

}
