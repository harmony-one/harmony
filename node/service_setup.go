package node

import (
	"fmt"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/blockproposal"
	"github.com/harmony-one/harmony/api/service/consensus"
	"github.com/harmony-one/harmony/api/service/explorer"
)

// RegisterValidatorServices register the validator services.
func (node *Node) RegisterValidatorServices() {
	// Register consensus service.
	node.serviceManager.Register(
		service.Consensus,
		consensus.New(node.Consensus),
	)
	// Register new block service.
	node.serviceManager.Register(
		service.BlockProposal,
		blockproposal.New(node.Consensus, node.WaitForConsensusReadyV2),
	)
}

// RegisterExplorerServices register the explorer services
func (node *Node) RegisterExplorerServices() {
	// Register explorer service.
	node.serviceManager.Register(
		service.SupportExplorer, explorer.New(node.HarmonyConfig, &node.SelfPeer, node.Blockchain(), node),
	)
}

// RegisterService register a service to the node service manager
func (node *Node) RegisterService(st service.Type, s service.Service) {
	node.serviceManager.Register(st, s)
}

// StartServices runs registered services.
func (node *Node) StartServices() error {
	return node.serviceManager.StartServices()
}

// StopServices runs registered services.
func (node *Node) StopServices() error {
	return node.serviceManager.StopServices()
}

func (node *Node) networkInfoDHTPath() string {
	return fmt.Sprintf(".dht-%s-%s-c%s",
		node.SelfPeer.IP,
		node.SelfPeer.Port,
		node.chainConfig.ChainID,
	)
}
