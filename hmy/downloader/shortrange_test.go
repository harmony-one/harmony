package downloader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/rs/zerolog"
)

func TestDownloader_doShortRangeSync(t *testing.T) {
	chain := newTestBlockChain(100, nil)

	d := &Downloader{
		bc:           chain,
		syncProtocol: newTestSyncProtocol(105, 32, nil),
		config: Config{
			Concurrency: 16,
			MinStreams:  16,
		},
		ctx:    context.Background(),
		logger: zerolog.Logger{},
	}
	n, err := d.doShortRangeSync()
	if err != nil {
		t.Error(err)
	}
	if n == 0 {
		t.Error("not synced")
	}
	if curNum := d.bc.CurrentBlock().NumberU64(); curNum != 105 {
		t.Errorf("unexpected block number after sync: %v / %v", curNum, 105)
	}
}

func TestSrHelper_getHashChain(t *testing.T) {
	tests := []struct {
		curBN        uint64
		syncProtocol syncProtocol
		config       Config

		expHashChainSize int
		expStSize        int
	}{
		{
			curBN:        100,
			syncProtocol: newTestSyncProtocol(1000, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: numBlockHashesPerRequest,
			expStSize:        16, // Concurrency
		},
		{
			curBN:        100,
			syncProtocol: newTestSyncProtocol(100, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: 0,
			expStSize:        0,
		},
		{
			curBN:        100,
			syncProtocol: newTestSyncProtocol(110, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: 10,
			expStSize:        16,
		},
		{
			// stream size is smaller than concurrency
			curBN:        100,
			syncProtocol: newTestSyncProtocol(1000, 10, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  8,
			},
			expHashChainSize: numBlockHashesPerRequest,
			expStSize:        10,
		},
		{
			// one stream reports an error, else are fine
			curBN:        100,
			syncProtocol: newTestSyncProtocol(1000, 32, makeOnceErrorFunc()),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: numBlockHashesPerRequest,
			expStSize:        15, // Concurrency
		},
		{
			// error happens at one block number, all stream removed
			curBN: 100,
			syncProtocol: newTestSyncProtocol(1000, 32, func(bn uint64) error {
				if bn == 110 {
					return errors.New("test error")
				}
				return nil
			}),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: 0,
			expStSize:        0,
		},
		{
			curBN:        100,
			syncProtocol: newTestSyncProtocol(1000, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expHashChainSize: numBlockHashesPerRequest,
			expStSize:        16, // Concurrency
		},
	}

	for i, test := range tests {
		sh := &srHelper{
			syncProtocol: test.syncProtocol,
			ctx:          context.Background(),
			config:       test.config,
		}
		hashChain, wl, err := sh.getHashChain(sh.prepareBlockHashNumbers(test.curBN))
		if err != nil {
			t.Error(err)
		}
		if len(hashChain) != test.expHashChainSize {
			t.Errorf("Test %v: hash chain size unexpected: %v / %v", i, len(hashChain), test.expHashChainSize)
		}
		if len(wl) != test.expStSize {
			t.Errorf("Test %v: whitelist size unexpected: %v / %v", i, len(wl), test.expStSize)
		}
	}
}

func TestSrHelper_GetBlocksByHashes(t *testing.T) {
	tests := []struct {
		hashes       []common.Hash
		syncProtocol syncProtocol
		config       Config

		expBlockNumbers []uint64
		expErr          error
	}{
		{
			hashes:       testNumberToHashes([]uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110}),
			syncProtocol: newTestSyncProtocol(1000, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expBlockNumbers: []uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110},
			expErr:          nil,
		},
		{
			// remote node cannot give the block with the given hash
			hashes:       testNumberToHashes([]uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110}),
			syncProtocol: newTestSyncProtocol(100, 32, nil),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expBlockNumbers: []uint64{},
			expErr:          errors.New("all streams are bad"),
		},
		{
			// one request return an error, else are fine
			hashes:       testNumberToHashes([]uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110}),
			syncProtocol: newTestSyncProtocol(1000, 32, makeOnceErrorFunc()),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expBlockNumbers: []uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110},
			expErr:          nil,
		},
		{
			// All nodes encounter an error
			hashes: testNumberToHashes([]uint64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110}),
			syncProtocol: newTestSyncProtocol(1000, 32, func(n uint64) error {
				if n == 109 {
					return errors.New("test error")
				}
				return nil
			}),
			config: Config{
				Concurrency: 16,
				MinStreams:  16,
			},
			expErr: errors.New("error expected"),
		},
	}
	for i, test := range tests {
		sh := &srHelper{
			syncProtocol: test.syncProtocol,
			ctx:          context.Background(),
			config:       test.config,
		}
		blocks, _, err := sh.getBlocksByHashes(test.hashes, makeStreamIDs(5))
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: unexpected error %v / %v", i, err, test.expErr)
		}
		if len(blocks) != len(test.expBlockNumbers) {
			t.Errorf("Test %v: unepxected block number size: %v / %v", i, len(blocks), len(test.expBlockNumbers))
		}
		for i, block := range blocks {
			gotNum := testHashToNumber(block.Hash())
			if gotNum != test.expBlockNumbers[i] {
				t.Errorf("Test %v: unexpected block number", i)
			}
		}
	}
}

