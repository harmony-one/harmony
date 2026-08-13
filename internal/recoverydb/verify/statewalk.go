package verify

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// Code locations, in resolution order (plan §2.2.9): prefixed contract code,
// prefixed validator code, legacy un-prefixed (physically in the bare-hash32
// keyspace, content-verified so the ambiguity is harmless).
const (
	CodeLocPrefixed  = "c"
	CodeLocValidator = "vc"
	CodeLocLegacy    = "bare"
)

// ErrCodeNotFound reports a code hash unresolvable at any of the three
// locations.
var ErrCodeNotFound = errors.New("verify: code not found at any location (c, vc, legacy unprefixed)")

// hasThenGet distinguishes "absent" from "read error" without relying on
// backend-specific not-found error identities (fail-closed: any Has/Get
// error is surfaced).
func hasThenGet(db ethdb.KeyValueReader, key []byte) (val []byte, found bool, err error) {
	ok, err := db.Has(key)
	if err != nil {
		return nil, false, fmt.Errorf("verify: has %x: %w", key, err)
	}
	if !ok {
		return nil, false, nil
	}
	v, err := db.Get(key)
	if err != nil {
		return nil, false, fmt.Errorf("verify: get %x: %w", key, err)
	}
	return v, true, nil
}

// ResolveCode fetches code by hash trying all three locations in order,
// verifying keccak256(code) == codeHash before returning. Unlike the stock
// state.Database path (core/state/iterator.go:123-131) an absent entry at
// one location does not abort the fallback; a positive read error is still
// surfaced.
func ResolveCode(db ethdb.KeyValueReader, codeHash common.Hash) ([]byte, string, error) {
	type loc struct {
		key []byte
		tag string
	}
	locs := []loc{
		{keys.CodeKey(codeHash), CodeLocPrefixed},
		{keys.ValidatorCodeKey(codeHash), CodeLocValidator},
		{codeHash.Bytes(), CodeLocLegacy},
	}
	for _, l := range locs {
		val, found, err := hasThenGet(db, l.key)
		if err != nil {
			return nil, "", err
		}
		if !found || len(val) == 0 {
			continue
		}
		if crypto.Keccak256Hash(val) != codeHash {
			return nil, "", fmt.Errorf("verify: code at location %q for hash %s fails content verification", l.tag, codeHash.Hex())
		}
		return val, l.tag, nil
	}
	return nil, "", fmt.Errorf("%w: %s", ErrCodeNotFound, codeHash.Hex())
}

// StateWalkResult carries the state half of the DigestSet plus coverage
// counters.
type StateWalkResult struct {
	Accounts     report.Digest
	StorageSlots report.Digest
	Codes        report.Digest

	AccountCount            uint64
	ContractCount           uint64
	StorageSlotCount        uint64
	UniqueCodeCount         uint64
	MissingAccountPreimages uint64
	MissingStoragePreimages uint64
	CodeLocationCounts      map[string]uint64
}

// StateWalkOptions tunes the traversal.
type StateWalkOptions struct {
	// RequirePreimages makes any missing account/storage preimage fatal,
	// naming the hashed key (mandatory for full-archival sources, plan WS2).
	RequirePreimages bool
	// CheckPreimages enables preimage coverage accounting at all (inspect
	// passes true; verify-db of the compact artifact passes false — the
	// artifact carries no preimages by default).
	CheckPreimages bool
	// OnAccount, when set, is invoked for every account leaf.
	OnAccount func(addrHash common.Hash, acc *state.Account) error
	// OnCode, when set, is invoked once per unique code hash.
	OnCode func(codeHash common.Hash, location string, code []byte) error
	// OnNode, when set, is invoked for every HASHED trie node visited
	// (account and storage tries) — the reachable-set feed for the
	// bare-hash32 orphan check (plan §11.1). Embedded (<32-byte) nodes have
	// no hash and are not stored separately, so they are not reported.
	OnNode func(nodeHash common.Hash) error
}

