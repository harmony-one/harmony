package discovery

import (
	"fmt"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/p2p"
)

type ConfigEntry struct {
	IP          string
	Port        string
	Role        string
	ShardID     string
	ValidatorID int // Validator ID in its shard.
	leader      p2p.Peer
	self        p2p.Peer
	peers       []p2p.Peer
	pubK        kyber.Scalar
	priK        kyber.Point
}

func (config ConfigEntry) String() string {
	return fmt.Sprintf("idc: %v:%v", config.IP, config.Port)
}

func New(pubK kyber.Scalar, priK kyber.Point) *ConfigEntry {
	var config ConfigEntry
	config.pubK = pubK
	config.priK = priK

	config.peers = make([]p2p.Peer, 0)

	return &config
}

func (config *ConfigEntry) StartClientMode(idcIP, idcPort string) error {
	config.IP = "myip"
	config.Port = "myport"

	fmt.Printf("idc ip/port: %v/%v\n", idcIP, idcPort)

	// ...
	// TODO: connect to idc, and wait unless acknowledge
	return nil
}

func (config *ConfigEntry) GetShardID() string {
	return config.ShardID
}

func (config *ConfigEntry) GetPeers() []p2p.Peer {
	return config.peers
}

func (config *ConfigEntry) GetLeader() p2p.Peer {
	return config.leader
}

func (config *ConfigEntry) GetSelfPeer() p2p.Peer {
	return config.self
}