func TestBlockHashResult_ComputeLongestHashChain(t *testing.T) {
	tests := []struct {
		bns          []uint64
		results      map[sttypes.StreamID][]int64
		expChain     []uint64
		expWhitelist map[sttypes.StreamID]struct{}
		expErr       error
	}{
		{
			bns: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			results: map[sttypes.StreamID][]int64{
				makeStreamID(0): {1, 2, 3, 4, 5, 6, 7},
				makeStreamID(1): {1, 2, 3, 4, 5, 6, 7},
				makeStreamID(2): {1, 2, 3, 4, 5}, // left behind
			},
			expChain: []uint64{1, 2, 3, 4, 5, 6, 7},
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(0): {},
				makeStreamID(1): {},
			},
		},
		{
			// minority fork
			bns: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			results: map[sttypes.StreamID][]int64{
				makeStreamID(0): {1, 2, 3, 4, 5, 6, 7},
				makeStreamID(1): {1, 2, 3, 4, 5, 6, 7},
				makeStreamID(2): {1, 2, 3, 4, 5, 7, 8, 9},
			},
			expChain: []uint64{1, 2, 3, 4, 5, 6, 7},
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(0): {},
				makeStreamID(1): {},
			},
		}, {
			// nil block
			bns: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			results: map[sttypes.StreamID][]int64{
				makeStreamID(0): {},
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
			expChain:     nil,
			expWhitelist: nil,
		}, {
			// not continuous block
			bns: []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			results: map[sttypes.StreamID][]int64{
				makeStreamID(0): {1, 2, 3, 4, 5, 6, 7, -1, 9},
				makeStreamID(1): {1, 2, 3, 4, 5, 6, 7},
				makeStreamID(2): {1, 2, 3, 4, 5, 7, 8, 9},
			},
			expChain: []uint64{1, 2, 3, 4, 5, 6, 7},
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(0): {},
				makeStreamID(1): {},
			},
		},
		{
			// not continuous block
			bns:     []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			results: map[sttypes.StreamID][]int64{},
			expErr:  errors.New("zero result"),
		},
	}

	for i, test := range tests {
		res := newBlockHashResults(test.bns)
		for st, hs := range test.results {
			res.addResult(makeTestBlockHashes(hs), st)
		}

		chain, wl := res.computeLongestHashChain()

		if err := checkHashChainResult(chain, test.expChain); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
		if err := checkStreamSetEqual(streamIDListToMap(wl), test.expWhitelist); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func checkHashChainResult(gots []common.Hash, exps []uint64) error {
	if len(gots) != len(exps) {
		return errors.New("unexpected size")
	}
	for i, got := range gots {
		exp := exps[i]
		if got != makeTestBlockHash(exp) {
			return errors.New("unexpected block hash")
		}
	}
	return nil
}

func TestHashMaxVote(t *testing.T) {
	tests := []struct {
		m            map[sttypes.StreamID]common.Hash
		whitelist    map[sttypes.StreamID]struct{}
		expRes       common.Hash
		expWhitelist map[sttypes.StreamID]struct{}
	}{
		{
			m: map[sttypes.StreamID]common.Hash{
				makeStreamID(0): makeTestBlockHash(0),
				makeStreamID(1): makeTestBlockHash(1),
				makeStreamID(2): makeTestBlockHash(1),
			},
			whitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(0): {},
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
			expRes: makeTestBlockHash(1),
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
		}, {
			m: map[sttypes.StreamID]common.Hash{
				makeStreamID(0): makeTestBlockHash(0),
				makeStreamID(1): makeTestBlockHash(1),
				makeStreamID(2): makeTestBlockHash(1),
			},
			whitelist: nil,
			expRes:    makeTestBlockHash(1),
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
		}, {
			m: map[sttypes.StreamID]common.Hash{
				makeStreamID(0): makeTestBlockHash(0),
				makeStreamID(1): makeTestBlockHash(1),
				makeStreamID(2): makeTestBlockHash(1),
				makeStreamID(3): makeTestBlockHash(0),
				makeStreamID(4): makeTestBlockHash(0),
			},
			whitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(0): {},
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
			expRes: makeTestBlockHash(1),
			expWhitelist: map[sttypes.StreamID]struct{}{
				makeStreamID(1): {},
				makeStreamID(2): {},
			},
		},
	}

	for i, test := range tests {
		h, wl := countHashMaxVote(test.m, test.whitelist)

		if h != test.expRes {
			t.Errorf("Test %v: unexpected hash: %x / %x", i, h, test.expRes)
		}
		if err := checkStreamSetEqual(wl, test.expWhitelist); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func checkStreamSetEqual(m1, m2 map[sttypes.StreamID]struct{}) error {
	if len(m1) != len(m2) {
		return fmt.Errorf("unexpected size: %v / %v", len(m1), len(m2))
	}
	for st := range m1 {
		if _, ok := m2[st]; !ok {
			return errors.New("not equal")
		}
	}
	return nil
}

func makeTestBlockHashes(bns []int64) []common.Hash {
	hs := make([]common.Hash, 0, len(bns))
	for _, bn := range bns {
		if bn < 0 {
			hs = append(hs, emptyHash)
		} else {
			hs = append(hs, makeTestBlockHash(uint64(bn)))
		}
	}
	return hs
}

func streamIDListToMap(sts []sttypes.StreamID) map[sttypes.StreamID]struct{} {
	res := make(map[sttypes.StreamID]struct{})

	for _, st := range sts {
		res[st] = struct{}{}
	}
	return res
}

func makeTestBlockHash(bn uint64) common.Hash {
	return makeTestBlock(bn).Hash()
}

func makeOnceErrorFunc() func(num uint64) error {
	var once sync.Once
	return func(num uint64) error {
		var err error
		once.Do(func() {
			err = errors.New("test error expected")
		})
		return err
	}
}
