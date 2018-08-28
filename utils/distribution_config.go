package utils

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/p2p"
)

type ConfigEntry struct {
	IP      string
	Port    string
	Role    string
	ShardID string
}

type DistributionConfig struct {
	config []ConfigEntry
}

// done
func NewDistributionConfig() *DistributionConfig {
	config := DistributionConfig{}
	return &config
}

// done
// Gets all the validator peers
func (config *DistributionConfig) GetValidators() []p2p.Peer {
	var peerList []p2p.Peer
	for _, entry := range config.config {
		if entry.Role != "validator" {
			continue
		}
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		peerList = append(peerList, peer)
	}
	return peerList
}

// done
// Gets all the leader peers and corresponding shard Ids
func (config *DistributionConfig) GetLeadersAndShardIds() ([]p2p.Peer, []uint32) {
	var peerList []p2p.Peer
	var shardIDs []uint32
	for _, entry := range config.config {
		if entry.Role == "leader" {
			peerList = append(peerList, p2p.Peer{Ip: entry.IP, Port: entry.Port})
			val, err := strconv.Atoi(entry.ShardID)
			if err == nil {
				shardIDs = append(shardIDs, uint32(val))
			} else {
				log.Print("[Generator] Error parsing the shard Id ", entry.ShardID)
			}
		}
	}
	return peerList, shardIDs
}

func (config *DistributionConfig) GetClientPeer() *p2p.Peer {
	for _, entry := range config.config {
		if entry.Role != "client" {
			continue
		}
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		return &peer
	}
	return nil
}

// done
// Gets the port of the client node in the config
func (config *DistributionConfig) GetClientPort() string {
	for _, entry := range config.config {
		if entry.Role == "client" {
			return entry.Port
		}
	}
	return ""
}

// done
// Parse the config file and return a 2d array containing the file data
func (config *DistributionConfig) ReadConfigFile(filename string) error {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Failed to read config file ", filename)
		return err
	}
	fscanner := bufio.NewScanner(file)

	result := []ConfigEntry{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		entry := ConfigEntry{p[0], p[1], p[2], p[3]}
		result = append(result, entry)
	}
	config.config = result
	return nil
}

// GetShardID Gets the shard id of the node corresponding to this ip and port
func (config *DistributionConfig) GetShardID(ip, port string) string {
	for _, entry := range config.config {
		if entry.IP == ip && entry.Port == port {
			return entry.ShardID
		}
	}
	return "N/A"
}

// GetPeers Gets the validator list
func (config *DistributionConfig) GetPeers(ip, port, shardID string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, entry := range config.config {
		if entry.Role != "validator" || entry.ShardID != shardID {
			continue
		}
		// Get public key deterministically based on ip and port
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		setKey(&peer)
		peerList = append(peerList, peer)
	}
	return peerList
}

// GetLeader Gets the leader of this shard id
func (config *DistributionConfig) GetLeader(shardID string) p2p.Peer {
	var leaderPeer p2p.Peer
	for _, entry := range config.config {
		if entry.Role == "leader" && entry.ShardID == shardID {
			leaderPeer.Ip = entry.IP
			leaderPeer.Port = entry.Port
			setKey(&leaderPeer)
		}
	}
	return leaderPeer
}

func (config *DistributionConfig) GetConfigEntries() []ConfigEntry {
	return config.config
}

func (config *DistributionConfig) GetMyConfigEntry(ip string, port string) *ConfigEntry {
	for _, entry := range config.config {
		if entry.IP == ip && entry.Port == port {
			return &entry
		}
	}
	return nil
}

func setKey(peer *p2p.Peer) {
	// Get public key deterministically based on ip and port
	priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(GetUniqueIdFromPeer(*peer))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
	peer.PubKey = pki.GetPublicKeyFromScalar(priKey)
}
