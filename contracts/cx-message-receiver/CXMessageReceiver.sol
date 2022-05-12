pragma solidity ^0.8.13;

type shardId is uint32;

abstract contract CXMessageReceiver {
    function recvCrossShardMessage(address fromAddr,
                                   shardId fromShard,
                                   bytes calldata payload) virtual public;
}

// vim: set ts=4 sw=4 et :
