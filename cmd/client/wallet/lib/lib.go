package lib

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
	return walletNode
}

// GetPeersFromBeaconChain get peers from beacon chain
// TODO: add support for normal shards
func GetPeersFromBeaconChain(walletNode *node.Node) []p2p.Peer {
	peers := []p2p.Peer{}

	time.Sleep(4 * time.Second)
	walletNode.BeaconNeighbors.Range(func(k, v interface{}) bool {
		peers = append(peers, v.(p2p.Peer))
		return true
	})
	return peers
}

// SubmitTransaction submits the transaction to the Harmony network
func SubmitTransaction(tx *types.Transaction, walletNode *node.Node, shardID uint32) error {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	walletNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	return nil
}

func getBootNodes() []ma.Multiaddr {
	addrStrings := []string{"/ip4/127.0.0.1/tcp/19876/p2p/QmbTLMb9C8dmjrDYoiJb2mayXspcURkNB4ARxgsoA5Aoe3"}
	//addrStrings := []string{"/ip4/54.213.43.194/tcp/9874/p2p/QmQhPRqqfTRExqWmTifjMaBvRd3HBmnmj9jYAvTy6HPPJj"}
	bootNodeAddrs, err := utils.StringsToAddrs(addrStrings)
	if err != nil {
		panic(err)
	}
	return bootNodeAddrs
}
