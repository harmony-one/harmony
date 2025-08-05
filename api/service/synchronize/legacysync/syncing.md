### Full state syncing

A node downloads all the missing blocks until it catches up with the block that is in the process of consensus.

### Node states

The states of a node have the following options:

NodeInit, NodeWaitToJoin, NodeNotInSync, NodeOffline, NodeReadyForConsensus, NodeDoingConsensus

When any node joins the network, it will join the shard and try to participate in the consensus process. It will assume its status is NodeReadyForConsensus until it finds it is not able to verify the new block. Then it will move its status into NodeNotInSync. After finish the syncing process, its status becomes NodeReadyForConsensus again. Simply speaking, most of the time, its status is jumping between these two states.

### Doing syncing

Syncing process consists of 3 parts: download the old blocks that have timestamps before state syncing beginning time; register to a few peers (full node) and accept new blocks that have timestampes after state syncing beginning time; catch the last mile blocks from consensus process when its latest block is only 1~2 blocks behind the current consensus block.
