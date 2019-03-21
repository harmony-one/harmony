package node

import (
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/clientsupport"
	"github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/discovery"
	"github.com/harmony-one/harmony/api/service/explorer"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	"github.com/harmony-one/harmony/api/service/randomness"
	"github.com/harmony-one/harmony/api/service/restclientsupport"
	"github.com/harmony-one/harmony/api/service/staking"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

func (node *Node) setupForShardLeader() {
	nodeConfig, chanPeer := node.initNodeConfiguration(false, false)

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))

	// Register explorer service.
	node.serviceManager.RegisterService(service.SupportExplorer, explorer.New(&node.SelfPeer, node.Consensus.GetNumPeers))
	// Register consensus service.
	node.serviceManager.RegisterService(service.Consensus, consensus.New(node.BlockChannel, node.Consensus, node.startConsensus))
	// Register new block service.
	node.serviceManager.RegisterService(service.BlockProposal, blockproposal.New(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register randomness service
	node.serviceManager.RegisterService(service.Randomness, randomness.New(node.DRand))
}

func (node *Node) setupForShardValidator() {
	nodeConfig, chanPeer := node.initNodeConfiguration(false, false)

	// Register client support service.
	node.serviceManager.RegisterService(service.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register peer discovery service. "0" is the beacon shard ID. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))
}

func (node *Node) setupForBeaconLeader() {
	nodeConfig, chanPeer := node.initNodeConfiguration(true, false)

	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, nil))
	// Register networkinfo service.
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))
	// Register consensus service.
	node.serviceManager.RegisterService(service.Consensus, consensus.New(node.BlockChannel, node.Consensus, node.startConsensus))
	// Register new block service.
	node.serviceManager.RegisterService(service.BlockProposal, blockproposal.New(node.Consensus.ReadySignal, node.WaitForConsensusReady))
	// Register client support service.
	node.serviceManager.RegisterService(service.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// TODO(minhdoan): We will remove the old client support and use the new client support which uses new message protocol.
	// Register client new support service.
	node.serviceManager.RegisterService(service.RestClientSupport, restclientsupport.New(
		node.CreateTransactionForEnterMethod, node.GetResult, node.CreateTransactionForPickWinner))
	// Register randomness service
	node.serviceManager.RegisterService(service.Randomness, randomness.New(node.DRand))
	// Register explorer service.
	node.serviceManager.RegisterService(service.SupportExplorer, explorer.New(&node.SelfPeer, node.Consensus.GetNumPeers))
}

func (node *Node) setupForBeaconValidator() {
	nodeConfig, chanPeer := node.initNodeConfiguration(true, false)

	// Register client support service.
	node.serviceManager.RegisterService(service.ClientSupport, clientsupport.New(node.blockchain.State, node.CallFaucetContract, node.getDeployedStakingContract, node.SelfPeer.IP, node.SelfPeer.Port))
	// Register peer discovery service. No need to do staking for beacon chain node.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, nil))
	// Register networkinfo service.
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))
}

func (node *Node) setupForNewNode() {
	// TODO determine the role of new node, currently assume it is beacon node
	nodeConfig, chanPeer := node.initNodeConfiguration(true, true)

	// Register staking service.
	node.serviceManager.RegisterService(service.Staking, staking.New(node.host, node.AccountKey, node.beaconChain, node.NodeConfig.ConsensusPubKey.GetAddress()))
	// Register peer discovery service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))

	// TODO: how to restart networkinfo and discovery service after receiving shard id info from beacon chain?
}

func (node *Node) setupForClientNode() {
	nodeConfig, chanPeer := node.initNodeConfiguration(false, true)

	// Register peer discovery service.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))
}

func (node *Node) setupForArchivalNode() {
	nodeConfig, chanPeer := node.initNodeConfiguration(false, false)
	// Register peer discovery service.
	node.serviceManager.RegisterService(service.PeerDiscovery, discovery.New(node.host, nodeConfig, chanPeer, node.AddBeaconPeer))
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(service.NetworkInfo, networkinfo.New(node.host, p2p.GroupIDBeacon, chanPeer, nil))
	//TODO: Add Syncing as a service.
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.serviceManager = &service.Manager{}
	node.serviceMessageChan = make(map[service.Type]chan *msg_pb.Message)
	switch node.NodeConfig.Role() {
	case nodeconfig.ShardLeader:
		node.setupForShardLeader()
	case nodeconfig.ShardValidator:
		node.setupForShardValidator()
	case nodeconfig.BeaconLeader:
		node.setupForBeaconLeader()
	case nodeconfig.BeaconValidator:
		node.setupForBeaconValidator()
	case nodeconfig.NewNode:
		node.setupForNewNode()
	case nodeconfig.ClientNode:
		node.setupForClientNode()
	case nodeconfig.ArchivalNode:
		node.setupForArchivalNode()
	}
	node.serviceManager.SetupServiceMessageChan(node.serviceMessageChan)
}

// RunServices runs registered services.
func (node *Node) RunServices() {
	if node.serviceManager == nil {
		utils.GetLogInstance().Info("Service manager is not set up yet.")
		return
	}
	node.serviceManager.RunServices()
}

// StopServices runs registered services.
func (node *Node) StopServices() {
	if node.serviceManager == nil {
		utils.GetLogInstance().Info("Service manager is not set up yet.")
		return
	}
	node.serviceManager.StopServicesByRole([]service.Type{})
}
