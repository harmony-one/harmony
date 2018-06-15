package main

import (
	"flag"
	"harmony-benchmark/consensus"
	"harmony-benchmark/p2p"
	"harmony-benchmark/node"
	"log"
	"os"
	"bufio"
	"strings"
)

func getLeader(iplist string) p2p.Peer {
	file, _ := os.Open(iplist)
	fscanner := bufio.NewScanner(file)
	var leaderPeer p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" {
			leaderPeer.Ip = ip
			leaderPeer.Port = port
		}
	}
	return leaderPeer
}

func getPeers(Ip, Port, iplist string) []p2p.Peer {
	file, _ := os.Open(iplist)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" || ip == Ip && port == Port {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	ipfile := flag.String("ipfile", "iplist.txt", "file containing all ip addresses")
	flag.Parse()

	consensus := consensus.NewConsensus(*ip, *port, getPeers(*ip, *port, *ipfile), getLeader(*ipfile))
	var nodeStatus string
	if consensus.IsLeader {
		nodeStatus = "leader"
	} else {
		nodeStatus = "validator"
	}

	log.Println("======================================")
	log.Printf("This node is a %s node listening on ip: %s and port: %s\n", nodeStatus, *ip, *port)
	log.Println("======================================")

	node := node.NewNode(&consensus)

	if consensus.IsLeader {
		// Let consensus run
		go func() {
			log.Println("Waiting for block")
			consensus.WaitForNewBlock(node.BlockChannel)
		}()
		// Node waiting for consensus readiness to create new block
		go func() {
			log.Println("Waiting for consensus ready")
			node.WaitForConsensusReady(consensus.ReadySignal)
		}()
	}

	node.StartServer(*port)
}
