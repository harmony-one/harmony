package node

import (
	"fmt"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	"github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/discovery"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
)

func (node *Node) setupForValidator() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(
		service.PeerDiscovery,
		discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer),
	)
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), chanPeer, nil, node.networkInfoDHTPath(),
		),
	)
	// Register consensus service.
	node.serviceManager.RegisterService(
		service.Consensus,
		consensus.New(node.BlockChannel, node.Consensus, node.startConsensus),
	)
	// Register new block service.
	node.serviceManager.RegisterService(
		service.BlockProposal,
		blockproposal.New(node.Consensus.ReadySignal, node.WaitForConsensusReadyV2),
	)

	if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet {
		// Register client support service.
		node.serviceManager.RegisterService(
			service.ClientSupport,
			clientsupport.New(
				node.Blockchain().State, node.CallFaucetContract, node.SelfPeer.IP, node.SelfPeer.Port,
			),
		)
	}
}

func (node *Node) setupForExplorerNode() {
	nodeConfig, chanPeer := node.initNodeConfiguration()

	// Register peer discovery service.
	node.serviceManager.RegisterService(
		service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, nil),
	)
	// Register networkinfo service.
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), chanPeer, nil, node.networkInfoDHTPath()),
	)
	// Register explorer service.
	node.serviceManager.RegisterService(
		service.SupportExplorer, explorer.New(&node.SelfPeer),
	)
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.serviceManager = &service.Manager{}
	node.serviceMessageChan = make(map[service.Type]chan *msg_pb.Message)
	switch node.NodeConfig.Role() {
	case nodeconfig.Validator:
		node.setupForValidator()
	case nodeconfig.ExplorerNode:
		node.setupForExplorerNode()
	}
	node.serviceManager.SetupServiceMessageChan(node.serviceMessageChan)
}

// RunServices runs registered services.
func (node *Node) RunServices() {
	if node.serviceManager == nil {
		utils.Logger().Info().Msg("Service manager is not set up yet.")
		return
	}
	node.serviceManager.RunServices()
}

// StopServices runs registered services.
func (node *Node) StopServices() {
	if node.serviceManager == nil {
		utils.Logger().Info().Msg("Service manager is not set up yet.")
		return
	}
	node.serviceManager.StopServicesByRole([]service.Type{})
}

func (node *Node) networkInfoDHTPath() string {
	return fmt.Sprintf(".dht-%s-%s-c%s",
		node.SelfPeer.IP,
		node.SelfPeer.Port,
		node.chainConfig.ChainID,
	)
}
