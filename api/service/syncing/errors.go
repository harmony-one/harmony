package syncing

import "errors"

// Errors ...
var (
	ErrRegistrationFail = errors.New("[SYNC]: registration failed")
	ErrGetBlock         = errors.New("[SYNC]: get block failed")
	ErrGetBlockHash     = errors.New("[SYNC]: get blockhash failed")
)
