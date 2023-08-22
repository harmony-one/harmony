package stagedstreamsync

import (
	"fmt"
)

// Errors ...
var (
	ErrSavingBodiesProgressFail      = WrapStagedSyncError("saving progress for block bodies stage failed")
	ErrSaveStateProgressFail         = WrapStagedSyncError("saving progress for block States stage failed")
	ErrInvalidBlockNumber            = WrapStagedSyncError("invalid block number")
	ErrInvalidBlockBytes             = WrapStagedSyncError("invalid block bytes to insert into chain")
	ErrStageNotFound                 = WrapStagedSyncError("stage not found")
	ErrUnexpectedNumberOfBlockHashes = WrapStagedSyncError("unexpected number of getBlocksByHashes result")
	ErrUnexpectedBlockHashes         = WrapStagedSyncError("unexpected get block hashes result delivered")
	ErrNilBlock                      = WrapStagedSyncError("nil block found")
	ErrNotEnoughStreams              = WrapStagedSyncError("number of streams smaller than minimum required")
	ErrParseCommitSigAndBitmapFail   = WrapStagedSyncError("parse commitSigAndBitmap failed")
	ErrVerifyHeaderFail              = WrapStagedSyncError("verify header failed")
	ErrInsertChainFail               = WrapStagedSyncError("insert to chain failed")
	ErrZeroBlockResponse             = WrapStagedSyncError("zero block number response from remote nodes")
	ErrEmptyWhitelist                = WrapStagedSyncError("empty white list")
	ErrWrongGetBlockNumberType       = WrapStagedSyncError("wrong type of getBlockNumber interface")
	ErrSaveBlocksToDbFailed          = WrapStagedSyncError("saving downloaded blocks to db failed")
)

// WrapStagedSyncError wraps errors for staged sync and returns error object
func WrapStagedSyncError(context string) error {
	return fmt.Errorf("[STAGED_STREAM_SYNC]: %s", context)
}

// WrapStagedSyncMsg wraps message for staged sync and returns string
func WrapStagedSyncMsg(context string) string {
	return fmt.Sprintf("[STAGED_STREAM_SYNC]: %s", context)
}
