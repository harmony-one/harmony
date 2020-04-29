package apiv1

import "errors"

var (
	// ErrInvalidLogLevel when invalid log level is provided
	ErrInvalidLogLevel = errors.New("invalid log level")
	// ErrIncorrectChainID when ChainID does not match running node
	ErrIncorrectChainID = errors.New("incorrect chain id")
	// ErrInvalidChainID when ChainID of signer does not match that of running node
	ErrInvalidChainID = errors.New("invalid chain id for signer")
	// ErrNotBeaconChainShard when rpc is called on not beacon chain node
	ErrNotBeaconChainShard = errors.New("cannot call this rpc on non beaconchain node")
)