// WalkState fully traverses the account trie at root with per-account
// storage traversal and code loads. It is purpose-built to avoid the two
// stock state-iterator defects (plan §2.1, round 6 finding 6, in-place §C3):
//
//  1. a failed FIRST storage-iterator step is distinguished from empty
//     storage by checking the iterator's Error() (stock discards it at
//     core/state/iterator.go:117-119);
//  2. code resolution falls back to the validator-code and legacy locations
//     when the prefixed contract-code location has no usable entry (stock
//     returns early on a lookup error at :123-131).
//
// Every iterator error is fatal (unlike core/state/dump.go:170-186).
// Digests are order-sensitive: tries iterate in lexical hashed-key order;
// codes are digested at the end in sorted code-hash order, location-tagged.
func WalkState(db ethdb.Database, root common.Hash, opts StateWalkOptions) (*StateWalkResult, error) {
	triedb := trie.NewDatabaseWithConfig(db, &trie.Config{Preimages: true})
	accTrie, err := trie.NewStateTrie(trie.StateTrieID(root), triedb)
	if err != nil {
		return nil, fmt.Errorf("verify: open account trie at %s: %w", root.Hex(), err)
	}

	res := &StateWalkResult{CodeLocationCounts: map[string]uint64{}}
	accH := report.NewHasher("state.accounts")
	storH := report.NewHasher("state.storage")

	seenCode := map[common.Hash]string{} // codeHash -> location (blobs re-fetched at digest time)

	nodeIt := accTrie.NodeIterator(nil)
	for nodeIt.Next(true) {
		if h := nodeIt.Hash(); h != (common.Hash{}) && opts.OnNode != nil {
			if err := opts.OnNode(h); err != nil {
				return nil, err
			}
		}
		if !nodeIt.Leaf() {
			continue
		}
		leafKey := append([]byte{}, nodeIt.LeafKey()...)
		leafBlob := append([]byte{}, nodeIt.LeafBlob()...)
		addrHash := common.BytesToHash(leafKey)
		var acc state.Account
		if err := rlp.DecodeBytes(leafBlob, &acc); err != nil {
			return nil, fmt.Errorf("verify: decode account %s: %w", addrHash.Hex(), err)
		}
		res.AccountCount++
		accH.Add(addrHash.Bytes(), leafBlob)

		if opts.CheckPreimages {
			if pre := accTrie.GetKey(leafKey); pre == nil {
				res.MissingAccountPreimages++
				if opts.RequirePreimages {
					return nil, fmt.Errorf("verify: missing account preimage for hashed key %s", addrHash.Hex())
				}
			}
		}
		if opts.OnAccount != nil {
			if err := opts.OnAccount(addrHash, &acc); err != nil {
				return nil, err
			}
		}

		// Storage traversal (defect 1 fixed: the iterator's Error() is
		// always consulted when iteration ends, including a failed first
		// step; a truly empty trie yields zero steps and nil error, and a
		// missing root node fails NewStateTrie construction).
		if acc.Root != state.EmptyRootHash && acc.Root != (common.Hash{}) {
			stTrie, err := trie.NewStateTrie(trie.StorageTrieID(root, addrHash, acc.Root), triedb)
			if err != nil {
				return nil, fmt.Errorf("verify: open storage trie of %s (root %s): %w", addrHash.Hex(), acc.Root.Hex(), err)
			}
			sIt := stTrie.NodeIterator(nil)
			slots := uint64(0)
			for sIt.Next(true) {
				if h := sIt.Hash(); h != (common.Hash{}) && opts.OnNode != nil {
					if err := opts.OnNode(h); err != nil {
						return nil, err
					}
				}
				if !sIt.Leaf() {
					continue
				}
				slotKey := append([]byte{}, sIt.LeafKey()...)
				slotHash := common.BytesToHash(slotKey)
				storH.Add(addrHash.Bytes(), slotHash.Bytes(), sIt.LeafBlob())
				res.StorageSlotCount++
				slots++
				if opts.CheckPreimages {
					if pre := stTrie.GetKey(slotKey); pre == nil {
						res.MissingStoragePreimages++
						if opts.RequirePreimages {
							return nil, fmt.Errorf("verify: missing storage-slot preimage for hashed key %s (account %s)", slotHash.Hex(), addrHash.Hex())
						}
					}
				}
			}
			if err := sIt.Error(); err != nil {
				return nil, fmt.Errorf("verify: storage traversal of %s (root %s) failed: %w", addrHash.Hex(), acc.Root.Hex(), err)
			}
			if slots == 0 {
				return nil, fmt.Errorf("verify: storage trie of %s (non-empty root %s) yielded no leaves and no error; corrupt trie", addrHash.Hex(), acc.Root.Hex())
			}
		}

		// Code load (defect 2 fixed inside ResolveCode).
		if len(acc.CodeHash) == 32 && !bytes.Equal(acc.CodeHash, state.EmptyCodeHash.Bytes()) {
			codeHash := common.BytesToHash(acc.CodeHash)
			res.ContractCount++
			if _, ok := seenCode[codeHash]; !ok {
				code, locTag, err := ResolveCode(db, codeHash)
				if err != nil {
					return nil, fmt.Errorf("verify: account %s: %w", addrHash.Hex(), err)
				}
				seenCode[codeHash] = locTag
				res.CodeLocationCounts[locTag]++
				if opts.OnCode != nil {
					if err := opts.OnCode(codeHash, locTag, code); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if err := nodeIt.Error(); err != nil {
		return nil, fmt.Errorf("verify: account trie traversal at %s failed: %w", root.Hex(), err)
	}

	// Codes digested in sorted hash order, location-tagged. Blobs are
	// re-fetched (and re-content-verified) rather than retained, to bound
	// memory on large states.
	hashes := make([]common.Hash, 0, len(seenCode))
	for h := range seenCode {
		hashes = append(hashes, h)
	}
	sortHashes(hashes)
	codeH := report.NewHasher("state.codes")
	for _, h := range hashes {
		code, locTag, err := ResolveCode(db, h)
		if err != nil {
			return nil, fmt.Errorf("verify: code digest pass: %w", err)
		}
		if locTag != seenCode[h] {
			return nil, fmt.Errorf("verify: code %s changed location during walk (%s -> %s)", h.Hex(), seenCode[h], locTag)
		}
		codeH.Add(h.Bytes(), []byte(locTag), code)
	}
	res.UniqueCodeCount = uint64(len(hashes))
	res.Accounts = accH.Digest()
	res.StorageSlots = storH.Digest()
	res.Codes = codeH.Digest()
	return res, nil
}

// WalkStateProbe checks that the account trie at root opens (its root node
// resolves) without running the full traversal.
func WalkStateProbe(db ethdb.Database, root common.Hash) (bool, error) {
	triedb := trie.NewDatabaseWithConfig(db, &trie.Config{Preimages: false})
	if _, err := trie.NewStateTrie(trie.StateTrieID(root), triedb); err != nil {
		return false, fmt.Errorf("verify: state at %s does not open: %w", root.Hex(), err)
	}
	return true, nil
}

func sortHashes(hs []common.Hash) {
	for i := 1; i < len(hs); i++ {
		for j := i; j > 0 && bytes.Compare(hs[j-1][:], hs[j][:]) > 0; j-- {
			hs[j-1], hs[j] = hs[j], hs[j-1]
		}
	}
}
