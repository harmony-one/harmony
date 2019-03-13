# Archival Node Design

## Background

Towards testnet launch, main goals are early dev/community engagement and start a long-running network. The network will be launched in one shard beacon chain only.

In mainnet launch, we expect to launch one shard beacon chain and at least 2 regular shards. In this model, one beacon chain shard will be launched at first and later new nodes will get launched next and do staking with the beacon chain. In the next epoch, the beacon chain generates the randomness and do resharding of all nodes based on the randomness and cuckoo rules. All nodes will do the another stage peer discovery and role transformation to form new shard distribuion in the next epoch.

## Goal

Blockchain data archival is very important for main launch as this is the last option to recover the network if any emergency happens such as the network getting shutdown or restarted.

The goal of this effort is to design archival system run by Harmony and separated from Harmony blockchain to backup block chain of beacon chain and other shard chains in both testnet (beacon chain only) and mainnet (1 + 2 model).

Specifically, if we run in testnet with on beacon chain we want to have a system to back up all data of beacon chain on the fly. And if we run in mainnet with beacon chain and N other shard chain, we want to back up all data of beacon chain and shard chain each.

## Architecture Design

### First proposal

#### Archival node or backup node

For each shard chain or beacon chain, we need to run a few nodes (let's call archival node or backup node) joining that shard or beacon shard and receive new block proposal from the leader of that shard.

Followings are the set of functions required by archival node. 

- NetworkInfo, Discovery.
- Receive new block that reaches final consensus by the shard.
- Keep sync with latest blockchain if backup node is offline or due to slow network condition
- Save and flush the received new block into EBS 
- Save the latest blockchain into S3 every N days (N=1) as long term storage
- Accept recover requests from other nodes and broadcast backup blockchain data when the whole network is down

#### Other functionality.

Though the archival nodes do not participate into consensus but the keep the blockchain data, they can function as a light node, answering any type of READ transactions about the blockchain like reading balance of an address, read data state of a deployed smart contract.