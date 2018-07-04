package main

import (
	"bufio"
	"flag"
	"fmt"
	"harmony-benchmark/attack"
	"harmony-benchmark/consensus"
	"harmony-benchmark/log"
	"harmony-benchmark/node"
	"harmony-benchmark/p2p"
	"math/rand"
	"os"
	"strings"
	"time"
)

const (
	AttackProbability = 20
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
		if status != "client" {
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

func attackDetermination(attackedMode int) bool {
	switch attackedMode {
	case 0:
		return false
	case 1:
		return true
	case 2:
		return rand.Intn(100) < AttackProbability
	}
	return false
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	attackedMode := flag.Int("attacked_mode", 0, "0 means not attacked, 1 means attacked, 2 means being open to be selected as attacked")
	flag.Parse()

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	// Attack determination.
	attack.GetAttackModel().SetAttackEnabled(attackDetermination(*attackedMode))

	config := readConfigFile(*configFile)
	shardId := getShardId(*ip, *port, &config)
	peers := getPeers(*ip, *port, shardId, &config)
	leader := getLeader(shardId, &config)

	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/%v.log", *logFolder, *port)
	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
		// log.Must.NetHandler("tcp", ":3000", log.JSONFormat()) // Log to remote
	)
	log.Root().SetHandler(h)

	consensus := consensus.NewConsensus(*ip, *port, shardId, peers, leader)

	node := node.NewNode(&consensus)
	// Set logger to attack model.
	attack.GetAttackModel().SetLogger(consensus.Log)

	clientPeer := getClientPeer(&config)
	// If there is a client configured in the node list.
	if clientPeer != nil {
		node.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = node.VerifyNewBlock
	consensus.OnConsensusDone = node.PostConsensusProcessing

	// Temporary testing code, to be removed.
	node.AddTestingAddresses(10000)

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
