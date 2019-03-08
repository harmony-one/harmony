The smart contract files in this folder contains protocol-level smart contracts that are critical to the overall operation of Harmony protocol:

* Faucet.sol is the smart contract to dispense free test tokens in our testnet.
* StakeLockContract.sol is the staking smart contract that receives and locks stakes. The stakes are used for the POS and sharding protocol.

Solc is needed to recompile the contracts into ABI and bytecode. Please follow https://solidity.readthedocs.io/en/v0.5.3/installing-solidity.html for the installation.

Example command to compile a contract file into golang ABI.
```bash

abigen -sol contracts/StakeLockContract.sol -pkg contracts -out contracts/StakeLockContract.go

```