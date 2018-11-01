package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

type ConfigEntry struct {
	IP      string
	Port    string
	Role    string
	ShardID string
}

type Config struct {
	config []ConfigEntry
}

func NewConfig() *Config {
	config := Config{}
	return &config
}

// Gets all the validator peers
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

// Gets all the leader peers and corresponding shard Ids
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

// Gets the port of the client node in the config
func (config *Config) GetClientPort() string {
	for _, entry := range config.config {
		if entry.Role == "client" {
			return entry.Port
		}
	}
	return ""
}

// Parse the config file and return a 2d array containing the file data
func (config *Config) ReadConfigFile(filename string) error {
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
