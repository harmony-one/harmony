package discovery

import (
	"fmt"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/p2p"
)

// ConfigEntry is the config entry.
type ConfigEntry struct {
	IP          string
	Port        string
	Role        string
	ShardID     string
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	self        p2p.Peer
	peers       []p2p.Peer
	priK        kyber.Scalar
	pubK        kyber.Point
}

func (config ConfigEntry) String() string {
	return fmt.Sprintf("bc: %v:%v", config.IP, config.Port)
}

// New return new ConfigEntry.
// TODO: This should be change because this package is discovery and New here implies New Discovery.
func New(priK kyber.Scalar, pubK kyber.Point) *ConfigEntry {
	var config ConfigEntry
	config.priK = priK
	config.pubK = pubK

	config.peers = make([]p2p.Peer, 0)

	return &config
}

// StartClientMode starts client mode.
func (config *ConfigEntry) StartClientMode(bcIP, bcPort string) error {
	config.IP = "myip"
	config.Port = "myport"

	fmt.Printf("bc ip/port: %v/%v\n", bcIP, bcPort)

	// ...
	// TODO: connect to bc, and wait unless acknowledge
	return nil
}

// GetShardID ...
func (config *ConfigEntry) GetShardID() string {
	return config.ShardID
}

// GetPeers ...
func (config *ConfigEntry) GetPeers() []p2p.Peer {
	return config.peers
}

// GetLeader ...
func (config *ConfigEntry) GetLeader() p2p.Peer {
	return config.leader
}

// GetSelfPeer ...
func (config *ConfigEntry) GetSelfPeer() p2p.Peer {
	return config.self
}
