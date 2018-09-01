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
	configr := Config{}
	return &configr
}

// Gets all the validator peers
func (configr *Config) GetValidators() []p2p.Peer {
	var peerList []p2p.Peer
	for _, entry := range configr.config {
		if entry.Role != "validator" {
			continue
		}
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		peerList = append(peerList, peer)
	}
	return peerList
}

// Gets all the leader peers and corresponding shard Ids
func (configr *Config) GetLeadersAndShardIds() ([]p2p.Peer, []uint32) {
	var peerList []p2p.Peer
	var shardIDs []uint32
	for _, entry := range configr.config {
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

func (configr *Config) GetClientPeer() *p2p.Peer {
	for _, entry := range configr.config {
		if entry.Role != "client" {
			continue
		}
		peer := p2p.Peer{Port: entry.Port, Ip: entry.IP}
		return &peer
	}
	return nil
}

// Gets the port of the client node in the config
func (configr *Config) GetClientPort() string {
	for _, entry := range configr.config {
		if entry.Role == "client" {
			return entry.Port
		}
	}
	return ""
}

// Parse the config file and return a 2d array containing the file data
func (configr *Config) ReadConfigFile(filename string) error {
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
	configr.config = result
	return nil
}
