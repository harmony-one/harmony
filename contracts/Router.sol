pragma solidity ^0.8.13;

type shardId is uint32;

// Interface for the router contract, used for cross-shard messaging. The
// implementation is written in Go, in `core/vm/contracts_write.go`
//
// TODO: document methods in detail. For now see
// <https://gitlab.com/mukn/harmony1-proposal>.
abstract contract Router {

    // Send a message to a contract on another shard.
    function send(address to_,
                  shardId toShard,
                  bytes calldata payload,
                  uint gasBudget,
                  uint gasPrice,
                  uint gasLimit,
                  address gasLeftoverTo) virtual public returns (address msgAddr);

    // Re-try sending a message, supplying additional funds for gas.
    function retrySend(address msgAddr,
                       uint gasLimit,
                       uint gasPrice) virtual public;
}

// vim: set ts=4 sw=4 et :
