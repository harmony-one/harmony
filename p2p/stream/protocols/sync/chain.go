package sync

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils/keylocker"
	"github.com/pkg/errors"
)

// chainHelper is the adapter for blockchain which is friendly to unit test.
type chainHelper interface {
	getCurrentBlockNumber() uint64
	getBlockHashes(bns []uint64) []common.Hash
	getBlocksByNumber(bns []uint64) ([]*types.Block, error)
	getBlocksByHashes(hs []common.Hash) ([]*types.Block, error)
	getNodeData(hs []common.Hash) ([][]byte, error)
	getReceipts(hs []common.Hash) ([]types.Receipts, error)
}

type chainHelperImpl struct {
	chain     engine.ChainReader
	schedule  shardingconfig.Schedule
	keyLocker *keylocker.KeyLocker
}

func newChainHelper(chain engine.ChainReader, schedule shardingconfig.Schedule) *chainHelperImpl {
	return &chainHelperImpl{
		chain:     chain,
		schedule:  schedule,
		keyLocker: keylocker.New(),
	}
}

func (ch *chainHelperImpl) getCurrentBlockNumber() uint64 {
	return ch.chain.CurrentBlock().NumberU64()
}

func (ch *chainHelperImpl) getBlockHashes(bns []uint64) []common.Hash {
	hashes := make([]common.Hash, 0, len(bns))
	for _, bn := range bns {
		var h common.Hash
		header := ch.chain.GetHeaderByNumber(bn)
		if header != nil {
			h = header.Hash()
		}
		hashes = append(hashes, h)
	}
	return hashes
}

func (ch *chainHelperImpl) getBlocksByNumber(bns []uint64) ([]*types.Block, error) {
	curBlock := ch.chain.CurrentHeader().Number().Uint64()
	blocks := make([]*types.Block, 0, len(bns))
	for _, bn := range bns {
		if curBlock < bn {
			continue
		}
		header := ch.chain.GetHeaderByNumber(bn)
		if header != nil {
			block, err := ch.getBlockWithSigByHeader(header)
			if err != nil {
				return nil, errors.Wrapf(err, "get block %v at %v", header.Hash().String(), header.Number())
			}
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (ch *chainHelperImpl) getBlocksByHashes(hs []common.Hash) ([]*types.Block, error) {
	var (
		blocks = make([]*types.Block, 0, len(hs))
	)
	for _, h := range hs {
		var (
			block *types.Block
			err   error
		)
		header := ch.chain.GetHeaderByHash(h)
		if header != nil {
			block, err = ch.getBlockWithSigByHeader(header)
			if err != nil {
				return nil, errors.Wrapf(err, "get block %v at %v", header.Hash().String(), header.Number())
			}
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (ch *chainHelperImpl) getBlockWithSigByHeader(header *block.Header) (*types.Block, error) {
	rs, err := ch.keyLocker.Lock(header.Number().Uint64(), func() (interface{}, error) {
		b := ch.chain.GetBlock(header.Hash(), header.Number().Uint64())
		if b == nil {
			return nil, errors.Errorf("block %d not found", header.Number().Uint64())
		}
		commitSig, err := ch.getBlockSigAndBitmap(header)
		if err != nil {
			return nil, errors.New("missing commit signature")
		}
		b.SetCurrentCommitSig(commitSig)
		return b, nil
	})

	if err != nil {
		return nil, err
	}

	return rs.(*types.Block), nil
}

func (ch *chainHelperImpl) getBlockSigAndBitmap(header *block.Header) ([]byte, error) {
	sb := ch.getBlockSigFromNextBlock(header)
	if len(sb) != 0 {
		return sb, nil
	}
	// Note: some commit sig read from db is different from [nextHeader.sig, nextHeader.bitMap]
	//       nextBlock data is better to be used.
	return ch.getBlockSigFromDB(header)
}

func (ch *chainHelperImpl) getBlockSigFromNextBlock(header *block.Header) []byte {
	nextBN := header.Number().Uint64() + 1
	nextHeader := ch.chain.GetHeaderByNumber(nextBN)
	if nextHeader == nil {
		return nil
	}

	sigBytes := nextHeader.LastCommitSignature()
	bitMap := nextHeader.LastCommitBitmap()
	sb := make([]byte, len(sigBytes)+len(bitMap))
	copy(sb[:], sigBytes[:])
	copy(sb[len(sigBytes):], bitMap[:])

	return sb
}

func (ch *chainHelperImpl) getBlockSigFromDB(header *block.Header) ([]byte, error) {
	return ch.chain.ReadCommitSig(header.Number().Uint64())
}

// getNodeData assembles the response to a node data query.
func (ch *chainHelperImpl) getNodeData(hs []common.Hash) ([][]byte, error) {
	var (
		bytes int
		nodes [][]byte
	)
	for _, hash := range hs {
		// Retrieve the requested state entry
		entry, err := ch.chain.TrieNode(hash)
		if len(entry) == 0 || err != nil {
			// Read the contract/validator code with prefix only to save unnecessary lookups.
			entry, err = ch.chain.ContractCode(hash)
			if len(entry) == 0 || err != nil {
				entry, err = ch.chain.ValidatorCode(hash)
			}
		}
		if err == nil && len(entry) > 0 {
			nodes = append(nodes, entry)
			bytes += len(entry)
		}
	}
	return nodes, nil
}

// getReceipts assembles the response to a receipt query.
func (ch *chainHelperImpl) getReceipts(hs []common.Hash) ([]types.Receipts, error) {
	receipts := make([]types.Receipts, len(hs))
	for i, hash := range hs {
		// Retrieve the requested block's receipts
		results := ch.chain.GetReceiptsByHash(hash)
		if results == nil {
			if header := ch.chain.GetHeaderByHash(hash); header == nil || header.ReceiptHash() != types.EmptyRootHash {
				continue
			}
			return nil, errors.New("invalid hashes to get receipts")
		}
		receipts[i] = results
	}
	return receipts, nil
}
