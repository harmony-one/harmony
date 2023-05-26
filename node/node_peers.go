package node

import (
	"fmt"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// BroadcastPeersExchange broadcasts missing cross shard receipts per request
func (node *Node) BroadcastPeersExchange() {
	groupID := []nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(node.Consensus.ShardID))}
	endpoint := fmt.Sprintf("%s:%d", node.HarmonyConfig.DNSSync.ServerIP, node.HarmonyConfig.DNSSync.ServerPort)
	serializedMessage, err := proto_node.ConstructPeerBroadcastMessage(endpoint, node.Consensus.ShardID)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[BroadcastPeersExchange] failed to construct message")
		return
	}

	go node.host.SendMessageToGroups(groupID, p2p.ConstructMessage(serializedMessage))
}
