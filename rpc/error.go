package rpc

import (
	"errors"
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
	// ErrUnknownRPCVersion when rpc method has an unknown or unhandled version
	ErrUnknownRPCVersion = errors.New("API service has an unknown version")
	// ErrTransactionNotFound when attempting to get a transaction that does not exist or has not been finalized
	ErrTransactionNotFound = errors.New("transaction not found")
)
