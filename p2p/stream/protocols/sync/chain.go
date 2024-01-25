package sync

import (
	Bytes "bytes"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/keylocker"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
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
	getAccountRange(root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.AccountData, [][]byte, error)
	getStorageRanges(root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.StoragesData, [][]byte, error)
	getByteCodes(hs []common.Hash, bytes uint64) ([][]byte, error)
	getTrieNodes(root common.Hash, paths []*message.TrieNodePathSet, bytes uint64, start time.Time) ([][]byte, error)
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
		if err != nil {
			return nil, err
		}
		if len(entry) > 0 {
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

// getAccountRange
func (ch *chainHelperImpl) getAccountRange(root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.AccountData, [][]byte, error) {
	if bytes > softResponseLimit {
		bytes = softResponseLimit
	}
	// Retrieve the requested state and bail out if non existent
	tr, err := trie.New(trie.StateTrieID(root), ch.chain.TrieDB())
	if err != nil {
		return nil, nil, err
	}
	snapshots := ch.chain.Snapshots()
	if snapshots == nil {
		return nil, nil, errors.Errorf("failed to retrieve snapshots")
	}
	it, err := snapshots.AccountIterator(root, origin)
	if err != nil {
		return nil, nil, err
	}
	// Iterate over the requested range and pile accounts up
	var (
		accounts []*message.AccountData
		size     uint64
		last     common.Hash
	)
	for it.Next() {
		hash, account := it.Hash(), common.CopyBytes(it.Account())

		// Track the returned interval for the Merkle proofs
		last = hash

		// Assemble the reply item
		size += uint64(common.HashLength + len(account))
		accounts = append(accounts, &message.AccountData{
			Hash: hash[:],
			Body: account,
		})
		// If we've exceeded the request threshold, abort
		if Bytes.Compare(hash[:], limit[:]) >= 0 {
			break
		}
		if size > bytes {
			break
		}
	}
	it.Release()

	// Generate the Merkle proofs for the first and last account
	proof := light.NewNodeSet()
	if err := tr.Prove(origin[:], 0, proof); err != nil {
		utils.Logger().Warn().Err(err).Interface("origin", origin).Msg("Failed to prove account range")
		return nil, nil, err
	}
	if last != (common.Hash{}) {
		if err := tr.Prove(last[:], 0, proof); err != nil {
			utils.Logger().Warn().Err(err).Interface("last", last).Msg("Failed to prove account range")
			return nil, nil, err
		}
	}
	var proofs [][]byte
	for _, blob := range proof.NodeList() {
		proofs = append(proofs, blob)
	}
	return accounts, proofs, nil
}

// getStorageRangesRequest
func (ch *chainHelperImpl) getStorageRanges(root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.StoragesData, [][]byte, error) {
	if bytes > softResponseLimit {
		bytes = softResponseLimit
	}

	// Calculate the hard limit at which to abort, even if mid storage trie
	hardLimit := uint64(float64(bytes) * (1 + stateLookupSlack))

	// Retrieve storage ranges until the packet limit is reached
	var (
		slots  []*message.StoragesData
		proofs [][]byte
		size   uint64
	)
	snapshots := ch.chain.Snapshots()
	if snapshots == nil {
		return nil, nil, errors.Errorf("failed to retrieve snapshots")
	}
	for _, account := range accounts {
		// If we've exceeded the requested data limit, abort without opening
		// a new storage range (that we'd need to prove due to exceeded size)
		if size >= bytes {
			break
		}
		// The first account might start from a different origin and end sooner
		// origin==nil or limit ==nil
		// Retrieve the requested state and bail out if non existent
		it, err := snapshots.StorageIterator(root, account, origin)
		if err != nil {
			return nil, nil, err
		}
		// Iterate over the requested range and pile slots up
		var (
			storage []*message.StorageData
			last    common.Hash
			abort   bool
		)
		for it.Next() {
			if size >= hardLimit {
				abort = true
				break
			}
			hash, slot := it.Hash(), common.CopyBytes(it.Slot())

			// Track the returned interval for the Merkle proofs
			last = hash

			// Assemble the reply item
			size += uint64(common.HashLength + len(slot))
			storage = append(storage, &message.StorageData{
				Hash: hash[:],
				Body: slot,
			})
			// If we've exceeded the request threshold, abort
			if Bytes.Compare(hash[:], limit[:]) >= 0 {
				break
			}
		}

		if len(storage) > 0 {
			storages := &message.StoragesData{
				Data: storage,
			}
			slots = append(slots, storages)
		}
		it.Release()

		// Generate the Merkle proofs for the first and last storage slot, but
		// only if the response was capped. If the entire storage trie included
		// in the response, no need for any proofs.
		if origin != (common.Hash{}) || (abort && len(storage) > 0) {
			// Request started at a non-zero hash or was capped prematurely, add
			// the endpoint Merkle proofs
			accTrie, err := trie.NewStateTrie(trie.StateTrieID(root), ch.chain.TrieDB())
			if err != nil {
				return nil, nil, err
			}
			acc, err := accTrie.TryGetAccountByHash(account)
			if err != nil || acc == nil {
				return nil, nil, err
			}
			id := trie.StorageTrieID(root, account, acc.Root)
			stTrie, err := trie.NewStateTrie(id, ch.chain.TrieDB())
			if err != nil {
				return nil, nil, err
			}
			proof := light.NewNodeSet()
			if err := stTrie.Prove(origin[:], 0, proof); err != nil {
				utils.Logger().Warn().Interface("origin", origin).Msg("Failed to prove storage range")
				return nil, nil, err
			}
			if last != (common.Hash{}) {
				if err := stTrie.Prove(last[:], 0, proof); err != nil {
					utils.Logger().Warn().Interface("last", last).Msg("Failed to prove storage range")
					return nil, nil, err
				}
			}
			for _, blob := range proof.NodeList() {
				proofs = append(proofs, blob)
			}
			// Proof terminates the reply as proofs are only added if a node
			// refuses to serve more data (exception when a contract fetch is
			// finishing, but that's that).
			break
		}
	}
	return slots, proofs, nil
}

// getByteCodesRequest
func (ch *chainHelperImpl) getByteCodes(hashes []common.Hash, bytes uint64) ([][]byte, error) {
	if bytes > softResponseLimit {
		bytes = softResponseLimit
	}
	if len(hashes) > maxCodeLookups {
		hashes = hashes[:maxCodeLookups]
	}
	// Retrieve bytecodes until the packet size limit is reached
	var (
		codes      [][]byte
		totalBytes uint64
	)
	for _, hash := range hashes {
		if hash == state.EmptyCodeHash {
			// Peers should not request the empty code, but if they do, at
			// least sent them back a correct response without db lookups
			codes = append(codes, []byte{})
		} else if blob, err := ch.chain.ContractCode(hash); err == nil { // Double Check: ContractCodeWithPrefix
			codes = append(codes, blob)
			totalBytes += uint64(len(blob))
		}
		if totalBytes > bytes {
			break
		}
	}
	return codes, nil
}

// getTrieNodesRequest
func (ch *chainHelperImpl) getTrieNodes(root common.Hash, paths []*message.TrieNodePathSet, bytes uint64, start time.Time) ([][]byte, error) {
	if bytes > softResponseLimit {
		bytes = softResponseLimit
	}
	// Make sure we have the state associated with the request
	triedb := ch.chain.TrieDB()

	accTrie, err := trie.NewStateTrie(trie.StateTrieID(root), triedb)
	if err != nil {
		// We don't have the requested state available, bail out
		return nil, nil
	}
	// The 'snap' might be nil, in which case we cannot serve storage slots.
	snapshots := ch.chain.Snapshots()
	if snapshots == nil {
		return nil, errors.Errorf("failed to retrieve snapshots")
	}
	snap := snapshots.Snapshot(root)
	// Retrieve trie nodes until the packet size limit is reached
	var (
		nodes      [][]byte
		TotalBytes uint64
		loads      int // Trie hash expansions to count database reads
	)
	for _, p := range paths {
		switch len(p.Pathset) {
		case 0:
			// Ensure we penalize invalid requests
			return nil, fmt.Errorf("zero-item pathset requested")

		case 1:
			// If we're only retrieving an account trie node, fetch it directly
			blob, resolved, err := accTrie.TryGetNode(p.Pathset[0])
			loads += resolved // always account database reads, even for failures
			if err != nil {
				break
			}
			nodes = append(nodes, blob)
			TotalBytes += uint64(len(blob))

		default:
			var stRoot common.Hash
			// Storage slots requested, open the storage trie and retrieve from there
			if snap == nil {
				// We don't have the requested state snapshotted yet (or it is stale),
				// but can look up the account via the trie instead.
				account, err := accTrie.TryGetAccountByHash(common.BytesToHash(p.Pathset[0]))
				loads += 8 // We don't know the exact cost of lookup, this is an estimate
				if err != nil || account == nil {
					break
				}
				stRoot = account.Root
			} else {
				account, err := snap.Account(common.BytesToHash(p.Pathset[0]))
				loads++ // always account database reads, even for failures
				if err != nil || account == nil {
					break
				}
				stRoot = common.BytesToHash(account.Root)
			}
			id := trie.StorageTrieID(root, common.BytesToHash(p.Pathset[0]), stRoot)
			stTrie, err := trie.NewStateTrie(id, triedb)
			loads++ // always account database reads, even for failures
			if err != nil {
				break
			}
			for _, path := range p.Pathset[1:] {
				blob, resolved, err := stTrie.TryGetNode(path)
				loads += resolved // always account database reads, even for failures
				if err != nil {
					break
				}
				nodes = append(nodes, blob)
				TotalBytes += uint64(len(blob))

				// Sanity check limits to avoid DoS on the store trie loads
				if TotalBytes > bytes || loads > maxTrieNodeLookups || time.Since(start) > maxTrieNodeTimeSpent {
					break
				}
			}
		}
		// Abort request processing if we've exceeded our limits
		if TotalBytes > bytes || loads > maxTrieNodeLookups || time.Since(start) > maxTrieNodeTimeSpent {
			break
		}
	}
	return nodes, nil
}
