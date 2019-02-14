package p2p

import "errors"

// Error of host package
var (
	ErrNewStream    = errors.New("[HOST]: new stream error")
	ErrMsgWrite     = errors.New("[HOST]: send message write error")
	ErrAddProtocols = errors.New("[HOST]: cannot add protocols")
	ErrFindPeers    = errors.New("[HOST]: cannot discover peers")
	ErrPeerNotFound = errors.New("[HOST]: requested peer not found")
)
