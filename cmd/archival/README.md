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

#### Archival node or light node

For each shard chain or beacon chain, we need to run a few nodes (let's call archival node or light node) joining that shard or beacon shard and receive new block proposal from the leader of that shard.

Followings are the set of services the archival node needs to run

- NetworkInfo, Discovery.
- Implement receiving new block in node_handler.
- Currently leaders in shard chain or beacon chain only broadcast only to validators in their shard. We may need to modify some logic in discovery to make sure the archival node will receive the broadcast.

#### Other functionality.

Though the archival nodes do not participate into consensus but the keep the blockchain data, they can function as a light node, answering any type of READ transactions about the blockchain like reading balance of an address, read data state of a deployed smart contract.
