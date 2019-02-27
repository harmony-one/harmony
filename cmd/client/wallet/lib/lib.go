package lib

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony/api/client"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

// CreateWalletNode creates wallet server node.
func CreateWalletNode() *node.Node {
	shardIDLeaderMap := make(map[uint32]p2p.Peer)

	//	port, _ := strconv.Atoi("9999")

	// dummy host for wallet
	self := p2p.Peer{IP: "127.0.0.1", Port: "6789"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "6789")
	host, err := p2pimpl.NewHost(&self, priKey)
	if err != nil {
		panic(err)
	}

	/*
		for _, leader := range response.Leaders {
			peerID, err := peer.IDB58Decode(leader.PeerID)
			if err != nil {
				panic(err)
			}
			leaderPeer := p2p.Peer{IP: leader.Ip, Port: leader.Port, PeerID: peerID}
			shardIDLeaderMap[leader.ShardId] = leaderPeer
			host.AddPeer(&leaderPeer)
		}
	*/
	walletNode := node.New(host, nil, nil)
	walletNode.Client = client.NewClient(walletNode.GetHost(), shardIDLeaderMap)
	return walletNode
}

// SubmitTransaction submits the transaction to the Harmony network
func SubmitTransaction(tx *types.Transaction, walletNode *node.Node, shardID uint32) error {
	//	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	//	leader := walletNode.Client.Leaders[shardID]
	//	walletNode.SendMessage(leader, msg)
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	time.Sleep(300 * time.Millisecond)
	return nil
}
