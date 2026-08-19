// Package statecheck walks the complete target state: every account, every
// storage trie, every code blob, with every standalone trie node
// keccak-authenticated against its hash.
//
// It is a self-contained walker independent of core/state/iterator.go, which
// has two confirmed defects this package must not inherit:
//
//  1. iterator.go:123-126 returns when ContractCode errors, making the
//     ValidatorCode fallback unreachable for missing code (validator
//     wrappers are stored under the "vc" namespace) - the stock iterator
//     hard-fails on every validator account. The walker probes the three
//     physical code namespaces (c, vc, legacy bare hash) with raw reads.
//  2. iterator.go:116-119 drops the error of the initial storage-iterator
//     step - a storage trie with a missing root is silently treated as
//     empty. The walker checks Error() after every Next, including the
//     first.
package statecheck

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
	"github.com/harmony-one/harmony/staking"
	staketypes "github.com/harmony-one/harmony/staking/types"
)

var (
	emptyRootHash = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
	emptyCodeHash = crypto.Keccak256Hash(nil)
)

// Config configures the walk.
type Config struct {
	KV          ethdb.KeyValueStore
	StateRoot   common.Hash
	TrieCacheMB int
	// Workers bounds the storage/code worker pool; results are folded in
	// account order so digests, counts and anomaly examples are
	// scheduling-independent. Default min(8, NumCPU).
	Workers  int
	Progress io.Writer
}

// Result is the outcome of a successful walk.
type Result struct {
	Counts    report.StateCounts
	Digest    [32]byte
	Anomalies *AnomalySet
}

// Walk runs the completeness walk. The returned error is a *report.Failure
// for integrity FAILs (missing node, authentication mismatch, decode
// failure, classification violation); other errors are read errors for the
// retry runner (transient I/O swallowed by the trie layer is rescued by the
// rodb latch).
func Walk(cfg Config) (*Result, error) {
	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 8 {
			workers = 8
		}
	}
	trieCfg := &trie.Config{Cache: cfg.TrieCacheMB}
	sdb := state.NewDatabaseWithConfig(rawdb.NewDatabase(cfg.KV), trieCfg)

	accountTrie, err := sdb.OpenTrie(cfg.StateRoot)
	if err != nil {
		return nil, trieFailure("account trie root", cfg.StateRoot, err)
	}

	w := &walker{cfg: cfg, sdb: sdb, workers: workers}
	return w.run(accountTrie)
}

// trieFailure converts trie-layer errors into named FAILs. Underlying
// transient I/O does not surface here (the trie layer swallows read errors
// into node absence); the rodb latch records it and the retry runner
// prefers the latched cause over this failure.
func trieFailure(where string, root common.Hash, err error) error {
	var missing *trie.MissingNodeError
	if errors.As(err, &missing) {
		return report.Failf("state_walk", "%s (root %s): missing trie node %s at path %x", where, root.Hex(), missing.NodeHash.Hex(), missing.Path)
	}
	return report.Failf("state_walk", "%s (root %s): %v", where, root.Hex(), err)
}

type accountJob struct {
	index    uint64
	leafKey  []byte // 32-byte hashed address (copied)
	leafBlob []byte // account RLP (copied)
}

type codeRef struct {
	class string // "contract" | "validator"
	hash  common.Hash
	size  uint64
}

type accountResult struct {
	index     uint64
	failure   error // *report.Failure or read error
	digest    [32]byte
	code      *codeRef
	counts    report.StateCounts
	anomalies *AnomalySet
}

type walker struct {
	cfg     Config
	sdb     state.Database
	workers int

	cancel atomic.Bool
}

