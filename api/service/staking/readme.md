### Staking Service

Staking service is used to send a stake to beacon chain to get a ticket to join Harmony blockchain.

Staking service first gets beacon info which is a result of networkinfo service, then create a staking transaction and send it to beacon chain. As we are currently using ethereum account model, the staking transaction is an ethereum transaction. In ethereum, to create a transaction, a client needs to call to Ethereum network to get the current account nonce (required), suggested gas (optional), and chainID (optional).

For development purpose, after getting beacon info, the staking service of the new node should send a RPC request to beacon chain to get the account nonce as the beacon chain already has a client service which serves that request. In the long term, we should switch to send a gossip request to beancon chain instead of a RPC call.

**TODO**: Rework on this matter.
