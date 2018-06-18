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

func getShardId(myIp, myPort string, config *[][]string) string {
	for _, node := range *config {
		ip, port, shardId := node[0], node[1], node[3]
		if ip == myIp && port == myPort {
			return shardId
		}
	}
	return "N/A"
}

func getLeader(myShardId string, config *[][]string) p2p.Peer {
	var leaderPeer p2p.Peer
	for _, node := range *config {
		ip, port, status, shardId := node[0], node[1], node[2], node[3]
		if status == "leader" && myShardId == shardId{
			leaderPeer.Ip = ip
			leaderPeer.Port = port
		}
	}
	return leaderPeer
}

func getPeers(myIp, myPort, myShardId string, config *[][]string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, node := range *config {
		ip, port, status := node[0], node[1], node[2]
		if status == "leader" || ip == myIp && port == myPort {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func readConfigFile(configFile string) [][]string{
	file, _ := os.Open(configFile)
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	return result
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	flag.Parse()

	config := readConfigFile(*configFile)
	shardId := getShardId(*ip, *port, &config)
	peers := getPeers(*ip, *port, shardId, &config)
	leader := getLeader(shardId, &config)

	consensus := consensus.NewConsensus(*ip, *port, shardId, peers, leader)

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
