package node

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	"github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/explorer"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

func (node *Node) setupForValidator() {
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
	// Register explorer service.
	node.serviceManager.RegisterService(
		service.SupportExplorer, explorer.New(&node.SelfPeer),
	)
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() error {
	node.serviceManager = &service.Manager{}
	node.serviceMessageChan = make(map[service.Type]chan *msg_pb.Message)

	groups := []nodeconfig.GroupID{
		node.NodeConfig.GetShardGroupID(),
		nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID),
		node.NodeConfig.GetClientGroupID(),
	}

	// force the side effect of topic join
	if err := node.host.SendMessageToGroups(groups, []byte{}); err != nil {
		return err
	}

	switch node.NodeConfig.Role() {
	case nodeconfig.Validator:
		node.setupForValidator()
	case nodeconfig.ExplorerNode:
		node.setupForExplorerNode()
	}
	node.serviceManager.SetupServiceMessageChan(node.serviceMessageChan)
	return nil
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
