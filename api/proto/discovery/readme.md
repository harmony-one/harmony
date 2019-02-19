## Ping

Ping message sent by new node who wants to join in the consensus.
The message is broadcasted to the shard.
It contains the public BLS key of consensus and the peerID of the node.
For backward compatibility, it still contains the IP/Port of the node,
but it will be removed once the full integration of libp2p is finished as the IP/Port is not needed.

It also contains a Role field to indicate if the node is a client node or regular node, as client node
won't join the consensus.

## Pong

Pong message is sent by leader to all validators, once the leader has enough validators.
It contains a list of peers and the corresponding BLS public keys.
Noted, the list of peers may not be needed once we have libp2p fully integrated.
The order of the peers and keys are important to the consensus.

At bootstrap, the Pong message is sent out and then the consensus should start.

## TODO

The following two todo should be worked on once we have full libp2p integration.
For network security reason, we should in general not expose the IP/Port of the node.

-[] remove peer info in Ping message, only keep peerID, which should be sufficient for p2p communication.
-[] remove peer list from Pong message.
