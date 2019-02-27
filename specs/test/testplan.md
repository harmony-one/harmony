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
* description : beacon chain node ss
* test procedure : one new beacon node join in the beacon chain after beacon chain reach a few consensuses
* passing criteria : the new node can join in the consensus after state syncing
* dependency
* note
* automated?
---
* test case # : SS2
* description :
* test procedure :
* passing criteria
* dependency
* note
* automated?
---
### consensus

* test case # : CS1
* description : beacon chain reach consensus
* test procedure : start beacon chain with 50, 100, 150, 200, 250, 300 nodes, start txgen for 300 seconds, check leader log on number of consensuses
* passing criteria
* dependency
* note
* automated?
---
* test case # : CS2
* description :
* test procedure :
* passing criteria
* dependency
* note
* automated?
---
### drand

* test case # : DR1
* description : drand generate random number
* test procedure : start beacon chain with 50, 150, 300 nodes, start txgen for 300 seconds, check leader log on the success of generating random number
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
* test procedure :
* passing criteria
* dependency
* note
* automated?

### networking level stress
### transaction stress

* test case # : STX1
* description : txgen send transaction to shard
* test procedure : started beacon chain with 50 nodes, start txgen to send 1,000, 10,000 tx to the shard
* passing criteria
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
