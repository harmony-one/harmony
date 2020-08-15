package common

import (
	"fmt"
	"time"

	"github.com/coinbase/rosetta-sdk-go/types"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/rpc"
	"github.com/harmony-one/harmony/shard"
)

const (
	// RosettaVersion ..
	RosettaVersion = "1.0.0"

	// Blockchain ..
	Blockchain = "Harmony"

	// Symbol ..
	Symbol = "ONE"

	// Decimals ..
	Decimals = 18

	// CurveType ..
	CurveType = types.Secp256k1
)

var (
	// ReadTimeout ..
	ReadTimeout = 30 * time.Second

	// WriteTimeout ..
	WriteTimeout = 30 * time.Second

	// IdleTimeout ..
	IdleTimeout = 120 * time.Second
)

// ShardMetadata for the network identifier
type ShardMetadata struct {
	IsBeacon bool `json:"is_beacon"`
}

// GetNetwork fetches the networking identifier for the given shard
func GetNetwork(shardID uint32) (*types.NetworkIdentifier, error) {
	metadata, err := rpc.NewStructuredResponse(ShardMetadata{
		IsBeacon: shardID == shard.BeaconChainShardID,
	})
	if err != nil {
		return nil, err
	}
	return &types.NetworkIdentifier{
		Blockchain: Blockchain,
		Network:    getNetworkName(),
		SubNetworkIdentifier: &types.SubNetworkIdentifier{
			Network:  fmt.Sprintf("shard %d", shardID),
			Metadata: metadata,
		},
	}, nil
}

func getNetworkName() string {
	if shard.Schedule.GetNetworkID() == shardingconfig.MainNet {
		return "Mainnet"
	}
	return "Testnet"
}
