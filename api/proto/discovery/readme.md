## Ping

Ping message sent by new node who wants to join in the consensus.
The message is broadcasted to the shard.
It contains the public BLS key of consensus and the peerID of the node.
For backward compatibility, it still contains the IP/Port of the node,
but it will be removed once the full integration of libp2p is finished as the IP/Port is not needed.

It also contains a Role field to indicate if the node is a client node or regular node, as client node
won't join the consensus.

## TODO

The following two todo should be worked on once we have full libp2p integration.
For network security reason, we should in general not expose the IP/Port of the node.

-[] remove peer info in Ping message, only keep peerID, which should be sufficient for p2p communication.
