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

// BootNodeMetadata captures select metadata of the RPC answering boot node
type BootNodeMetadata struct {
	Version      string  `json:"version"`
	Network      string  `json:"network"`
	NodeBootTime int64   `json:"node-unix-start-time"`
	PeerID       peer.ID `json:"peerid"`
	C            C       `json:"p2p-connectivity"`
}

// BootNodePeerInfo captures the peer connectivity info of the boot node
type BootNodePeerInfo struct {
	PeerID         peer.ID   `json:"peerid"`
	KnownPeers     []peer.ID `json:"known-peers"`
	ConnectedPeers []peer.ID `json:"connected-peers"`
	C              C         `json:"c"`
}

type Config struct {
	HarmonyConfig harmonyconfig.HarmonyConfig
	NodeConfig    nodeconfig.ConfigType
	ChainConfig   params.ChainConfig
}
