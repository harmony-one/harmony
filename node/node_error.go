package node

import "errors"

var (
	// ErrLotteryAppFailed is the error when a transaction failed to process lottery app.
	ErrLotteryAppFailed = errors.New("Failed to process lottery app transaction")
)
