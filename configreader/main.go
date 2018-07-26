package configreader

import (
	"bufio"
	"harmony-benchmark/p2p"
	"log"
	"os"
	"strconv"
	"strings"
)

// Gets all the validator peers
func GetValidators(config string) []p2p.Peer {
	file, _ := os.Open(config)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" || status == "client" {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

// Gets all the leader peers and corresponding shard Ids
func GetLeadersAndShardIds(config *[][]string) ([]p2p.Peer, []uint32) {
	var peerList []p2p.Peer
	var shardIds []uint32
	for _, node := range *config {
		ip, port, status, shardId := node[0], node[1], node[2], node[3]
		if status == "leader" {
			peerList = append(peerList, p2p.Peer{Ip: ip, Port: port})
			val, err := strconv.Atoi(shardId)
			if err == nil {
				shardIds = append(shardIds, uint32(val))
			} else {
				log.Print("[Generator] Error parsing the shard Id ", shardId)
			}
		}
	}
	return peerList, shardIds
}

func GetClientPeer(config *[][]string) *p2p.Peer {
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

// Parse the config file and return a 2d array containing the file data
func ReadConfigFile(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Failed to read config file ", filename)
		return nil, err
	}
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	log.Println(result)
	return result, nil
}
