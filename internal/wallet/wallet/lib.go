package wallet

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	ma "github.com/multiformats/go-multiaddr"
)

// CreateWalletNode creates wallet server node.
func CreateWalletNode() *node.Node {
	utils.BootNodes = getBootNodes()
	shardIDs := []uint32{0}

	// dummy host for wallet
	self := p2p.Peer{IP: "127.0.0.1", Port: "6999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "6999")
	host, err := p2pimpl.NewHost(&self, priKey)
	if err != nil {
		panic(err)
	}
	walletNode := node.New(host, nil, nil)
	walletNode.Client = client.NewClient(walletNode.GetHost(), shardIDs)

	walletNode.NodeConfig.SetRole(nodeconfig.ClientNode)
	walletNode.ServiceManagerSetup()
	walletNode.RunServices()
	// wait for networkinfo/discovery service to start fully
	// FIXME (leo): use async mode or channel to communicate
	time.Sleep(2 * time.Second)
	return walletNode
}

// GetPeersFromBeaconChain get peers from beacon chain
// TODO: add support for normal shards
func GetPeersFromBeaconChain(walletNode *node.Node) []p2p.Peer {
	peers := []p2p.Peer{}

	// wait until we got beacon peer
	// FIXME (chao): use async channel for communiation
	time.Sleep(4 * time.Second)
	walletNode.BeaconNeighbors.Range(func(k, v interface{}) bool {
		peers = append(peers, v.(p2p.Peer))
		return true
	})
	utils.GetLogInstance().Debug("GetPeersFromBeaconChain", "peers:", peers)
	return peers
}

// SubmitTransaction submits the transaction to the Harmony network
func SubmitTransaction(tx *types.Transaction, walletNode *node.Node, shardID uint32, stopChan chan struct{}) error {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	err := walletNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
	if err != nil {
		fmt.Printf("Error in SubmitTransaction: %v\n", err)
		return err
	}
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	// FIXME (leo): how to we know the tx was successful sent to the network
	// this is a hacky way to wait for sometime
	time.Sleep(2 * time.Second)
	if stopChan != nil {
		stopChan <- struct{}{}
	}
	return nil
}

func getBootNodes() []ma.Multiaddr {
	// These are the bootnodes of banjo testnet
	addrStrings := []string{"/ip4/100.26.90.187/tcp/9876/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9", "/ip4/54.213.43.194/tcp/9876/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX"}
	bootNodeAddrs, err := utils.StringsToAddrs(addrStrings)
	if err != nil {
		panic(err)
	}
	return bootNodeAddrs
}
