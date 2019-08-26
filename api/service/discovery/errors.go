package discovery

import "errors"

// Errors of peer discovery
var (
	ErrGetPeers       = errors.New("[DISCOVERY]: get peer list failed")
	ErrConnectionFull = errors.New("[DISCOVERY]: node's incoming connection full")
	ErrPing           = errors.New("[DISCOVERY]: ping peer failed")
	ErrDHTBootstrap   = errors.New("[DISCOVERY]: DHT bootstrap failed")
)
