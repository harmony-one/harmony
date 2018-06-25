package main

import (
	"bufio"
	"flag"
	"fmt"
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"harmony-benchmark/node"
	"harmony-benchmark/p2p"
	"os"
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
		if status == "leader" && myShardId == shardId {
			leaderPeer.Ip = ip
			leaderPeer.Port = port
		}
	}
	return leaderPeer
}

func getPeers(myIp, myPort, myShardId string, config *[][]string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, node := range *config {
		ip, port, status, shardId := node[0], node[1], node[2], node[3]
		if status != "validator" || ip == myIp && port == myPort || myShardId != shardId {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func getClientPeer(config *[][]string) *p2p.Peer {
	for _, node := range *config {
		ip, port, status := node[0], node[1], node[2]
		if status == "client" {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		return &peer
	}
	return nil
}

func readConfigFile(configFile string) [][]string {
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
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	flag.Parse()

	config := readConfigFile(*configFile)
	shardId := getShardId(*ip, *port, &config)
	peers := getPeers(*ip, *port, shardId, &config)
	leader := getLeader(shardId, &config)

	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/%v.log", *logFolder, *port)
	h := log.MultiHandler(
		log.Must.FileHandler(logFileName, log.LogfmtFormat()),
		log.StdoutHandler)
	// In cases where you just want a stdout logger, use the following one instead.
	// h := log.CallerFileHandler(log.StdoutHandler)
	log.Root().SetHandler(h)

	consensus := consensus.NewConsensus(*ip, *port, shardId, peers, leader)

	node := node.NewNode(&consensus)

	clientPeer := getClientPeer(&config)
	// If there is a client configured in the node list.
	if clientPeer != nil {
		node.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = node.VerifyNewBlock
	consensus.OnConsensusDone = node.PostConsensusProcessing

	// Temporary testing code, to be removed.
	node.AddMoreFakeTransactions(10000)

	if consensus.IsLeader {
		// Let consensus run
		go func() {
			consensus.WaitForNewBlock(node.BlockChannel)
		}()
		// Node waiting for consensus readiness to create new block
		go func() {
			node.WaitForConsensusReady(consensus.ReadySignal)
		}()
	}

	node.StartServer(*port)
}
