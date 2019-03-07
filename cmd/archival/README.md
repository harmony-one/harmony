# Archival Node Design

## Background

Towards testnet launch, main goals are early dev/community engagement and start a long-running network. The network will be launched in one shard beacon chain only.

In mainnet launch, we expect to launch one shard beacon chain and at least 2 regular shards. In this model, one beacon chain shard will be launched at first and later new nodes will get launched next and do staking with the beacon chain. In the next epoch, the beacon chain generates the randomness and do resharding of all nodes based on the randomness and cuckoo rules. All nodes will do the another stage peer discovery and role transformation to form new shard distribuion in the next epoch.

## Goal

Blockchain data archival is very important for main launch as this is the last option to recover the network if any emergency happens such as the network getting shutdown or restarted.

The goal of this effort is to design archival system run by Harmony to store block chain of beacon chain and other shard chains in both testnet (beacon chain only) and mainnet (1 + 2 model).

Specifically, if we run in testnet with on beacon chain we want to have a system to back up all data of beacon chain on the fly. And if we run in mainnet with beacon chain and N other shard chain, we want to back up all data of beacon chain and shard chain each.

## Architecture Design

### How data is stored currently in beacon chain and shard chain

Currently our code reuse all blockchain, block, account model data structure, hence all data are stored by leveldb. There are 3 three roots stateRoot, transactionRoot, receiptsRoot stored in both memory in current block and in db for all blocks.

The operations to store the data are implemented with following interface in rawdb package

```
// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Put(key []byte, value []byte) error
}

// DatabaseDeleter wraps the Delete method of a backing data store.
type DatabaseDeleter interface {
	Delete(key []byte) error
}
```

Put and Delete are two methods to modify data in db.

### First proposal

For each shard chain or beacon chain we'd set up a leveldb db and deploy a corresponding server which listen to receive Put and Delete operation together with key and value data, then execute the same operation with the same key, value in the leveldb db.

#### What needs to implement in our code

- Beacon chain or shard chain should be able to access the IP/Port of its corresponding server (backup server) to back up its data. The server info (IP/Port) can be stored in bootnode or hard-code.
- We should set up a client to the backup server.
- We need to extend the implemenation of Put and Delete interface to make an RPC call to the backup server before executing Put or Delete.
- Keep in mind that beacon chain only has one leveldb but the shard chain currently has one leveldb for itself and another leveldb for syncing beacon chain.

#### What needs to implement in backup server node

- Set up an gRPC server.
- Send server info to bootnode or an centralized server.
- Implement the Put and Delete logic when receiving messages from beacon chain or shard chain.

Protobuf code for gRPC server:

```
syntax = "proto3";

package backup;

// BackupService services Put and Delete operation in backup node.
service BackupService {
  rpc Put(PutRequest) returns (PutResponse) {}
  rpc Delete(DeleteRequest) returns (DeleteResponse) {}
}

// PutRequest is the request of Put operation.
message PutRequest {
  bytes key = 1;
  bytes value = 2;
}

message PutResponse {
}

// DeleteRequest is the request of Delete operation.
message DeleteRequest {
  bytes key = 1;
}

message DeleteResponse {
}

```
