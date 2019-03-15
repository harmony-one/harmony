package message

import "errors"

// Error of host package
var (
	ErrWrongMessage           = errors.New("Error as receiving wrong message")
	ErrEnterMethod            = errors.New("Error when processing enter method")
	ErrResultMethod           = errors.New("Error when processing result/getPlayers method")
	ErrEnterProcessorNotReady = errors.New("Error because enter processor is not ready")
)
