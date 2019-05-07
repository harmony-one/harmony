package node

import "errors"

var (
	// ErrLotteryAppFailed is the error when a transaction failed to process lottery app.
	ErrLotteryAppFailed = errors.New("Failed to process lottery app transaction")
	// ErrPuzzleInsufficientFund is the error when a user does not have sufficient fund to enter.
	ErrPuzzleInsufficientFund = errors.New("You do not have sufficient fund to play")
)
