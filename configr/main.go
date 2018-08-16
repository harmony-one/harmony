package configr

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/utils"
)

type ConfigEntry struct {
	IP      string
	Port    string
	Role    string
	ShardID string
}

type Configr struct {
	config []ConfigEntry
}

func NewConfigr() *Configr {
	configr := Configr{}
	return &configr
}

// Gets all the validator peers
func (configr *Configr) GetValidators() []p2p.Peer {
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
func (configr *Configr) GetLeadersAndShardIds() ([]p2p.Peer, []uint32) {
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

func (configr *Configr) GetClientPeer() *p2p.Peer {
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
func (configr *Configr) GetClientPort() string {
	for _, entry := range configr.config {
		if entry.Role == "client" {
			return entry.Port
		}
	}
	return ""
}

// Parse the config file and return a 2d array containing the file data
func (configr *Configr) ReadConfigFile(filename string) error {
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
	log.Println(result)
	configr.config = result
	return nil
}

// GetShardID Gets the shard id of the node corresponding to this ip and port
func (configr *Configr) GetShardID(ip, port string) string {
	for _, entry := range configr.config {
		if entry.IP == ip && entry.Port == port {
			return entry.ShardID
		}
	}
	return "N/A"
}

// GetPeers Gets the peer list of the node corresponding to this ip and port
func (configr *Configr) GetPeers(ip, port, shardID string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, entry := range configr.config {
		if entry.Role != "validator" || entry.IP == ip && entry.Port == port || entry.ShardID != shardID {
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
func (configr *Configr) GetLeader(shardID string) p2p.Peer {
	var leaderPeer p2p.Peer
	for _, entry := range configr.config {
		if entry.Role == "leader" && entry.ShardID == shardID {
			leaderPeer.Ip = entry.IP
			leaderPeer.Port = entry.Port
			setKey(&leaderPeer)
		}
	}
	return leaderPeer
}

func (configr *Configr) GetConfigEntries() []ConfigEntry {
	return configr.config
}

func (configr *Configr) GetMyConfigEntry(ip string, port string) *ConfigEntry {
	for _, entry := range configr.config {
		if entry.IP == ip && entry.Port == port {
			return &entry
		}
	}
	return nil
}

func setKey(peer *p2p.Peer) {
	// Get public key deterministically based on ip and port
	priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(utils.GetUniqueIdFromPeer(*peer))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
	peer.PubKey = crypto.GetPublicKeyFromScalar(crypto.Ed25519Curve, priKey)
}
