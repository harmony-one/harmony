package configr

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/utils"
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

// Gets the port of the client node in the config
func GetClientPort(config *[][]string) string {
	for _, node := range *config {
		_, port, status, _ := node[0], node[1], node[2], node[3]
		if status == "client" {
			return port
		}
	}
	return ""
}

// GetShardID Gets the shard id of the node corresponding to this ip and port
func GetShardID(myIp, myPort string, config *[][]string) string {
	for _, node := range *config {
		ip, port, shardId := node[0], node[1], node[3]
		if ip == myIp && port == myPort {
			return shardId
		}
	}
	return "N/A"
}

// GetPeers Gets the peer list of the node corresponding to this ip and port
func GetPeers(myIp, myPort, myShardId string, config *[][]string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, node := range *config {
		ip, port, status, shardId := node[0], node[1], node[2], node[3]
		if status != "validator" || ip == myIp && port == myPort || myShardId != shardId {
			continue
		}
		// Get public key deterministically based on ip and port
		peer := p2p.Peer{Port: port, Ip: ip}
		priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(utils.GetUniqueIdFromPeer(peer)))
		peer.PubKey = pki.GetPublicKeyFromScalar(priKey)
		peerList = append(peerList, peer)
	}
	return peerList
}

// GetLeader Gets the leader of this shard id
func GetLeader(myShardId string, config *[][]string) p2p.Peer {
	var leaderPeer p2p.Peer
	for _, node := range *config {
		ip, port, status, shardId := node[0], node[1], node[2], node[3]
		if status == "leader" && myShardId == shardId {
			leaderPeer.Ip = ip
			leaderPeer.Port = port

			// Get public key deterministically based on ip and port
			priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(utils.GetUniqueIdFromPeer(leaderPeer))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
			leaderPeer.PubKey = pki.GetPublicKeyFromScalar(priKey)
		}
	}
	return leaderPeer
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
