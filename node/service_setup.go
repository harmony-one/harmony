package node

import (
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	"github.com/harmony-one/harmony/api/service/explorer"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/shard"
)

func (node *Node) setupForValidator() {

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
		service.SupportExplorer, explorer.New(
			node.SelfPeer, node.NodeConfig.DBDir,
		),
	)
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() error {
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
	node.serviceManager.RunServices()
	return nil
}
