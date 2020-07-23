package common

import (
	"github.com/harmony-one/harmony/internal/params"
	"github.com/libp2p/go-libp2p-core/peer"
)

// C ..
type C struct {
	TotalKnownPeers int `json:"total-known-peers"`
	Connected       int `json:"connected"`
	NotConnected    int `json:"not-connected"`
}

// NodeMetadata captures select metadata of the RPC answering node
type NodeMetadata struct {
	BLSPublicKey   []string           `json:"blskey"`
	Version        string             `json:"version"`
	NetworkType    string             `json:"network"`
	ChainConfig    params.ChainConfig `json:"chain-config"`
	IsLeader       bool               `json:"is-leader"`
	ShardID        uint32             `json:"shard-id"`
	CurrentEpoch   uint64             `json:"current-epoch"`
	BlocksPerEpoch *uint64            `json:"blocks-per-epoch,omitempty"`
	Role           string             `json:"role"`
	DNSZone        string             `json:"dns-zone"`
	Archival       bool               `json:"is-archival"`
	NodeBootTime   int64              `json:"node-unix-start-time"`
	PeerID         peer.ID            `json:"peerid"`
	C              C                  `json:"p2p-connectivity"`
}

// P captures the connected peers per topic
type P struct {
	Topic string    `json:"topic"`
	Peers []peer.ID `json:"peers"`
}

// NodePeerInfo captures the peer connectivity info of the node
type NodePeerInfo struct {
	PeerID       peer.ID   `json:"peerid"`
	BlockedPeers []peer.ID `json:"blocked-peers"`
	P            []P       `json:"connected-peers"`
}
