# Harmony blockchain test plan

We list the major testing area and the test cases.
For each test case, it should have a description of the test case, expected behavior and passing criteria.

## test case template

### template
* test case # :
* description :
* test procedure :
* passing criteria
* dependency
* note
* automated?
---

## functional test
Functional test is used to validate the function of each component in the blockchain.
It should cover the basic function to pass, to fail, and error conditions.

### peer discovery

* test case # : PD1 
* description : find peers using rendezvous string
* test procedure : a new node connected to bootnode and call FindPeers function to return a list of peers using different rendezvous strings
* passing criteria : new node can get a list of peers using the same rendezvous string
* dependency
* note
* automated? N
---
* test case # : PD2
* description : boot node recovery
* test procedure : start a bootnode, start a few new node to connect to bootnode, restart the bootnode, start a few new node to discover peers
* passing criteria : bootnode should be able to store previously connected peers
* dependency
* note
* automated? N
---
* test case # : PD3
* description : multiple boot nodes
* test procedure : start 3-5 bootnode, start a few new node to connect to all the bootnodes for peer discovery
* passing criteria : new node can get a list of peers using the same rendezvous string
* dependency : redundant peers maybe returned and we should handle the reduanancy (or not)
* note
* automated? N
---
* test case # : PD4
* description : number of peers supported in bootnode
* test procedure : start a bootnode, connect up to 5000 new node using the same rendezvous string
* passing criteria : each new node should get a list of peers, not sure how many though
* dependency
* note
* automated? N
---

### state syncing

* test case # : SS1
* description : node state sync basic
* test procedure :  node joins network and is able to sync to latest block
* passing criteria : blockheight of node is equal to that of the shards blockchain and node has joined consensus.
* dependency
* note
* automated? N
---
* test case # : SS2
* description : network connectivity issues 
* test procedure :  node experiences network connectivity issues or is down and then regains connectivity
* passing criteria : the node is able to sync back to current state of blockchain from the point where it dropped off
* dependency
* note
* automated? N
---
* test case # : SS1
* description : beacon chain node ss
* test procedure : one new beacon node join in the beacon chain after beacon chain reach a few consensuses
* passing criteria : the new node can join in the consensus after state syncing
* dependency
* note
* automated? N
---

### consensus

* test case # : CS1
* description : beacon chain reach consensus
* test procedure : start beacon chain with 50, 100, 150, 200, 250, 300 nodes, check leader log on number of consensuses
* passing criteria
* dependency
* note
* automated?
---

### drand

* test case # : DR1
* description : drand generate random number
* test procedure : start beacon chain with 50, 150, 300 nodes, check leader log on the success of generating random number
* passing criteria : random number genreated
* dependency
* note
* automated?
---
* test case # : DR2
* description :
* test procedure :
* passing criteria
* dependency
* note
* automated?
---

### smartcontract

* test case # : SC1
* description : deploy ERC20 smart contract in 1 shard
* test procedure : smart contract is deployed in a automated fashion e.g. erc20 (utility) token contract
* passing criteria: ability to interact with smart contract e.g. transfer erc20 tokens from one address to another
* dependency
* note
* automated?
---
* test case # : SC2
* description : deploy ERC721 (non-fungible tokens) smart contract in 1 shard
* test procedure : smart contract is deployed in a automated fashion e.g. smart contract like Cryptokitties
* passing criteria: ability to succesfully transfer the non-fungible token from one address to another via transaction
* dependency
* note
* automated?
---

### staking

* test case # : SK1
* description : 
* test procedure :
* passing criteria
* dependency
* note
* automated?
---
* test case # : SK2
* description :
* test procedure :
* passing criteria
* dependency
* note
* automated?
---

### resharding

* test case # : RS1
* description : reshard with 1 shard only
* test procedure : start beacon chain with 50 nodes, wait for resharding after 5 blocks
* passing criteria : a new leader should be selected
* dependency
* note
* automated?
---
* test case # : RS2
* description : reshard with 2 shards 
* test procedure : 0 new nodes join the network for the new epoch
* passing criteria : a new leader should be selected
* dependency
* note
* automated?
---
* test case # : RS3
* description : reshard with 2 shards with 250 nodes each
* test procedure : 50 to 250 new nodes join the network for the new epoch
* passing criteria : a new leader should be selected
* dependency
* note
* automated?
---

### transaction

* test case # : TX1
* description : wallet send transaction to blockchain
* test procedure : start beacon chain with 50 nodes, send one transaction from wallet
* passing criteria : transaction is recorded in blockchain
* dependency
* note
* automated?
---

### view change

* test case # : VC1
* description : change leader after resharding
* test procedure : after resharding, new leader was selected and changed
* passing criteria : consensus keeps going
* dependency : resharding
* note
* automated?
---
* test case # : VC2
* description : change malicious leader
* test procedure : started beacon chain with 50 nodes, leader started consensus and offline, after sometime, a new leader is selected
* passing criteria : new leader keeps the consensus going
* dependency : resharding
* note
* automated?
---

## stress test

### protocol level stress

* test case # : STP1
* description : 
* test procedure : increase number of txns in block from 1000 to 10,000, stepwise
* evaluation: change in block latency per shard/change in transactions per sec, per shard.
* dependency
* note
* automated?
---
* test case # : STP2
* description : increasing number of nodes in a shard
* test procedure : increase number of nodes in a shard from 100 to 1000
* evaluation: change in latency per shard/change in transactions per sec, per shard.
* dependency
* note
* automated?
---
* test case # : STP3
* description :  epoch change with increasing number of nodes in shard
* test procedure : initiate epoch change with increasing number of nodes in a shard from 100 to 1000
* evaluation: latency in leader election
* dependency
* note
* automated?
---

## epoch change stress

* test case # : EC1
* description : hight waiting nodes
* test procedure : for 5 shards having 200 nodes each initiate epoch change with 10,000 nodes waiting
* evaluation: latency in leader election/resharding
* dependency
* note
* automated?
---

### networking level stress

* test case # : NT1
* description : 
* test procedure : start consensus with a peer-p2p network of 50 peers, increase it to 1000 peers, send 1 ping message every 100 ms.
* evaluation: change in latency per shard/change in transactions per sec, per shard.
* dependency
* note
* automated?
---

### long running stress
### storage

## security test
### malicious node
### DoS attack

## stability test
### 
