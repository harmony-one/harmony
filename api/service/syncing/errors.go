package syncing

import "errors"

// Errors ...
var (
	ErrRegistrationFail      = errors.New("[SYNC]: registration failed")
	ErrGetBlock              = errors.New("[SYNC]: get block failed")
	ErrGetBlockHash          = errors.New("[SYNC]: get blockhash failed")
	ErrProcessStateSync      = errors.New("[SYNC]: get blockhash failed")
	ErrGetConsensusHashes    = errors.New("[SYNC]: get consensus hashes failed")
	ErrGenStateSyncTaskQueue = errors.New("[SYNC]: generate state sync task queue failed")
	ErrDownloadBlocks        = errors.New("[SYNC]: get download blocks failed")
	ErrUpdateBlockAndStatus  = errors.New("[SYNC]: update block and status failed")
	ErrGenerateNewState      = errors.New("[SYNC]: get generate new state failed")
)
