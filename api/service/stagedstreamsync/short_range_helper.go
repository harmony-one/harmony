package stagedstreamsync

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	syncProto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type srHelper struct {
	syncProtocol syncProtocol
	config       Config
	logger       zerolog.Logger
}

func (sh *srHelper) getHashChain(ctx context.Context, bns []uint64) ([]common.Hash, []sttypes.StreamID, error) {
	results := newBlockHashResults(bns)

	var wg sync.WaitGroup
	wg.Add(sh.config.Concurrency)

	for i := 0; i != sh.config.Concurrency; i++ {
		go func(index int) {
			defer wg.Done()

			hashes, stid, err := sh.doGetBlockHashesRequest(ctx, bns)
			if err != nil {
				sh.logger.Warn().Err(err).Str("StreamID", string(stid)).
					Msg(WrapStagedSyncMsg("doGetBlockHashes return error"))
				return
			}
			results.addResult(hashes, stid)
		}(i)
	}
	wg.Wait()

	select {
	case <-ctx.Done():
		sh.logger.Info().Err(ctx.Err()).Int("num blocks", results.numBlocksWithResults()).
			Msg(WrapStagedSyncMsg("short range sync get hashes timed out"))
		return nil, nil, ctx.Err()
	default:
	}

	hashChain, wl := results.computeLongestHashChain()
	sh.logger.Info().Int("hashChain size", len(hashChain)).Int("whitelist", len(wl)).
		Msg(WrapStagedSyncMsg("computeLongestHashChain result"))
	return hashChain, wl, nil
}

func (sh *srHelper) getBlocksChain(ctx context.Context, bns []uint64) ([]*types.Block, sttypes.StreamID, error) {
	return sh.doGetBlocksByNumbersRequest(ctx, bns)
}

func (sh *srHelper) getBlocksByHashes(ctx context.Context, hashes []common.Hash, whitelist []sttypes.StreamID) ([]*types.Block, []sttypes.StreamID, error) {

	m := newGetBlocksByHashManager(hashes, whitelist)

	var (
		wg      sync.WaitGroup
		gErr    error
		errLock sync.Mutex
	)

	concurrency := sh.config.Concurrency
	if concurrency > m.numRequests() {
		concurrency = m.numRequests()
	}

	wg.Add(concurrency)
	for i := 0; i != concurrency; i++ {
		go func(index int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			for {
				if m.isDone() {
					return
				}
				hashes, wl, err := m.getNextHashes()
				if err != nil {
					errLock.Lock()
					gErr = err
					errLock.Unlock()
					return
				}
				if len(hashes) == 0 {
					select {
					case <-time.After(200 * time.Millisecond):
						continue
					case <-ctx.Done():
						return
					}
				}
				blocks, stid, err := sh.doGetBlocksByHashesRequest(ctx, hashes, wl)
				if err != nil {
					sh.logger.Warn().Err(err).
						Str("StreamID", string(stid)).
						Int("hashes", len(hashes)).
						Int("index", index).
						Msg(WrapStagedSyncMsg("getBlocksByHashes worker failed"))
					m.handleResultError(hashes, stid)
				} else {
					m.addResult(hashes, blocks, stid)
				}
			}
		}(i)
	}
	wg.Wait()

	if gErr != nil {
		return nil, nil, gErr
	}
	select {
	case <-ctx.Done():
		res, _, _ := m.getResults()
		sh.logger.Info().Err(ctx.Err()).Int("num blocks", len(res)).
			Msg(WrapStagedSyncMsg("short range sync get blocks timed out"))
		return nil, nil, ctx.Err()
	default:
	}

	return m.getResults()
}

func (sh *srHelper) checkPrerequisites() error {
	if sh.syncProtocol.NumStreams() < sh.config.Concurrency {
		return ErrNotEnoughStreams
	}
	return nil
}

func (sh *srHelper) prepareBlockHashNumbers(curNumber uint64) []uint64 {

	res := make([]uint64, 0, BlockHashesPerRequest)

	for bn := curNumber + 1; bn <= curNumber+uint64(BlockHashesPerRequest); bn++ {
		res = append(res, bn)
	}
	return res
}

func (sh *srHelper) doGetBlockHashesRequest(ctx context.Context, bns []uint64) ([]common.Hash, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	hashes, stid, err := sh.syncProtocol.GetBlockHashes(ctx, bns)
	if err != nil {
		sh.logger.Warn().Err(err).
			Interface("block numbers", bns).
			Str("stream", string(stid)).
			Msg(WrapStagedSyncMsg("failed to doGetBlockHashesRequest"))
		return nil, stid, err
	}
	if len(hashes) != len(bns) {
		sh.logger.Warn().Err(ErrUnexpectedBlockHashes).
			Str("stream", string(stid)).
			Msg(WrapStagedSyncMsg("failed to doGetBlockHashesRequest"))
		sh.syncProtocol.StreamFailed(stid, "unexpected get block hashes result delivered")
		return nil, stid, ErrUnexpectedBlockHashes
	}
	return hashes, stid, nil
}

func (sh *srHelper) doGetBlocksByNumbersRequest(ctx context.Context, bns []uint64) ([]*types.Block, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocks, stid, err := sh.syncProtocol.GetBlocksByNumber(ctx, bns)
	if err != nil {
		sh.logger.Warn().Err(err).
			Str("stream", string(stid)).
			Msg(WrapStagedSyncMsg("failed to doGetBlockHashesRequest"))
		return nil, stid, err
	}
	return blocks, stid, nil
}

func (sh *srHelper) doGetBlocksByHashesRequest(ctx context.Context, hashes []common.Hash, wl []sttypes.StreamID) ([]*types.Block, sttypes.StreamID, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	blocks, stid, err := sh.syncProtocol.GetBlocksByHashes(ctx, hashes,
		syncProto.WithWhitelist(wl))
	if err != nil {
		sh.logger.Warn().Err(err).Str("stream", string(stid)).Msg("failed to getBlockByHashes")
		return nil, stid, err
	}
	if err := checkGetBlockByHashesResult(blocks, hashes); err != nil {
		sh.logger.Warn().Err(err).Str("stream", string(stid)).Msg(WrapStagedSyncMsg("failed to getBlockByHashes"))
		sh.syncProtocol.StreamFailed(stid, "failed to getBlockByHashes")
		return nil, stid, err
	}
	return blocks, stid, nil
}

func (sh *srHelper) removeStreams(sts []sttypes.StreamID) {
	for _, st := range sts {
		sh.syncProtocol.RemoveStream(st)
	}
}

func (sh *srHelper) streamsFailed(sts []sttypes.StreamID, reason string) {
	for _, st := range sts {
		sh.syncProtocol.StreamFailed(st, reason)
	}
}

// blameAllStreams only not to blame all whitelisted streams when the it's not the last block signature verification failed.
func (sh *srHelper) blameAllStreams(blocks types.Blocks, errIndex int, err error) bool {
	if errors.As(err, &emptySigVerifyErr) && errIndex == len(blocks)-1 {
		return false
	}
	return true
}
