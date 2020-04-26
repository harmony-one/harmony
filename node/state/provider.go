package state

import (
	"errors"
	"fmt"
	"strconv"

	ethCommon "github.com/ethereum/go-ethereum/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
)

const (
	SyncingPortDifference = 3000
)

type SyncPeerProvider interface {
	Peers() []p2p.Peer
}

// GetSyncingPort ..
func GetSyncingPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-SyncingPortDifference)
	}
	return ""
}

// NewPeerProvider ..
func NewPeerProvider(
	port string,
	networkType string,
	dnsZone string,
	dnsFlag bool,
	cfg *nodeconfig.ConfigType,
) (SyncPeerProvider, error) {

	switch {
	case nodeconfig.Network(networkType) == nodeconfig.Localnet:
		epochConfig := shard.Schedule.InstanceForEpoch(ethCommon.Big0)
		selfPort, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return nil, err
		}
		return p2p.NewLocal(6000, uint16(selfPort),
			epochConfig.NumShards(),
			uint32(epochConfig.NumNodesPerShard()),
		), nil

	case dnsZone != "":
		return p2p.NewDNS(dnsZone, GetSyncingPort(port)), nil

	case dnsFlag:
		return p2p.NewDNS("t.hmny.io", GetSyncingPort(port)), nil

	}

	return nil, errors.New("no peer provider made for state sync")
}
