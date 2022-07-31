package stagedsync

import "errors"

// Errors ...
var (
	ErrRegistrationFail                   = errors.New("[STAGED_SYNC]: registration failed")
	ErrGetBlock                           = errors.New("[STAGED_SYNC]: get block failed")
	ErrGetBlockHash                       = errors.New("[STAGED_SYNC]: get block hash failed")
	ErrProcessStateSync                   = errors.New("[STAGED_SYNC]: get block hash failed")
	ErrGetConsensusHashes                 = errors.New("[STAGED_SYNC]: get consensus hashes failed")
	ErrGenStateSyncTaskQueue              = errors.New("[STAGED_SYNC]: generate state sync task queue failed")
	ErrDownloadBlocks                     = errors.New("[STAGED_SYNC]: get download blocks failed")
	ErrUpdateBlockAndStatus               = errors.New("[STAGED_SYNC]: update block and status failed")
	ErrGenerateNewState                   = errors.New("[STAGED_SYNC]: get generate new state failed")
	ErrFetchBlockHashProgressFail         = errors.New("[STAGED_SYNC]: fetch cache progress for block hashes stage failed")
	ErrFetchCachedBlockHashFail           = errors.New("[STAGED_SYNC]: fetch cached block hashes failed")
	ErrNotEnoughBlockHashes               = errors.New("[STAGED_SYNC]: peers haven't sent all requested block hashes")
	ErrRetrieveCachedProgressFail         = errors.New("[STAGED_SYNC]: retrieving cache progress for block hashes stage failed")
	ErrRetrieveCachedHashProgressFail     = errors.New("[STAGED_SYNC]: retrieving cache progress for block hashes stage failed")
	ErrSaveBlockHashesProgressFail        = errors.New("[STAGED_SYNC]: saving progress for block hashes stage failed")
	ErrSaveCachedBlockHashesProgressFail  = errors.New("[STAGED_SYNC]: saving cache progress for block hashes stage failed")
	ErrSavingCacheLastBlockHashFail       = errors.New("[STAGED_SYNC]: saving cache last block hash for block hashes stage failed")
	ErrCachingBlockHashFail               = errors.New("[STAGED_SYNC]: caching downloaded block hashes failed")
	ErrCommitTransactionFail              = errors.New("[STAGED_SYNC]: failed to write db commit")
	ErrUnexpectedNumberOfBlocks           = errors.New("[STAGED_SYNC]: unexpected number of block delivered")
	ErrSavingBodiesProgressFail           = errors.New("[STAGED_SYNC]: saving progress for block bodies stage failed")
	ErrAddTasksToQueueFail                = errors.New("[STAGED_SYNC]: cannot add task to queue")
	ErrSavingCachedBodiesProgressFail     = errors.New("[STAGED_SYNC]: saving cache progress for blocks stage failed")
	ErrRetrievingCachedBodiesProgressFail = errors.New("[STAGED_SYNC]: retrieving cache progress for blocks stage failed")
	ErrNoConnectedPeers                   = errors.New("[STAGED_SYNC]: haven't connected to any peer yet!")
	ErrSaveStateProgressFail              = errors.New("[STAGED_SYNC]: saving progress for block States stage failed")
	ErrPruningCursorCreationFail          = errors.New("[STAGED_SYNC]: failed to create cursor for pruning")
)
