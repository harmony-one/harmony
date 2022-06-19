package stagedsync

import "errors"

// Errors ...
var (
	ErrRegistrationFail      = errors.New("[STAGED_SYNC]: registration failed")
	ErrGetBlock              = errors.New("[STAGED_SYNC]: get block failed")
	ErrGetBlockHash          = errors.New("[STAGED_SYNC]: get blockhash failed")
	ErrProcessStateSync      = errors.New("[STAGED_SYNC]: get blockhash failed")
	ErrGetConsensusHashes    = errors.New("[STAGED_SYNC]: get consensus hashes failed")
	ErrGenStateSyncTaskQueue = errors.New("[STAGED_SYNC]: generate state sync task queue failed")
	ErrDownloadBlocks        = errors.New("[STAGED_SYNC]: get download blocks failed")
	ErrUpdateBlockAndStatus  = errors.New("[STAGED_SYNC]: update block and status failed")
	ErrGenerateNewState      = errors.New("[STAGED_SYNC]: get generate new state failed")
)
