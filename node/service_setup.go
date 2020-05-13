package node

import (
	"fmt"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	"github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

func (node *Node) setupForValidator() {

	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), nil, node.networkInfoDHTPath(),
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

	// Register networkinfo service.
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), nil, node.networkInfoDHTPath()),
	)
	// Register explorer service.
	node.serviceManager.RegisterService(
		service.SupportExplorer, explorer.New(&node.SelfPeer),
	)
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.initNodeConfiguration()

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
	node.serviceManager.RunServices()
}

// StopServices runs registered services.
func (node *Node) StopServices() {
	node.serviceManager.StopServicesByRole([]service.Type{})
}

func (node *Node) networkInfoDHTPath() string {
	return fmt.Sprintf(".dht-%s-%s-c%s",
		node.SelfPeer.IP,
		node.SelfPeer.Port,
		node.chainConfig.ChainID,
	)
}
