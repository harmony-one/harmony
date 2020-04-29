package apiv1

import (
	"errors"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/shard"
)

var (
	// ErrInvalidLogLevel when invalid log level is provided
	ErrInvalidLogLevel = errors.New("invalid log level")
	// ErrIncorrectChainID when ChainID does not match running node
	ErrIncorrectChainID = errors.New("incorrect chain id")
	// ErrInvalidChainID when ChainID of signer does not match that of running node
	ErrInvalidChainID = errors.New("invalid chain id for signer")
	// ErrNotBeaconShard when rpc is called on not beacon chain node
	ErrNotBeaconShard = errors.New("cannot call this rpc on non beaconchain node")
	// ErrRequestedBlockTooHigh when given block is greater than latest block number
	ErrRequestedBlockTooHigh = errors.New("requested block number greater than current block number")
)

func (s *PublicBlockChainAPI) isBeaconShard() error {
	if s.b.GetShardID() != shard.BeaconChainShardID {
		return ErrNotBeaconShard
	}
	return nil
}

func (s *PublicBlockChainAPI) isBlockGreaterThanLatest(blockNum rpc.BlockNumber) error {
	if uint64(blockNum) >= uint64(s.BlockNumber()) {
		return ErrRequestedBlockTooHigh
	}
	return nil
}
