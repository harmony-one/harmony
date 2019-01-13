package syncing

import "errors"

// Errors ...
var (
	ErrSyncPeerConfigClientNotReady = errors.New("client is not ready")
	ErrRegistrationFail             = errors.New("registration failed")
)
