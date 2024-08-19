package hmy_boot

import (
	"github.com/harmony-one/harmony/api/proto"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	commonRPC "github.com/harmony-one/harmony/rpc/harmony/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

// BootService implements the BootService full node service.
type BootService struct {
	// Channel for shutting down the service
	ShutdownChan chan bool // Channel for shutting down the BootService
	// Boot node API
	BootNodeAPI BootNodeAPI
	// Shard ID
	ShardID uint32
}

// BootNodeAPI is the list of functions from node used to call rpc apis.
type BootNodeAPI interface {
	GetNodeBootTime() int64
	PeerConnectivity() (int, int, int)
	ListPeer(topic string) []peer.ID
	ListTopic() []string
	ListBlockedPeer() []peer.ID
	GetConfig() commonRPC.Config
	ShutDown()
}

// New creates a new BootService object (including the
// initialisation of the common BootService object)
func New(nodeAPI BootNodeAPI) *BootService {
	backend := &BootService{
		ShutdownChan: make(chan bool),

		BootNodeAPI: nodeAPI,
	}

	return backend
}

// ProtocolVersion ...
func (hmyboot *BootService) ProtocolVersion() int {
	return proto.ProtocolVersion
}

// GetNodeMetadata returns the node metadata.
func (hmyboot *BootService) GetNodeMetadata() commonRPC.NodeMetadata {
	var c commonRPC.C

	c.TotalKnownPeers, c.Connected, c.NotConnected = hmyboot.BootNodeAPI.PeerConnectivity()

	return commonRPC.NodeMetadata{
		Version:      nodeconfig.GetVersion(),
		ShardID:      hmyboot.ShardID,
		NodeBootTime: hmyboot.BootNodeAPI.GetNodeBootTime(),
		PeerID:       nodeconfig.GetPeerID(),
		C:            c,
	}
}
