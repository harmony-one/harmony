# JSON RPC

## JSSDK
[FireStack-Lab/Harmony-sdk-core](https://github.com/FireStack-Lab/Harmony-sdk-core)

## JSON-RPC methods

### Network info related
* [ ] net_listening - check if network is connected
* [x] hmy_protocolVersion - check protocol version
* [ ] net_version - get network id
* [ ] net_peerCount - peer count

### BlockChain info related
* [ ] hmy_gasPrice - return min-gas-price
* [ ] hmy_estimateGas - calculating estimate gas using signed bytes
* [ ] hmy_blockNumber - get latest block number
* [x] hmy_getBlockByHash - get block by block hash
* [x] hmy_getBlockByNumber
* [ ] hmy_getUncleByBlockHashAndIndex - get uncle by block hash and index number
* [ ] hmy_getUncleByBlockNumberAndIndex - get uncle by block number and index number
* [ ] hmy_getUncleCountByBlockHash - get uncle count by block hash
* [ ] hmy_getUncleCountByBlockNumber - get uncle count by block number
* [ ] hmy_syncing - Returns an object with data about the sync status 
* [ ] hmy_coinbase - return coinbase address
* [ ] hmy_mining - return if mining client is mining
* [ ] hmy_hashrate - return current hash rate for blockchain


### Account related
* [x] hmy_getBalance - get balance for account address
* [x] hmy_getTransactionCount - get nonce for account address
* [ ] hmy_accounts - return accounts that lives in node

### Transactions related
* [ ] hmy_getTransactionReceipt - get transaction receipt by given transaction hash
* [ ] hmy_sendRawTransaction - send transaction bytes(signed) to blockchain
* [ ] hmy_sendTransaction - send transaction object(with signature) to blockchain
* [x] hmy_getBlockTransactionCountByHash - get transaction count of block by block hash
* [x] hmy_getBlockTransactionCountByNumber - get transaction count of block by block number
* [x] hmy_getTransactionByHash  - get transaction object of block by block hash
* [x] hmy_getTransactionByBlockHashAndIndex  - get transaction object of block by block hash and index number
* [x] hmy_getTransactionByBlockNumberAndIndex - get transaction object of block by block number and index number
* [ ] hmy_sign - sign message using node specific sign method.

### Contract related
* [ ] hmy_call - call contract method 
* [x] hmy_getCode - get deployed contract's byte code 
* [x] hmy_getStorageAt - get storage position at a given address
* ~~[ ] hmy_getCompilers~~ - DEPRECATED
* ~~[ ] hmy_compileLLL~~ - DEPRECATED
* ~~[ ] hmy_compileSolidity~~ - DEPRECATED
* ~~[ ] hmy_compileSerpent~~ - DEPRECATED

### Subscribes
* [ ] hmy_pendingTransactions - pending transaction subscriber
* [ ] hmy_getLogs - log subscriber
* [ ] hmy_newFilter -  creates a filter object, based on filter options
* [ ] hmy_newBlockFilter - creates a filter in the node, to notify when a new block arrives
* [ ] hmy_newPendingTransactionFilter - creates a filter in the node, to notify when new pending transactions arrive
* [ ] hmy_getFilterChanges - polling method for a filter
* [ ] hmy_getFilterLogs - returns an array of all logs matching filter with given id.
* [ ] hmy_uninstallFilter - uninstalls a filter with given id


### Others, not very important for current stage of work
* [ ] web3_clientVersion
* [ ] web3_sha3
* [ ] hmy_getWork
* [ ] hmy_submitWork
* [ ] hmy_submitHashrate
* [ ] hmy_getProof
* [ ] db_putString
* [ ] db_getString
* [ ] db_putHex
* [ ] db_getHex
* [ ] shh_post
* [ ] shh_version
* [ ] shh_newIdentity
* [ ] shh_hasIdentity
* [ ] shh_newGroup
* [ ] shh_addToGroup
* [ ] shh_newFilter
* [ ] shh_uninstallFilter
* [ ] shh_getFilterChanges
* [ ] shh_getMessages














