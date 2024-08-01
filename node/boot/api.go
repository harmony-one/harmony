package bootnode

import (
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
	hmy_boot "github.com/harmony-one/harmony/hmy_boot"
	boot_rpc "github.com/harmony-one/harmony/rpc/boot"
	rpc_common "github.com/harmony-one/harmony/rpc/harmony/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerConnectivity ..
func (bootnode *BootNode) PeerConnectivity() (int, int, int) {
	return bootnode.host.PeerConnectivity()
}

// ListPeer return list of peers for a certain topic
func (bootnode *BootNode) ListPeer(topic string) []peer.ID {
	return bootnode.host.ListPeer(topic)
}

// ListTopic return list of topics the node subscribed
func (bootnode *BootNode) ListTopic() []string {
	return bootnode.host.ListTopic()
}

// ListBlockedPeer return list of blocked peers
func (bootnode *BootNode) ListBlockedPeer() []peer.ID {
	return bootnode.host.ListBlockedPeer()
}

// GetNodeBootTime ..
func (bootnode *BootNode) GetNodeBootTime() int64 {
	return bootnode.unixTimeAtNodeStart
}

// ReportPlainErrorSink is the report of failed transactions this node has (held in memory only)
func (bootnode *BootNode) ReportPlainErrorSink() types.TransactionErrorReports {
	return bootnode.TransactionErrorSink.PlainReport()
}

// StartRPC start RPC service
func (bootnode *BootNode) StartRPC() error {
	bootService := hmy_boot.New(bootnode)

	// Gather all the possible APIs to surface
	apis := bootnode.APIs(bootService)

	return boot_rpc.StartServers(bootService, apis, bootnode.NodeConfig.RPCServer, bootnode.HarmonyConfig.RPCOpt)
}

// StopRPC stop RPC service
func (bootnode *BootNode) StopRPC() error {
	return boot_rpc.StopServers()
}

// APIs return the collection of local RPC services.
// NOTE, some of these services probably need to be moved to somewhere else.
func (bootnode *BootNode) APIs(harmony *hmy_boot.BootService) []rpc.API {
	// Append all the local APIs and return
	return []rpc.API{}
}

func (bootnode *BootNode) GetConfig() rpc_common.Config {
	return rpc_common.Config{
		HarmonyConfig: *bootnode.HarmonyConfig,
		NodeConfig:    *bootnode.NodeConfig,
		ChainConfig:   bootnode.chainConfig,
	}
}