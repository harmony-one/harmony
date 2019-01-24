# Design of Beacon

## Beacon Chain and Sharding Chain

The beacon chain is in charge of managing the staking procedure for new node participating, randomness genereation, resharding mechanism. Moreover, the beacon blockchain also stores staking tokens

By current design, sharding chain only handle money transactions.

The difference between beacon chain and sharding chain

| Property                  | Beacon Chain          | Sharding Chain     |
| ------------------------- | --------------------- | ------------------ |
| Money transaction         | Not supported         | Supported          |
| Staking token transaction | Supported             | Not Supported      |
| Randomness transaction    | Supported             | Not Supported      |
| Resharding mechanism      | Trigged by this chain | Not supported here |


## Accounts and Transactions

### Externally owned account (EOA)

Currently we adopted ethereum account model by using EOA. We will also support contract account but it is not in our focus as of now.

#### Account state

As we borrow EOA from Ethereum the account state will contain followings:

* Nonce is the number of transactions sent from this address a (EOA) or the number of contract created by this account if it is a contract account.
* Balance: the money of the account in Harmony. **TODO**(figure it out Harmony units like ether etc..)
* storageRoot: a hash of the root node of a Merkle Patricia tree. This tree encodes the hash of the storage contents of this account, and is empty by default.
* codeHash: the hash of the EVM code of this account. For contract accounts, this is the code that gets hashed and stored as the codeHash. For externally owned accounts, the codeHash field is the hash of the empty string.

### Beacon account

As 

* Nonce is the number of transactions sent from this address a (EOA) or the number of contract created by this account if it is a contract account.
* Balance: the money of the account in Harmony. **TODO**(figure it out Harmony units like ether etc..)
* storageRoot: a hash of the root node of a Merkle Patricia tree. This tree encodes the hash of the storage contents of this account, and is empty by default.
* 

### Transactions

#### Transaction for sharding chain

As currently we are using EOA from ethererum - the transacion data model structure is similar to ethereum as followings:

* Nonce: a count of the number of transactions sent by the sender.
* gasPrice: the number of Wei that the sender is willing to pay per unit of gas required to execute the transaction
* gasLimit: the maximum amount of gas that the sender is willing to pay for executing this transaction
* To:
  *  the address of the recipient/contract account
  *  Empty if it is contract creation
* Value: the amount of Wei to be transferred from the sender to the recipient. In a contract-creating transaction, this value serves as the starting balance within the newly created contract account.
* v, r, s: used to generate the signature that identifies the sender of the transaction.
* init (only exists for contract-creating transactions): An EVM code fragment that is used to initialize the new contract account. init is run only once, and then is discarded. When init is first run, it returns the body of the account code, which is the piece of code that is permanently associated with the contract account.
* data (optional field that only exists for message calls): the input data (i.e. parameters) of the message call. For example, if a smart contract serves as a domain registration service, a call to that contract might expect input fields such as the domain and IP address.

#### Transactions in beacon chain

##### Staking transaction
```
message StakingTransaction {
    int64 nonce = 1; // WIP.
    int64 staking_amount = 2; 
    Address from = 3; //
    Address to = 4; // empty for staking
    Signatue signature = 5; //
}
```

This transaction is sent from the new node.

##### Randomness transaction

```
message RandomnessTrasaction {
    int64 epoch = 1; //
}
```

This transaction is triggered by the beacon chain. Since epoch can happen every X hours (X ~ 24) we can send this transaction from a client for development purpose to simulate the epoch transition.

The goal of this transaction is to generate **pRnd** in beacon chain. See **pRnd** in the whitepaper.


## Harmony's PBFT Consensus

Harmony’s FBFT consensus involves the following steps:

1. The leader constructs the new block and broadcasts the block header to all validators. Meanwhile, the leader broadcasts the content of the block with erasure coding. This is called the “announce” phase.
2. The validators check the validity of the block header, sign the block header with a BLS signature, and send the signature back to the leader.
3. The leader waits for at least 2f+1 valid signatures from validators (including the leader itself) and aggregates them into a BLS multi-signature. Then the leader broadcasts the aggregated multi-signature along with a bitmap indicating which validators have signed. Together with Step 2, this concludes the “prepare” phase of PBFT.
4. The validators check that the multi-signature has at least 2f+1 signers, verify the transactions in the block content broadcasted from the leader in Step 1, sign the received message from Step 3, and send it back to the leader.
5. The leader waits for at least 2f+1 valid signatures (can be different signers from Step 3) from Step 4, aggregates them together into a BLS multi-signature, and creates a bitmap logging all the signers. Finally, the leader commits the new block with all the multi-signatures and bitmaps attached, and broadcasts the new block for all validators to commit. Together with Step 4, this concludes the “commit” phase of PBFT.


## Bootstrap

We will start with M+1 shards running at bootstrap where shard 0 is considered as beacon chain and M other shards are numbered from 1 to M. 

As the staking will happen in beacon chain the beacon block should contain accounts where the new node should have claimed to do staking with beacon chain.

1. Provide the communication protocol with new node.
2. How staking happens.
3. Data structure for staking.
4. How to update discovery with DNS Seeder (bootnodes)
5.

## Flow of new node

A new node should have claimed some accounts for staking for participating in the network. For main net, people need to buy stake tokens for their own node before joining the network. For development, we need to hard-cord some staking accounts for new nodes.

The flow of the new node can be as follows:

* A new node first talks (p2p communication) to bootnoode to get routing information about the beacon chain. Currently we are using unit-cast p2p and will switch to multi-cast p2p as soon as it's ready for use.
  * Question: how does the new node receive the confirmation? Need to understand p2p.
  * For first prototype: just use uni-cast
* As a new node gets beacon chain info from bootnode, we can start talking with beacon chain by sending a staking transaction
  * How does the new node get confirmation from beacon chain?
* The new node gets confirmation from beacon chain to become a validator but it won't know which shard it will be.
* The new node needs to wait for the randomness.



Beacon transaction message:

```

enum MessageType {
  UNKNOWN = 0;
  NEWNODE_BOOTNODE = 1;
  BOOTNODE_NEWNODE = 2;
  NEWNODE_BEACON = 3;
  BEACON_NEWNODE = 4;
  ...
  NEWNODE_BEACON_STAKING = 10;
}

message Message {
    MessageType type = 1;
    oneof request {
        NewNodeBootNodeRequest newnode_bootnode_request = 2;
        BootNodeNewNodeRequest bootnode_newnode_request = 3;
        ...
        BeaconStaking staking = 10;
    }
    oneof transaction {
        StakingTransaction staking_txn = 100;
    }
}
```

Beacon leader receives beacon transactions and put them into pending transactions list as a part of its consensus.

As beacon leader receives enought transactions to start the consensus, it will verify all transactions. For Staking transaction, it will verify as follows:


## Messaging

We should use introduced universal message for messaging modeling.

## Sharding chain

Sharding chain runs PBFT consensus. Moreover after each consensus they have to sync header with beacon chain.


## Constants

## Development environment

- M = 3, 1 beacon chain, 2 shard chains.
- Each shard will have N = 10 nodes.

## Testing environment

- M = 10
- Each shard will have N = 300 nodes.

## Terminologies

- Beacon leader: Beacon consensus runs by PBFT where there is one leader and validators as other nodes.
- Beacon validator:

