package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

// Entry is a single config of a node.
type Entry struct {
	IP      string
	Port    string
	Role    string
	ShardID string
}

// Config is a struct containing multiple Entry of all nodes.
type Config struct {
	config []Entry
}

// NewConfig returns a pointer to a Config.
func NewConfig() *Config {
	config := Config{}
	return &config
}

// GetValidators returns all the validator peers
func (config *Config) GetValidators() []p2p.Peer {
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

// GetShardIDToLeaderMap returns all the leader peers and corresponding shard Ids
func (config *Config) GetShardIDToLeaderMap() map[uint32]p2p.Peer {
	shardIDLeaderMap := map[uint32]p2p.Peer{}
	for _, entry := range config.config {
		if entry.Role == "leader" {
			val, err := strconv.Atoi(entry.ShardID)
			if err == nil {
				shardIDLeaderMap[uint32(val)] = p2p.Peer{Ip: entry.IP, Port: entry.Port}
			} else {
				log.Print("[Generator] Error parsing the shard Id ", entry.ShardID)
			}
		}
	}
	return shardIDLeaderMap
}

// GetClientPeer returns the client peer.
func (config *Config) GetClientPeer() *p2p.Peer {
	for _, entry := range config.config {
		if entry.Role != "client" {
			continue
		}
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		return &peer
	}
	return nil
}

// GetClientPort returns the port of the client node in the config
func (config *Config) GetClientPort() string {
	for _, entry := range config.config {
		if entry.Role == "client" {
			return entry.Port
		}
	}
	return ""
}

// ReadConfigFile parses the config file and return a 2d array containing the file data
func (config *Config) ReadConfigFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Failed to read config file ", filename)
		return err
	}
	defer file.Close()
	fscanner := bufio.NewScanner(file)

	result := []Entry{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		entry := Entry{p[0], p[1], p[2], p[3]}
		result = append(result, entry)
	}
	config.config = result
	return nil
}
