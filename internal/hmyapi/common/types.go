package common

import "github.com/harmony-one/harmony/internal/params"

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
}
