package rpc

import (
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/libp2p/go-libp2p/core/peer"
)

// StructuredResponse type of RPCs
type StructuredResponse = map[string]interface{}

type C struct {
	TotalKnownPeers int `json:"total-known-peers"`
	Connected       int `json:"connected"`
	NotConnected    int `json:"not-connected"`
}

// NodeMetadata captures select metadata of the RPC answering node
type BootNodeMetadata struct {
	Version      string  `json:"version"`
	ShardID      uint32  `json:"shard-id"`
	NodeBootTime int64   `json:"node-unix-start-time"`
	PeerID       peer.ID `json:"peerid"`
	C            C       `json:"p2p-connectivity"`
}

type Config struct {
	HarmonyConfig harmonyconfig.HarmonyConfig
	NodeConfig    nodeconfig.ConfigType
	ChainConfig   params.ChainConfig
}
