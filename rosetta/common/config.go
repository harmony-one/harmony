package common

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coinbase/rosetta-sdk-go/types"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/shard"
)

const (
	// RosettaVersion tied back to the version of the rosetta go-sdk
	RosettaVersion = "1.4.6" // TODO (dm): set variable via build flags

	// Blockchain ..
	Blockchain = "Harmony"

	// NativeSymbol ..
	NativeSymbol = "ONE"

	// NativePrecision in the number of decimal places
	NativePrecision = 18

	// CurveType ..
	CurveType = types.Secp256k1

	// SignatureType ..
	SignatureType = types.EcdsaRecovery
)

var (
	// ReadTimeout ..
	ReadTimeout = 60 * time.Second

	// WriteTimeout ..
	WriteTimeout = 60 * time.Second

	// IdleTimeout ..
	IdleTimeout = 120 * time.Second

	// NativeCurrency ..
	NativeCurrency = types.Currency{
		Symbol:   NativeSymbol,
		Decimals: NativePrecision,
	}

	// NativeCurrencyHash for quick equivalent checks
	NativeCurrencyHash = types.Hash(NativeCurrency)
)

// SyncStatus ..
type SyncStatus int

// Sync status enum
const (
	SyncingUnknown SyncStatus = iota
	SyncingNewBlock
	SyncingFinish
)

// String ..
func (s SyncStatus) String() string {
	return [...]string{"unknown", "syncing new block(s)", "fully synced"}[s]
}

// SubNetworkMetadata for the sub network identifier of a shard
type SubNetworkMetadata struct {
	IsBeacon bool `json:"is_beacon"`
}

// UnmarshalFromInterface ..
func (s *SubNetworkMetadata) UnmarshalFromInterface(metadata interface{}) error {
	var newMetadata SubNetworkMetadata
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &newMetadata); err != nil {
		return err
	}
	*s = newMetadata
	return nil
}

// GetNetwork fetches the networking identifier for the given shard
func GetNetwork(shardID uint32) (*types.NetworkIdentifier, error) {
	metadata, err := types.MarshalMap(SubNetworkMetadata{
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