func (w *walker) run(accountTrie state.Trie) (*Result, error) {
	jobs := make(chan accountJob, w.workers*2)
	results := make(chan accountResult, w.workers*2)

	var wg sync.WaitGroup
	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if w.cancel.Load() {
					results <- accountResult{index: job.index, failure: errCanceled}
					continue
				}
				results <- w.processAccount(job)
			}
		}()
	}

	collector := newCollector(w, results)

	// Sequential account-trie pass: authenticate every standalone node,
	// dispatch account leaves to the pool in trie order.
	var (
		accountTrieNodes uint64
		accounts         uint64
		trieFail         error
		nextIndex        uint64
	)
	it := accountTrie.NodeIterator(nil)
	for it.Next(true) {
		if w.cancel.Load() {
			break
		}
		if it.Hash() != (common.Hash{}) {
			accountTrieNodes++
			blob := it.NodeBlob()
			if blob == nil {
				// resolveBlob failed; surfaced via it.Error() below.
				break
			}
			if got := crypto.Keccak256Hash(blob); got != it.Hash() {
				trieFail = report.Failf("state_walk",
					"account trie node %s at path %x fails content authentication (blob hashes to %s)",
					it.Hash().Hex(), it.Path(), got.Hex())
				break
			}
		}
		if it.Leaf() {
			job := accountJob{
				index:    nextIndex,
				leafKey:  append([]byte(nil), it.LeafKey()...),
				leafBlob: append([]byte(nil), it.LeafBlob()...),
			}
			nextIndex++
			accounts++
			jobs <- job
			if w.cfg.Progress != nil && accounts%200000 == 0 {
				fmt.Fprintf(w.cfg.Progress, "state walk: %d accounts dispatched, %d account-trie nodes authenticated\n", accounts, accountTrieNodes)
			}
		}
	}
	if trieFail == nil {
		if err := it.Error(); err != nil {
			trieFail = trieFailure("account trie walk", w.cfg.StateRoot, err)
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	collector.wait()

	// Precedence: the earliest failure in walk order wins. Account leaves
	// dispatched before the trie-level failure position precede it.
	if collector.failure != nil && !errors.Is(collector.failure, errCanceled) {
		return nil, collector.failure
	}
	if trieFail != nil {
		return nil, trieFail
	}
	if collector.failure != nil {
		return nil, collector.failure
	}

	res := collector.result
	res.Counts.Accounts = accounts
	res.Counts.AccountTrieNodes = accountTrieNodes
	res.Digest = collector.digest.sum()
	if w.cfg.Progress != nil {
		fmt.Fprintf(w.cfg.Progress, "state walk: complete - %d accounts, %d account-trie nodes, %d storage tries, %d storage nodes, %d storage leaves, %d+%d unique code blobs\n",
			res.Counts.Accounts, res.Counts.AccountTrieNodes, res.Counts.StorageTries,
			res.Counts.StorageTrieNodes, res.Counts.StorageLeaves,
			res.Counts.UniqueCodeContract, res.Counts.UniqueCodeValidator)
	}
	return &res, nil
}

var errCanceled = errors.New("statecheck: canceled after earlier failure")

// collector folds worker results in account order, keeping the digest,
// counts and anomaly examples deterministic regardless of scheduling.
type collector struct {
	w       *walker
	digest  *stateDigest
	result  Result
	failure error
	done    chan struct{}

	uniqueCode map[codeRef]struct{}    // (class,hash) pairs seen
	classesFor map[common.Hash][2]bool // hash -> [contract, validator]
}

func newCollector(w *walker, results <-chan accountResult) *collector {
	c := &collector{
		w:          w,
		digest:     newStateDigest(w.cfg.StateRoot),
		done:       make(chan struct{}),
		uniqueCode: make(map[codeRef]struct{}),
		classesFor: make(map[common.Hash][2]bool),
	}
	c.result.Anomalies = NewAnomalySet()
	go c.loop(results)
	return c
}

func (c *collector) wait() { <-c.done }

func (c *collector) loop(results <-chan accountResult) {
	defer close(c.done)
	pending := make(map[uint64]accountResult)
	next := uint64(0)
	for r := range results {
		pending[r.index] = r
		for {
			rr, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			c.fold(rr)
		}
	}
}

func (c *collector) fold(r accountResult) {
	if c.failure != nil {
		return // draining after the first ordered failure
	}
	if r.failure != nil {
		c.failure = r.failure
		c.w.cancel.Store(true)
		return
	}
	c.digest.addAccount(r.digest)
	c.result.Counts.StorageTries += r.counts.StorageTries
	c.result.Counts.StorageTrieNodes += r.counts.StorageTrieNodes
	c.result.Counts.StorageLeaves += r.counts.StorageLeaves
	c.result.Anomalies.AddAll(r.anomalies)
	if r.code != nil {
		key := *r.code
		if r.code.class == "validator" {
			c.result.Counts.CodeRefsValidator++
		} else {
			c.result.Counts.CodeRefsContract++
		}
		if _, seen := c.uniqueCode[key]; !seen {
			c.uniqueCode[key] = struct{}{}
			c.result.Counts.UniqueCodeBytes += r.code.size
			if r.code.class == "validator" {
				c.result.Counts.UniqueCodeValidator++
			} else {
				c.result.Counts.UniqueCodeContract++
			}
			classes := c.classesFor[r.code.hash]
			if r.code.class == "validator" {
				classes[1] = true
			} else {
				classes[0] = true
			}
			c.classesFor[r.code.hash] = classes
			if classes[0] && classes[1] {
				c.result.Anomalies.Add(AnomalyCodeDualClass,
					fmt.Sprintf("code hash %s referenced as both contract and validator code", r.code.hash.Hex()))
			}
		}
	}
}

// processAccount runs the per-account checks: flag classification, full
// storage-trie walk, code resolution and validation, digest contribution.
func (w *walker) processAccount(job accountJob) accountResult {
	res := accountResult{index: job.index, anomalies: NewAnomalySet()}
	leafKeyHex := common.BytesToHash(job.leafKey).Hex()

	var acct state.Account
	if err := rlp.DecodeBytes(job.leafBlob, &acct); err != nil {
		res.failure = report.Failf("state_walk", "account leaf %s does not decode: %v", leafKeyHex, err)
		return res
	}
	if acct.Balance == nil {
		acct.Balance = new(big.Int)
	}
	hasStorage := acct.Root != emptyRootHash
	emptyCode := bytes.Equal(acct.CodeHash, emptyCodeHash.Bytes())
	addrHash := common.BytesToHash(job.leafKey)

	// Validator flag - for every account, independent of code presence.
	// Empty-root accounts are trivially unflagged. The leaf value is
	// decoded, not presence-tested, matching Object.IsValidator.
	flagged := false
	var storageTrie state.Trie
	if hasStorage {
		var err error
		storageTrie, err = w.sdb.OpenStorageTrie(w.cfg.StateRoot, addrHash, acct.Root)
		if err != nil {
			res.failure = trieFailure(fmt.Sprintf("storage trie open for account %s", leafKeyHex), acct.Root, err)
			return res
		}
		raw, err := storageTrie.TryGet(staking.IsValidatorKey.Bytes())
		if err != nil {
			res.failure = trieFailure(fmt.Sprintf("IsValidator flag lookup for account %s", leafKeyHex), acct.Root, err)
			return res
		}
		if len(raw) > 0 {
			_, content, _, err := rlp.Split(raw)
			if err != nil {
				res.failure = report.Failf("state_walk", "account %s IsValidator flag leaf is not an RLP byte string: %v", leafKeyHex, err)
				return res
			}
			value := common.BytesToHash(content)
			switch {
			case value == (common.Hash{}):
				// Stock SetState deletes zero-valued slots; a leaf whose RLP
				// decodes to zero is unflagged (decode-and-test) + anomaly.
				res.anomalies.Add(AnomalyFlagDecodedZero,
					fmt.Sprintf("account %s has an IsValidator flag leaf decoding to zero", leafKeyHex))
			case value == staking.IsValidator:
				flagged = true
			default:
				flagged = true
				res.anomalies.Add(AnomalyFlagNonCanonical,
					fmt.Sprintf("account %s IsValidator flag value %s differs from canonical", leafKeyHex, value.Hex()))
			}
		}
	}

	// Full storage walk with node content authentication.
	hStorage := newStorageDigest(!hasStorage)
	if hasStorage {
		res.counts.StorageTries++
		if failure := w.walkStorage(storageTrie, acct.Root, leafKeyHex, hStorage, &res); failure != nil {
			res.failure = failure
			return res
		}
	}

	// Code across the three namespaces.
	var hCode [32]byte
	if emptyCode {
		if flagged {
			res.failure = report.Failf("state_walk", "flagged validator account %s has empty code hash", leafKeyHex)
			return res
		}
		hCode = codeDigest(nil, true)
	} else {
		codeHash := common.BytesToHash(acct.CodeHash)
		code, class, failure := w.resolveCode(codeHash, flagged, job.leafKey, leafKeyHex, &res)
		if failure != nil {
			res.failure = failure
			return res
		}
		hCode = codeDigest(code, false)
		res.code = &codeRef{class: class, hash: codeHash, size: uint64(len(code))}
	}

	res.digest = accountDigest(job.leafKey, acct.Nonce, acct.Balance.Bytes(), acct.Root, acct.CodeHash, hStorage.sum(), hCode)
	return res
}

// walkStorage iterates the full storage trie, authenticating every
// standalone node and folding leaves into H_storage in trie order. The
// error status of every Next step is checked, including the very first
// (the stock iterator's defect-2 silently treats a storage trie whose
// initial step fails as empty).
func (w *walker) walkStorage(st state.Trie, root common.Hash, leafKeyHex string, h *storageDigest, res *accountResult) error {
	sit := st.NodeIterator(nil)
	for sit.Next(true) {
		if sit.Hash() != (common.Hash{}) {
			res.counts.StorageTrieNodes++
			blob := sit.NodeBlob()
			if blob == nil {
				break // surfaced via sit.Error()
			}
			if got := crypto.Keccak256Hash(blob); got != sit.Hash() {
				return report.Failf("state_walk",
					"storage trie node %s (account %s, path %x) fails content authentication (blob hashes to %s)",
					sit.Hash().Hex(), leafKeyHex, sit.Path(), got.Hex())
			}
		}
		if sit.Leaf() {
			blob := sit.LeafBlob()
			if len(blob) == 0 {
				return report.Failf("state_walk", "storage leaf %x of account %s has an empty value", sit.LeafKey(), leafKeyHex)
			}
			// Byte and String kinds are both byte strings in RLP (values
			// 0x00-0x7f encode as a single byte); lists are not.
			kind, content, rest, err := rlp.Split(blob)
			if err != nil || (kind != rlp.String && kind != rlp.Byte) || len(rest) != 0 {
				return report.Failf("state_walk", "storage leaf %x of account %s is not an RLP byte string", sit.LeafKey(), leafKeyHex)
			}
			h.addLeaf(append([]byte(nil), sit.LeafKey()...), content)
			res.counts.StorageLeaves++
		}
	}
	if err := sit.Error(); err != nil {
		return trieFailure(fmt.Sprintf("storage trie walk for account %s", leafKeyHex), root, err)
	}
	return nil
}

// resolveCode probes the raw code keys in order c -> vc -> legacy bare hash
// (physical location does not determine class), requires exactly one
// resolved location (identical bytes at multiple locations is an anomaly
// resolved by precedence; differing bytes FAIL), keccak-authenticates the
// bytes, and classifies from the account's flag: flagged accounts must
// carry a valid address-bound validator wrapper.
func (w *walker) resolveCode(codeHash common.Hash, flagged bool, leafKey []byte, leafKeyHex string, res *accountResult) ([]byte, string, error) {
	type loc struct {
		name string
		key  []byte
	}
	locs := []loc{
		{"c", append([]byte("c"), codeHash.Bytes()...)},
		{"vc", append([]byte("vc"), codeHash.Bytes()...)},
		{"legacy", codeHash.Bytes()},
	}
	var (
		found []string
		code  []byte
	)
	for _, l := range locs {
		val, ok, err := strictGet(w.cfg.KV, l.key)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			continue
		}
		if code == nil {
			code = val
		} else if !bytes.Equal(code, val) {
			return nil, "", report.Failf("state_walk",
				"code hash %s resolves to DIFFERENT bytes at locations %v and %s (account %s)",
				codeHash.Hex(), found, l.name, leafKeyHex)
		}
		found = append(found, l.name)
	}
	if len(found) == 0 {
		return nil, "", report.Failf("state_walk",
			"code %s for account %s missing from all namespaces (c, vc, legacy)", codeHash.Hex(), leafKeyHex)
	}
	if len(found) > 1 {
		res.anomalies.Add(AnomalyCodeMultiLocation,
			fmt.Sprintf("code %s present at %v (identical bytes); precedence %s", codeHash.Hex(), found, found[0]))
	}
	if got := crypto.Keccak256Hash(code); got != codeHash {
		return nil, "", report.Failf("state_walk",
			"code at %v for account %s hashes to %s, want %s", found, leafKeyHex, got.Hex(), codeHash.Hex())
	}

	var wrapper staketypes.ValidatorWrapper
	wrapperErr := rlp.DecodeBytes(code, &wrapper)
	if flagged {
		if wrapperErr != nil {
			return nil, "", report.Failf("state_walk",
				"flagged validator account %s: code %s does not decode as a validator wrapper: %v",
				leafKeyHex, codeHash.Hex(), wrapperErr)
		}
		if crypto.Keccak256Hash(wrapper.Address.Bytes()) != common.BytesToHash(leafKey) {
			return nil, "", report.Failf("state_walk",
				"flagged validator account %s: wrapper address %s does not bind to the account leaf key",
				leafKeyHex, wrapper.Address.Hex())
		}
		return code, "validator", nil
	}
	if wrapperErr == nil {
		res.anomalies.Add(AnomalyWrapperShapedContract,
			fmt.Sprintf("unflagged account %s carries wrapper-shaped code %s (stays contract)", leafKeyHex, codeHash.Hex()))
	}
	return code, "contract", nil
}

func strictGet(kv ethdb.KeyValueReader, key []byte) ([]byte, bool, error) {
	val, err := kv.Get(key)
	if err != nil {
		if rodb.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}
