package bootnode

import (
	"errors"
	"time"

	"github.com/harmony-one/harmony/eth/rpc"
	hmy_boot "github.com/harmony-one/harmony/hmy_boot"
	bootnodeConfigs "github.com/harmony-one/harmony/internal/configs/bootnode"
	nodeConfigs "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	boot_rpc "github.com/harmony-one/harmony/rpc/boot"
	rpc_common "github.com/harmony-one/harmony/rpc/boot/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

var errHostNil = errors.New("bootnode host is nil")

// PeerID returns self Peer ID
func (bootnode *BootNode) PeerID() peer.ID {
	if bootnode.host == nil {
		return ""
	}
	return bootnode.host.GetID()
}

// PeerConnectivity ..
func (bootnode *BootNode) PeerConnectivity() (int, int, int) {
	if bootnode.host == nil {
		return 0, 0, 0
	}
	return bootnode.host.PeerConnectivity()
}

// ListKnownPeers return known peers
func (bootnode *BootNode) ListKnownPeers() peer.IDSlice {
	if bootnode.host == nil {
		return peer.IDSlice{}
	}
	bs := bootnode.host.GetP2PHost().Peerstore()
	if bs == nil {
		return peer.IDSlice{}
	}
	return bs.Peers()
}

// ListConnectedPeers return connected peers
func (bootnode *BootNode) ListConnectedPeers() []peer.ID {
	if bootnode.host == nil {
		return nil
	}
	return bootnode.host.Network().Peers()
}

// ListPeer return list of peers for a certain topic
func (bootnode *BootNode) ListPeer(topic string) []peer.ID {
	if bootnode.host == nil {
		return nil
	}
	return bootnode.host.ListPeer(topic)
}

// ListTopic return list of topics the node subscribed
func (bootnode *BootNode) ListTopic() []string {
	if bootnode.host == nil {
		return nil
	}
	return bootnode.host.ListTopic()
}

// ListBlockedPeer return list of blocked peers
func (bootnode *BootNode) ListBlockedPeer() []peer.ID {
	if bootnode.host == nil {
		return nil
	}
	return bootnode.host.ListBlockedPeer()
}

// GetNodeBootTime ..
func (bootnode *BootNode) GetNodeBootTime() int64 {
	return bootnode.unixTimeAtNodeStart
}

// StartRPC start RPC service
func (bootnode *BootNode) StartRPC() error {
	if bootnode.host == nil {
		return errHostNil
	}
	if bootnode.RPCConfig == nil {
		return errors.New("bootnode RPC config is nil")
	}
	if bootnode.HarmonyConfig == nil {
		return errors.New("bootnode harmony config is nil")
	}
	bootService := hmy_boot.New(bootnode)
	// Gather all the possible APIs to surface
	apis := bootnode.APIs(bootService)

	return boot_rpc.StartServers(bootService, apis, *bootnode.RPCConfig, bootnode.HarmonyConfig.RPCOpt)
}

func (bootnode *BootNode) initRPCServerConfig() {
	cfg := bootnode.HarmonyConfig

	readTimeout, err := time.ParseDuration(cfg.HTTP.ReadTimeout)
	if err != nil {
		readTimeout, _ = time.ParseDuration(nodeConfigs.DefaultHTTPTimeoutRead)
		utils.Logger().Warn().
			Str("provided", cfg.HTTP.ReadTimeout).
			Dur("updated", readTimeout).
			Msg("Sanitizing invalid http read timeout")
	}
	writeTimeout, err := time.ParseDuration(cfg.HTTP.WriteTimeout)
	if err != nil {
		writeTimeout, _ = time.ParseDuration(nodeConfigs.DefaultHTTPTimeoutWrite)
		utils.Logger().Warn().
			Str("provided", cfg.HTTP.WriteTimeout).
			Dur("updated", writeTimeout).
			Msg("Sanitizing invalid http write timeout")
	}
	idleTimeout, err := time.ParseDuration(cfg.HTTP.IdleTimeout)
	if err != nil {
		idleTimeout, _ = time.ParseDuration(nodeConfigs.DefaultHTTPTimeoutIdle)
		utils.Logger().Warn().
			Str("provided", cfg.HTTP.IdleTimeout).
			Dur("updated", idleTimeout).
			Msg("Sanitizing invalid http idle timeout")
	}
	bootnode.RPCConfig = &bootnodeConfigs.RPCServerConfig{
		HTTPEnabled:        cfg.HTTP.Enabled,
		HTTPIp:             cfg.HTTP.IP,
		HTTPPort:           cfg.HTTP.Port,
		HTTPTimeoutRead:    readTimeout,
		HTTPTimeoutWrite:   writeTimeout,
		HTTPTimeoutIdle:    idleTimeout,
		WSEnabled:          cfg.WS.Enabled,
		WSIp:               cfg.WS.IP,
		WSPort:             cfg.WS.Port,
		DebugEnabled:       cfg.RPCOpt.DebugEnabled,
		RpcFilterFile:      cfg.RPCOpt.RpcFilterFile,
		RateLimiterEnabled: cfg.RPCOpt.RateLimterEnabled,
		RequestsPerSecond:  cfg.RPCOpt.RequestsPerSecond,
	}
}

func (bootnode *BootNode) GetRPCServerConfig() *bootnodeConfigs.RPCServerConfig {
	return bootnode.RPCConfig
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
	cfg := rpc_common.Config{
		ChainConfig: params.ChainConfig{},
	}
	if bootnode.HarmonyConfig != nil {
		cfg.HarmonyConfig = *bootnode.HarmonyConfig
	}
	if bootnode.NodeConfig != nil {
		cfg.NodeConfig = *bootnode.NodeConfig
	}
	return cfg
}
