package stagedsync

import (
	"fmt"
)

// Errors ...
var (
	ErrRegistrationFail                   = WrapStagedSyncError("registration failed")
	ErrGetBlock                           = WrapStagedSyncError("get block failed")
	ErrGetBlockHash                       = WrapStagedSyncError("get block hash failed")
	ErrGetConsensusHashes                 = WrapStagedSyncError("get consensus hashes failed")
	ErrGenStateSyncTaskQueue              = WrapStagedSyncError("generate state sync task queue failed")
	ErrDownloadBlocks                     = WrapStagedSyncError("get download blocks failed")
	ErrUpdateBlockAndStatus               = WrapStagedSyncError("update block and status failed")
	ErrGenerateNewState                   = WrapStagedSyncError("get generate new state failed")
	ErrFetchBlockHashProgressFail         = WrapStagedSyncError("fetch cache progress for block hashes stage failed")
	ErrFetchCachedBlockHashFail           = WrapStagedSyncError("fetch cached block hashes failed")
	ErrNotEnoughBlockHashes               = WrapStagedSyncError("peers haven't sent all requested block hashes")
	ErrRetrieveCachedProgressFail         = WrapStagedSyncError("retrieving cache progress for block hashes stage failed")
	ErrRetrieveCachedHashProgressFail     = WrapStagedSyncError("retrieving cache progress for block hashes stage failed")
	ErrSaveBlockHashesProgressFail        = WrapStagedSyncError("saving progress for block hashes stage failed")
	ErrSaveCachedBlockHashesProgressFail  = WrapStagedSyncError("saving cache progress for block hashes stage failed")
	ErrSavingCacheLastBlockHashFail       = WrapStagedSyncError("saving cache last block hash for block hashes stage failed")
	ErrCachingBlockHashFail               = WrapStagedSyncError("caching downloaded block hashes failed")
	ErrCommitTransactionFail              = WrapStagedSyncError("failed to write db commit")
	ErrUnexpectedNumberOfBlocks           = WrapStagedSyncError("unexpected number of block delivered")
	ErrSavingBodiesProgressFail           = WrapStagedSyncError("saving progress for block bodies stage failed")
	ErrAddTasksToQueueFail                = WrapStagedSyncError("cannot add task to queue")
	ErrSavingCachedBodiesProgressFail     = WrapStagedSyncError("saving cache progress for blocks stage failed")
	ErrRetrievingCachedBodiesProgressFail = WrapStagedSyncError("retrieving cache progress for blocks stage failed")
	ErrNoConnectedPeers                   = WrapStagedSyncError("haven't connected to any peer yet!")
	ErrNotEnoughConnectedPeers            = WrapStagedSyncError("not enough connected peers")
	ErrSaveStateProgressFail              = WrapStagedSyncError("saving progress for block States stage failed")
	ErrPruningCursorCreationFail          = WrapStagedSyncError("failed to create cursor for pruning")
	ErrInvalidBlockNumber                 = WrapStagedSyncError("invalid block number")
	ErrInvalidBlockBytes                  = WrapStagedSyncError("invalid block bytes to insert into chain")
	ErrAddTaskFailed                      = WrapStagedSyncError("cannot add task to queue")
	ErrNodeNotEnoughBlockHashes           = WrapStagedSyncError("some of the nodes didn't provide all block hashes")
	ErrCachingBlocksFail                  = WrapStagedSyncError("caching downloaded block bodies failed")
	ErrSaveBlocksFail                     = WrapStagedSyncError("save downloaded block bodies failed")
	ErrStageNotFound                      = WrapStagedSyncError("stage not found")
	ErrSomeNodesNotReady                  = WrapStagedSyncError("some nodes are not ready")
	ErrSomeNodesBlockHashFail             = WrapStagedSyncError("some nodes failed to download block hashes")
	ErrMaxPeerHeightFail                  = WrapStagedSyncError("get max peer height failed")
)

// WrapStagedSyncError wraps errors for staged sync and returns error object
func WrapStagedSyncError(context string) error {
	return fmt.Errorf("[STAGED_SYNC]: %s", context)
}
