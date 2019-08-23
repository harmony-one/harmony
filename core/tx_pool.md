# TxPool design
---------------



## Functionalities
1. Being a storage of coming transactions
2. Managing all tx policies/filters (see below)
3. Handling logic of accepting new txs and generating a new block of tx


## Policies

### PriceLimit
This policy is to ensure that minimum gas price of a coming transaction should be enforced for acceptance into the pool.

### PriceBump
This policy is to accept a later transaction from the sender with the same nonce of an accepted transaction and drop that accepted transaction if the gas price of later transaction is (100+PriceBump)% more than the gas price of the accepted transaction.

### AccountSlots
This policy is to limit the number of executable transactions slots per account.

### GlobalSlots
This policy is to limit the number of executable transactions slots of all accounts.

### AccountQueue
This policy is to limit the number of non-executable transactions slots per account. The non-executable transactions are stored in a queue data structure. See later.
### GlobalQueue
This policy is to limit the number of non-executable transactions slots of all accounts. The non-executable transactions are stored in a queue data structure. See later.
### Lifetime
This policy is to limit the amount of time non-executable transactions are queued.

### DropOldTx
This policy is to drop all transactions that are deemed too old

### DropLowBalance
This policy is to drop all transactions that are too costly (low balance or out of gas)

## Data Structure

Coming up later.
