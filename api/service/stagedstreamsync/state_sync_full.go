package stagedstreamsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	gomath "math"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	//"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/harmony-one/harmony/common/math"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/log/v3"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/sha3"
	// "github.com/ethereum/go-ethereum/eth/protocols/snap/range"
)

const (
	// minRequestSize is the minimum number of bytes to request from a remote peer.
	// This number is used as the low cap for account and storage range requests.
	// Bytecode and trienode are limited inherently by item count (1).
	minRequestSize = 64 * 1024

	// maxRequestSize is the maximum number of bytes to request from a remote peer.
	// This number is used as the high cap for account and storage range requests.
	// Bytecode and trienode are limited more explicitly by the caps below.
	maxRequestSize = 512 * 1024

	// maxCodeRequestCount is the maximum number of bytecode blobs to request in a
	// single query. If this number is too low, we're not filling responses fully
	// and waste round trip times. If it's too high, we're capping responses and
	// waste bandwidth.
	//
	// Deployed bytecodes are currently capped at 24KB, so the minimum request
	// size should be maxRequestSize / 24K. Assuming that most contracts do not
	// come close to that, requesting 4x should be a good approximation.
	maxCodeRequestCount = maxRequestSize / (24 * 1024) * 4

	// maxTrieRequestCount is the maximum number of trie node blobs to request in
	// a single query. If this number is too low, we're not filling responses fully
	// and waste round trip times. If it's too high, we're capping responses and
	// waste bandwidth.
	maxTrieRequestCount = maxRequestSize / 512

	// trienodeHealRateMeasurementImpact is the impact a single measurement has on
	// the local node's trienode processing capacity. A value closer to 0 reacts
	// slower to sudden changes, but it is also more stable against temporary hiccups.
	trienodeHealRateMeasurementImpact = 0.005

	// minTrienodeHealThrottle is the minimum divisor for throttling trie node
	// heal requests to avoid overloading the local node and excessively expanding
	// the state trie breadth wise.
	minTrienodeHealThrottle = 1

	// maxTrienodeHealThrottle is the maximum divisor for throttling trie node
	// heal requests to avoid overloading the local node and exessively expanding
	// the state trie bedth wise.
	maxTrienodeHealThrottle = maxTrieRequestCount

	// trienodeHealThrottleIncrease is the multiplier for the throttle when the
	// rate of arriving data is higher than the rate of processing it.
	trienodeHealThrottleIncrease = 1.33

	// trienodeHealThrottleDecrease is the divisor for the throttle when the
	// rate of arriving data is lower than the rate of processing it.
	trienodeHealThrottleDecrease = 1.25
)

// of only the account path. There's no need to be able to address both an
// account node and a storage node in the same request as it cannot happen
// that a slot is accessed before the account path is fully expanded.
type TrieNodePathSet [][]byte

var (
	// accountConcurrency is the number of chunks to split the account trie into
	// to allow concurrent retrievals.
	accountConcurrency = 16

	// storageConcurrency is the number of chunks to split the a large contract
	// storage trie into to allow concurrent retrievals.
	storageConcurrency = 16

	// MaxHash represents the maximum possible hash value.
	MaxHash = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
)

// accountTask represents the sync task for a chunk of the account snapshot.
type accountTask struct {
	id uint64 //unique id for account task

	root   common.Hash
	origin common.Hash
	limit  common.Hash
	cap    int

	// These fields get serialized to leveldb on shutdown
	Next     common.Hash                    // Next account to sync in this interval
	Last     common.Hash                    // Last account to sync in this interval
	SubTasks map[common.Hash][]*storageTask // Storage intervals needing fetching for large contracts

	pend int // Number of pending subtasks for this round

	needCode  []bool // Flags whether the filling accounts need code retrieval
	needState []bool // Flags whether the filling accounts need storage retrieval
	needHeal  []bool // Flags whether the filling accounts's state was chunked and need healing

	codeTasks  map[common.Hash]struct{}    // Code hashes that need retrieval
	stateTasks map[common.Hash]common.Hash // Account hashes->roots that need full state retrieval

	genBatch ethdb.Batch     // Batch used by the node generator
	genTrie  *trie.StackTrie // Node generator from storage slots

	requested bool
	done      bool // Flag whether the task can be removed

	res *accountResponse
}

// accountResponse is an already Merkle-verified remote response to an account
// range request. It contains the subtrie for the requested account range and
// the database that's going to be filled with the internal nodes on commit.
type accountResponse struct {
	task     *accountTask          // Task which this request is filling
	hashes   []common.Hash         // Account hashes in the returned range
	accounts []*types.StateAccount // Expanded accounts in the returned range
	cont     bool                  // Whether the account range has a continuation
}

// storageTask represents the sync task for a chunk of the storage snapshot.
type storageTask struct {
	Next      common.Hash     // Next account to sync in this interval
	Last      common.Hash     // Last account to sync in this interval
	root      common.Hash     // Storage root hash for this instance
	genBatch  ethdb.Batch     // Batch used by the node generator
	genTrie   *trie.StackTrie // Node generator from storage slots
	requested bool
	done      bool // Flag whether the task can be removed
}

// healRequestSort implements the Sort interface, allowing sorting trienode
// heal requests, which is a prerequisite for merging storage-requests.
type healRequestSort struct {
	paths     []string
	hashes    []common.Hash
	syncPaths []trie.SyncPath
}

func (t *healRequestSort) Len() int {
	return len(t.hashes)
}

func (t *healRequestSort) Less(i, j int) bool {
	a := t.syncPaths[i]
	b := t.syncPaths[j]
	switch bytes.Compare(a[0], b[0]) {
	case -1:
		return true
	case 1:
		return false
	}
	// identical first part
	if len(a) < len(b) {
		return true
	}
	if len(b) < len(a) {
		return false
	}
	if len(a) == 2 {
		return bytes.Compare(a[1], b[1]) < 0
	}
	return false
}

func (t *healRequestSort) Swap(i, j int) {
	t.paths[i], t.paths[j] = t.paths[j], t.paths[i]
	t.hashes[i], t.hashes[j] = t.hashes[j], t.hashes[i]
	t.syncPaths[i], t.syncPaths[j] = t.syncPaths[j], t.syncPaths[i]
}

// Merge merges the pathsets, so that several storage requests concerning the
// same account are merged into one, to reduce bandwidth.
// This operation is moot if t has not first been sorted.
func (t *healRequestSort) Merge() []*message.TrieNodePathSet {
	var result []TrieNodePathSet
	for _, path := range t.syncPaths {
		pathset := TrieNodePathSet(path)
		if len(path) == 1 {
			// It's an account reference.
			result = append(result, pathset)
		} else {
			// It's a storage reference.
			end := len(result) - 1
			if len(result) == 0 || !bytes.Equal(pathset[0], result[end][0]) {
				// The account doesn't match last, create a new entry.
				result = append(result, pathset)
			} else {
				// It's the same account as the previous one, add to the storage
				// paths of that request.
				result[end] = append(result[end], pathset[1])
			}
		}
	}
	// convert to array of pointers
	result_ptr := make([]*message.TrieNodePathSet, 0)
	for _, p := range result {
		result_ptr = append(result_ptr, &message.TrieNodePathSet{
			Pathset: p,
		})
	}
	return result_ptr
}

type byteCodeTasksBundle struct {
	id     uint64 //unique id for bytecode task bundle
	task   *accountTask
	hashes []common.Hash
	cap    int
}

type storageTaskBundle struct {
	id       uint64 //unique id for storage task bundle
	root     common.Hash
	accounts []common.Hash
	roots    []common.Hash
	mainTask *accountTask
	subtask  *storageTask
	origin   common.Hash
	limit    common.Hash
	cap      int
}

// healTask represents the sync task for healing the snap-synced chunk boundaries.
type healTask struct {
	id          uint64
	trieTasks   map[string]common.Hash   // Set of trie node tasks currently queued for retrieval, indexed by node path
	codeTasks   map[common.Hash]struct{} // Set of byte code tasks currently queued for retrieval, indexed by code hash
	paths       []string
	hashes      []common.Hash
	pathsets    []*message.TrieNodePathSet
	task        *healTask
	root        common.Hash
	bytes       int
	byteCodeReq bool
}

type tasks struct {
	accountTasks map[uint64]*accountTask         // Current account task set being synced
	storageTasks map[uint64]*storageTaskBundle   // Set of trie node tasks currently queued for retrieval, indexed by path
	codeTasks    map[uint64]*byteCodeTasksBundle // Set of byte code tasks currently queued for retrieval, indexed by hash
	healer       map[uint64]*healTask
}

func newTasks() *tasks {
	return &tasks{
		accountTasks: make(map[uint64]*accountTask, 0),
		storageTasks: make(map[uint64]*storageTaskBundle, 0),
		codeTasks:    make(map[uint64]*byteCodeTasksBundle),
		healer:       make(map[uint64]*healTask, 0),
	}
}

func (t *tasks) addAccountTask(accountTaskIndex uint64, ct *accountTask) {
	t.accountTasks[accountTaskIndex] = ct
}

func (t *tasks) getAccountTask(accountTaskIndex uint64) *accountTask {
	if _, ok := t.accountTasks[accountTaskIndex]; ok {
		return t.accountTasks[accountTaskIndex]
	}
	return nil
}

func (t *tasks) deleteAccountTask(accountTaskIndex uint64) {
	if _, ok := t.accountTasks[accountTaskIndex]; ok {
		delete(t.accountTasks, accountTaskIndex)
	}
}

func (t *tasks) addCodeTask(id uint64, bytecodeTask *byteCodeTasksBundle) {
	t.codeTasks[id] = bytecodeTask
}

func (t *tasks) deleteCodeTask(id uint64) {
	if _, ok := t.codeTasks[id]; ok {
		delete(t.codeTasks, id)
	}
}

func (t *tasks) addStorageTaskBundle(storageBundleIndex uint64, storages *storageTaskBundle) {
	t.storageTasks[storageBundleIndex] = storages
}

func (t *tasks) deleteStorageTaskBundle(storageBundleIndex uint64) {
	if _, ok := t.storageTasks[storageBundleIndex]; ok {
		delete(t.storageTasks, storageBundleIndex)
	}
}

func (t *tasks) addHealerTask(taskID uint64, task *healTask) {
	t.healer[taskID] = task
}

func (t *tasks) deleteHealerTask(taskID uint64) {
	if _, ok := t.healer[taskID]; ok {
		delete(t.healer, taskID)
	}
}

func (t *tasks) addHealerTrieTask(taskID uint64, path string, h common.Hash) {
	if _, ok := t.healer[taskID]; ok {
		t.healer[taskID].trieTasks[path] = h
	}
}

func (t *tasks) getHealerTrieTask(taskID uint64, path string) common.Hash {
	if _, ok := t.healer[taskID]; ok {
		return t.healer[taskID].trieTasks[path]
	}
	return common.Hash{}
}

func (t *tasks) addHealerTrieCodeTask(taskID uint64, hash common.Hash, v struct{}) {
	if _, ok := t.healer[taskID]; ok {
		t.healer[taskID].codeTasks[hash] = v
	}
}

func (t *tasks) getHealerTrieCodeTask(taskID uint64, h common.Hash) struct{} {
	if _, ok := t.healer[taskID]; ok {
		return t.healer[taskID].codeTasks[h]
	}
	return struct{}{}
}

// SyncProgress is a database entry to allow suspending and resuming a snapshot state
// sync. Opposed to full and fast sync, there is no way to restart a suspended
// snap sync without prior knowledge of the suspension point.
type SyncProgress struct {
	Tasks map[uint64]*accountTask // The suspended account tasks (contract tasks within)

	// Status report during syncing phase
	AccountSynced  uint64             // Number of accounts downloaded
	AccountBytes   common.StorageSize // Number of account trie bytes persisted to disk
	BytecodeSynced uint64             // Number of bytecodes downloaded
	BytecodeBytes  common.StorageSize // Number of bytecode bytes downloaded
	StorageSynced  uint64             // Number of storage slots downloaded
	StorageBytes   common.StorageSize // Number of storage trie bytes persisted to disk

	// Status report during healing phase
	TrienodeHealSynced uint64             // Number of state trie nodes downloaded
	TrienodeHealBytes  common.StorageSize // Number of state trie bytes persisted to disk
	BytecodeHealSynced uint64             // Number of bytecodes downloaded
	BytecodeHealBytes  common.StorageSize // Number of bytecodes persisted to disk
}

// FullStateDownloadManager is the helper structure for get blocks request management
type FullStateDownloadManager struct {
	bc core.BlockChain
	tx kv.RwTx

	db     ethdb.KeyValueStore // Database to store the trie nodes into (and dedup)
	scheme string              // Node scheme used in node database

	tasks      *tasks
	requesting *tasks
	processing *tasks
	retries    *tasks

	root    common.Hash // Current state trie root being synced
	snapped bool        // Flag to signal that snap phase is done

	protocol    syncProtocol
	scheduler   *trie.Sync         // State trie sync scheduler defining the tasks
	keccak      crypto.KeccakState // Keccak256 hasher to verify deliveries with
	concurrency int
	logger      zerolog.Logger
	lock        sync.RWMutex

	numUncommitted   int
	bytesUncommitted int

	accountSynced  uint64             // Number of accounts downloaded
	accountBytes   common.StorageSize // Number of account trie bytes persisted to disk
	bytecodeSynced uint64             // Number of bytecodes downloaded
	bytecodeBytes  common.StorageSize // Number of bytecode bytes downloaded
	storageSynced  uint64             // Number of storage slots downloaded
	storageBytes   common.StorageSize // Number of storage trie bytes persisted to disk

	stateWriter        ethdb.Batch        // Shared batch writer used for persisting raw states
	accountHealed      uint64             // Number of accounts downloaded during the healing stage
	accountHealedBytes common.StorageSize // Number of raw account bytes persisted to disk during the healing stage
	storageHealed      uint64             // Number of storage slots downloaded during the healing stage
	storageHealedBytes common.StorageSize // Number of raw storage bytes persisted to disk during the healing stage

	trienodeHealRate      float64       // Average heal rate for processing trie node data
	trienodeHealPend      atomic.Uint64 // Number of trie nodes currently pending for processing
	trienodeHealThrottle  float64       // Divisor for throttling the amount of trienode heal data requested
	trienodeHealThrottled time.Time     // Timestamp the last time the throttle was updated

	trienodeHealSynced uint64             // Number of state trie nodes downloaded
	trienodeHealBytes  common.StorageSize // Number of state trie bytes persisted to disk
	trienodeHealDups   uint64             // Number of state trie nodes already processed
	trienodeHealNops   uint64             // Number of state trie nodes not requested
	bytecodeHealSynced uint64             // Number of bytecodes downloaded
	bytecodeHealBytes  common.StorageSize // Number of bytecodes persisted to disk
	bytecodeHealDups   uint64             // Number of bytecodes already processed
	bytecodeHealNops   uint64             // Number of bytecodes not requested

	startTime time.Time // Time instance when snapshot sync started
	logTime   time.Time // Time instance when status was last reported
}

func newFullStateDownloadManager(db ethdb.KeyValueStore,
	scheme string,
	tx kv.RwTx,
	bc core.BlockChain,
	concurrency int,
	logger zerolog.Logger) *FullStateDownloadManager {

	return &FullStateDownloadManager{
		db:                   db,
		scheme:               scheme,
		bc:                   bc,
		stateWriter:          db.NewBatch(),
		tx:                   tx,
		keccak:               sha3.NewLegacyKeccak256().(crypto.KeccakState),
		concurrency:          concurrency,
		logger:               logger,
		tasks:                newTasks(),
		requesting:           newTasks(),
		processing:           newTasks(),
		retries:              newTasks(),
		trienodeHealThrottle: maxTrienodeHealThrottle, // Tune downward instead of insta-filling with junk
	}
}

func (s *FullStateDownloadManager) setRootHash(root common.Hash) {
	s.root = root
	s.scheduler = state.NewStateSync(root, s.db, s.onHealState, s.scheme)
	s.loadSyncStatus()
}

func (s *FullStateDownloadManager) taskDone(taskID uint64) {
	s.tasks.accountTasks[taskID].done = true
}

// SlimAccount is a modified version of an Account, where the root is replaced
// with a byte slice. This format can be used to represent full-consensus format
// or slim format which replaces the empty root and code hash as nil byte slice.
type SlimAccount struct {
	Nonce    uint64
	Balance  *big.Int
	Root     []byte // Nil if root equals to types.EmptyRootHash
	CodeHash []byte // Nil if hash equals to types.EmptyCodeHash
}

// SlimAccountRLP encodes the state account in 'slim RLP' format.
func (s *FullStateDownloadManager) SlimAccountRLP(account types.StateAccount) []byte {
	slim := SlimAccount{
		Nonce:   account.Nonce,
		Balance: account.Balance,
	}
	if account.Root != types.EmptyRootHash {
		slim.Root = account.Root[:]
	}
	if !bytes.Equal(account.CodeHash, types.EmptyCodeHash[:]) {
		slim.CodeHash = account.CodeHash
	}
	data, err := rlp.EncodeToBytes(slim)
	if err != nil {
		panic(err)
	}
	return data
}

// FullAccount decodes the data on the 'slim RLP' format and returns
// the consensus format account.
func FullAccount(data []byte) (*types.StateAccount, error) {
	var slim SlimAccount
	if err := rlp.DecodeBytes(data, &slim); err != nil {
		return nil, err
	}
	var account types.StateAccount
	account.Nonce, account.Balance = slim.Nonce, slim.Balance

	// Interpret the storage root and code hash in slim format.
	if len(slim.Root) == 0 {
		account.Root = types.EmptyRootHash
	} else {
		account.Root = common.BytesToHash(slim.Root)
	}
	if len(slim.CodeHash) == 0 {
		account.CodeHash = types.EmptyCodeHash[:]
	} else {
		account.CodeHash = slim.CodeHash
	}
	return &account, nil
}

// FullAccountRLP converts data on the 'slim RLP' format into the full RLP-format.
func FullAccountRLP(data []byte) ([]byte, error) {
	account, err := FullAccount(data)
	if err != nil {
		return nil, err
	}
	return rlp.EncodeToBytes(account)
}

func (s *FullStateDownloadManager) commitHealer(force bool) {
	if !force && s.scheduler.MemSize() < ethdb.IdealBatchSize {
		return
	}
	batch := s.db.NewBatch()
	if err := s.scheduler.Commit(batch); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to commit healing data")
	}
	if err := batch.Write(); err != nil {
		log.Crit("Failed to persist healing data", "err", err)
	}
	utils.Logger().Debug().Str("type", "trienodes").Interface("bytes", common.StorageSize(batch.ValueSize())).Msg("Persisted set of healing data")
}

func (s *FullStateDownloadManager) SyncStarted() {
	if s.startTime == (time.Time{}) {
		s.startTime = time.Now()
	}
}

func (s *FullStateDownloadManager) SyncCompleted() {
	defer func() { // Persist any progress, independent of failure
		for _, task := range s.tasks.accountTasks {
			s.forwardAccountTask(task)
		}
		s.cleanAccountTasks()
		s.saveSyncStatus()
	}()

	// Flush out the last committed raw states
	defer func() {
		if s.stateWriter.ValueSize() > 0 {
			s.stateWriter.Write()
			s.stateWriter.Reset()
		}
	}()

	// commit any trie- and bytecode-healing data.
	defer s.commitHealer(true)

	// Whether sync completed or not, disregard any future packets
	defer func() {
		utils.Logger().Debug().Interface("root", s.root).Msg("Terminating snapshot sync cycle")
	}()

	elapsed := time.Since(s.startTime)
	utils.Logger().Debug().Interface("elapsed", elapsed).Msg("Snapshot sync already completed")
}

// getNextBatch returns objects with a maximum of n state download
// tasks to send to the remote peer.
func (s *FullStateDownloadManager) GetNextBatch() (accounts []*accountTask,
	codes []*byteCodeTasksBundle,
	storages *storageTaskBundle,
	healtask *healTask,
	codetask *healTask,
	nItems int,
	err error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	accounts, codes, storages, healtask, codetask, nItems = s.getBatchFromRetries()

	if nItems > 0 {
		return
	}

	if len(s.tasks.accountTasks) == 0 && s.scheduler.Pending() == 0 {
		s.SyncCompleted()
		return
	}

	// Refill available tasks from the scheduler.
	newAccounts, newCodes, newStorageTaskBundle, newHealTask, newCodeTask, nItems := s.getBatchFromUnprocessed()
	accounts = append(accounts, newAccounts...)
	codes = append(codes, newCodes...)
	storages = newStorageTaskBundle
	healtask = newHealTask
	codetask = newCodeTask

	return
}

// saveSyncStatus marshals the remaining sync tasks into leveldb.
func (s *FullStateDownloadManager) saveSyncStatus() {
	// Serialize any partial progress to disk before spinning down
	for _, task := range s.tasks.accountTasks {
		if err := task.genBatch.Write(); err != nil {
			utils.Logger().Debug().
				Err(err).
				Msg("Failed to persist account slots")
		}
		for _, subtasks := range task.SubTasks {
			for _, subtask := range subtasks {
				if err := subtask.genBatch.Write(); err != nil {
					utils.Logger().Debug().
						Err(err).
						Msg("Failed to persist storage slots")
				}
			}
		}
	}
	// Store the actual progress markers
	progress := &SyncProgress{
		Tasks:              s.tasks.accountTasks,
		AccountSynced:      s.accountSynced,
		AccountBytes:       s.accountBytes,
		BytecodeSynced:     s.bytecodeSynced,
		BytecodeBytes:      s.bytecodeBytes,
		StorageSynced:      s.storageSynced,
		StorageBytes:       s.storageBytes,
		TrienodeHealSynced: s.trienodeHealSynced,
		TrienodeHealBytes:  s.trienodeHealBytes,
		BytecodeHealSynced: s.bytecodeHealSynced,
		BytecodeHealBytes:  s.bytecodeHealBytes,
	}
	status, err := json.Marshal(progress)
	if err != nil {
		panic(err) // This can only fail during implementation
	}
	rawdb.WriteSnapshotSyncStatus(s.db, status)
}

// loadSyncStatus retrieves a previously aborted sync status from the database,
// or generates a fresh one if none is available.
func (s *FullStateDownloadManager) loadSyncStatus() {
	var progress SyncProgress

	if status := rawdb.ReadSnapshotSyncStatus(s.db); status != nil {
		if err := json.Unmarshal(status, &progress); err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("Failed to decode snap sync status")
		} else {
			for _, task := range progress.Tasks {
				utils.Logger().Debug().
					Interface("from", task.Next).
					Interface("last", task.Last).
					Msg("Scheduled account sync task")
			}
			s.tasks.accountTasks = progress.Tasks
			for _, task := range s.tasks.accountTasks {
				task := task // closure for task.genBatch in the stacktrie writer callback

				task.genBatch = ethdb.HookedBatch{
					Batch: s.db.NewBatch(),
					OnPut: func(key []byte, value []byte) {
						s.accountBytes += common.StorageSize(len(key) + len(value))
					},
				}
				// options := trie.NewStackTrieOptions()
				writeFn := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
					rawdb.WriteTrieNode(task.genBatch, common.Hash{}, path, hash, blob, s.scheme)
				}
				task.genTrie = trie.NewStackTrie(writeFn)
				for accountHash, subtasks := range task.SubTasks {
					for _, subtask := range subtasks {
						subtask := subtask // closure for subtask.genBatch in the stacktrie writer callback

						subtask.genBatch = ethdb.HookedBatch{
							Batch: s.db.NewBatch(),
							OnPut: func(key []byte, value []byte) {
								s.storageBytes += common.StorageSize(len(key) + len(value))
							},
						}
						// owner := accountHash // local assignment for stacktrie writer closure
						writeFn = func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
							rawdb.WriteTrieNode(subtask.genBatch, accountHash, path, hash, blob, s.scheme)
						}
						subtask.genTrie = trie.NewStackTrie(writeFn)
					}
				}
			}
			s.lock.Lock()
			defer s.lock.Unlock()

			s.snapped = len(s.tasks.accountTasks) == 0

			s.accountSynced = progress.AccountSynced
			s.accountBytes = progress.AccountBytes
			s.bytecodeSynced = progress.BytecodeSynced
			s.bytecodeBytes = progress.BytecodeBytes
			s.storageSynced = progress.StorageSynced
			s.storageBytes = progress.StorageBytes

			s.trienodeHealSynced = progress.TrienodeHealSynced
			s.trienodeHealBytes = progress.TrienodeHealBytes
			s.bytecodeHealSynced = progress.BytecodeHealSynced
			s.bytecodeHealBytes = progress.BytecodeHealBytes
			return
		}
	}
	// Either we've failed to decode the previous state, or there was none.
	// Start a fresh sync by chunking up the account range and scheduling
	// them for retrieval.
	s.tasks = newTasks()
	s.accountSynced, s.accountBytes = 0, 0
	s.bytecodeSynced, s.bytecodeBytes = 0, 0
	s.storageSynced, s.storageBytes = 0, 0
	s.trienodeHealSynced, s.trienodeHealBytes = 0, 0
	s.bytecodeHealSynced, s.bytecodeHealBytes = 0, 0

	var next common.Hash
	step := new(big.Int).Sub(
		new(big.Int).Div(
			new(big.Int).Exp(common.Big2, common.Big256, nil),
			big.NewInt(int64(accountConcurrency)),
		), common.Big1,
	)
	for i := 0; i < accountConcurrency; i++ {
		last := common.BigToHash(new(big.Int).Add(next.Big(), step))
		if i == accountConcurrency-1 {
			// Make sure we don't overflow if the step is not a proper divisor
			last = MaxHash
		}
		batch := ethdb.HookedBatch{
			Batch: s.db.NewBatch(),
			OnPut: func(key []byte, value []byte) {
				s.accountBytes += common.StorageSize(len(key) + len(value))
			},
		}
		// options := trie.NewStackTrieOptions()
		writeFn := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
			rawdb.WriteTrieNode(batch, common.Hash{}, path, hash, blob, s.scheme)
		}
		// create a unique id for task
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.accountTasks[taskID]; ok {
				continue
			}
			break
		}
		s.tasks.addAccountTask(taskID, &accountTask{
			id:       taskID,
			Next:     next,
			Last:     last,
			SubTasks: make(map[common.Hash][]*storageTask),
			genBatch: batch,
			genTrie:  trie.NewStackTrie(writeFn),
		})
		utils.Logger().Debug().
			Interface("from", next).
			Interface("last", last).
			Msg("Created account sync task")

		next = common.BigToHash(new(big.Int).Add(last.Big(), common.Big1))
	}
}

// cleanAccountTasks removes account range retrieval tasks that have already been
// completed.
func (s *FullStateDownloadManager) cleanAccountTasks() {
	// If the sync was already done before, don't even bother
	if len(s.tasks.accountTasks) == 0 {
		return
	}
	// Sync wasn't finished previously, check for any task that can be finalized
	for taskID, _ := range s.tasks.accountTasks {
		if s.tasks.accountTasks[taskID].done {
			s.tasks.deleteAccountTask(taskID)
		}
	}
	// If everything was just finalized just, generate the account trie and start heal
	if len(s.tasks.accountTasks) == 0 {
		s.lock.Lock()
		s.snapped = true
		s.lock.Unlock()

		// Push the final sync report
		//s.reportSyncProgress(true)
	}
}

// cleanStorageTasks iterates over all the account tasks and storage sub-tasks
// within, cleaning any that have been completed.
func (s *FullStateDownloadManager) cleanStorageTasks() {
	for _, task := range s.tasks.accountTasks {
		for account, subtasks := range task.SubTasks {
			// Remove storage range retrieval tasks that completed
			for j := 0; j < len(subtasks); j++ {
				if subtasks[j].done {
					subtasks = append(subtasks[:j], subtasks[j+1:]...)
					j--
				}
			}
			if len(subtasks) > 0 {
				task.SubTasks[account] = subtasks
				continue
			}
			// If all storage chunks are done, mark the account as done too
			for j, hash := range task.res.hashes {
				if hash == account {
					task.needState[j] = false
				}
			}
			delete(task.SubTasks, account)
			task.pend--

			// If this was the last pending task, forward the account task
			if task.pend == 0 {
				s.forwardAccountTask(task)
			}
		}
	}
}

// forwardAccountTask takes a filled account task and persists anything available
// into the database, after which it forwards the next account marker so that the
// task's next chunk may be filled.
func (s *FullStateDownloadManager) forwardAccountTask(task *accountTask) {
	// Remove any pending delivery
	res := task.res
	if res == nil {
		return // nothing to forward
	}
	task.res = nil

	// Persist the received account segments. These flat state maybe
	// outdated during the sync, but it can be fixed later during the
	// snapshot generation.
	oldAccountBytes := s.accountBytes

	batch := ethdb.HookedBatch{
		Batch: s.db.NewBatch(),
		OnPut: func(key []byte, value []byte) {
			s.accountBytes += common.StorageSize(len(key) + len(value))
		},
	}
	for i, hash := range res.hashes {
		if task.needCode[i] || task.needState[i] {
			break
		}
		slim := s.SlimAccountRLP(*res.accounts[i])
		rawdb.WriteAccountSnapshot(batch, hash, slim)

		// If the task is complete, drop it into the stack trie to generate
		// account trie nodes for it
		if !task.needHeal[i] {
			full, err := FullAccountRLP(slim) // TODO(karalabe): Slim parsing can be omitted
			if err != nil {
				panic(err) // Really shouldn't ever happen
			}
			task.genTrie.Update(hash[:], full)
		}
	}
	// Flush anything written just now and update the stats
	if err := batch.Write(); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to persist accounts")
	}
	s.accountSynced += uint64(len(res.accounts))

	// Task filling persisted, push it the chunk marker forward to the first
	// account still missing data.
	for i, hash := range res.hashes {
		if task.needCode[i] || task.needState[i] {
			return
		}
		task.Next = incHash(hash)
	}
	// All accounts marked as complete, track if the entire task is done
	task.done = !res.cont

	// Stack trie could have generated trie nodes, push them to disk (we need to
	// flush after finalizing task.done. It's fine even if we crash and lose this
	// write as it will only cause more data to be downloaded during heal.
	if task.done {
		task.genTrie.Commit()
	}
	if task.genBatch.ValueSize() > ethdb.IdealBatchSize || task.done {
		if err := task.genBatch.Write(); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to persist stack account")
		}
		task.genBatch.Reset()
	}
	utils.Logger().Debug().
		Int("accounts", len(res.accounts)).
		Float64("bytes", float64(s.accountBytes-oldAccountBytes)).
		Msg("Persisted range of accounts")
}

// updateStats bumps the various state sync progress counters and displays a log
// message for the user to see.
func (s *FullStateDownloadManager) updateStats(written, duplicate, unexpected int, duration time.Duration) {
	// TODO: here it updates the stats for total pending, processed, duplicates and unexpected

	// for now, we just jog current stats
	if written > 0 || duplicate > 0 || unexpected > 0 {
		utils.Logger().Info().
			Int("count", written).
			Int("duplicate", duplicate).
			Int("unexpected", unexpected).
			Msg("Imported new state entries")
	}
}

// getBatchFromUnprocessed returns objects with a maximum of n unprocessed state download
// tasks to send to the remote peer.
func (s *FullStateDownloadManager) getBatchFromUnprocessed() (
	accounts []*accountTask,
	codes []*byteCodeTasksBundle,
	storages *storageTaskBundle,
	healtask *healTask,
	codetask *healTask,
	count int) {

	// over trie nodes as those can be written to disk and forgotten about.
	codes = make([]*byteCodeTasksBundle, 0)
	accounts = make([]*accountTask, 0)
	count = 0

	for i, task := range s.tasks.accountTasks {
		// Stop when we've gathered enough requests
		// if len(accounts) == n {
		// 	return
		// }

		// if already requested
		if task.requested {
			continue
		}

		// create a unique id for healer task
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.accountTasks[taskID]; ok {
				continue
			}
			break
		}

		task.root = s.root
		task.origin = task.Next
		task.limit = task.Last
		task.cap = maxRequestSize
		task.requested = true
		s.tasks.accountTasks[i].requested = true
		accounts = append(accounts, task)
		s.requesting.addAccountTask(task.id, task)
		s.tasks.addAccountTask(task.id, task)

		// one task account is enough for an stream
		count = len(accounts)
		return
	}

	totalHashes := int(0)

	for _, task := range s.tasks.accountTasks {
		// Skip tasks that are already retrieving (or done with) all codes
		if len(task.codeTasks) == 0 {
			continue
		}

		var hashes []common.Hash
		for hash := range task.codeTasks {
			delete(task.codeTasks, hash)
			hashes = append(hashes, hash)
		}
		totalHashes += len(hashes)

		// create a unique id for task bundle
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.codeTasks[taskID]; ok {
				continue
			}
			break
		}

		bytecodeTask := &byteCodeTasksBundle{
			id:     taskID,
			hashes: hashes,
			task:   task,
			cap:    maxRequestSize,
		}
		codes = append(codes, bytecodeTask)

		s.requesting.addCodeTask(taskID, bytecodeTask)
		s.tasks.addCodeTask(taskID, bytecodeTask)

		// Stop when we've gathered enough requests
		if totalHashes >= maxCodeRequestCount {
			count = totalHashes
			return
		}
	}

	// if we found some codes, can assign it to node
	if totalHashes > 0 {
		count = totalHashes
		return
	}

	for accTaskID, task := range s.tasks.accountTasks {
		// Skip tasks that are already retrieving (or done with) all small states
		if len(task.SubTasks) == 0 && len(task.stateTasks) == 0 {
			continue
		}

		cap := maxRequestSize
		storageSets := cap / 1024

		storages = &storageTaskBundle{
			accounts: make([]common.Hash, 0, storageSets),
			roots:    make([]common.Hash, 0, storageSets),
			mainTask: task,
		}

		// create a unique id for task bundle
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.storageTasks[taskID]; ok {
				continue
			}
			break
		}
		storages.id = taskID

		for account, subtasks := range task.SubTasks {
			// find the first subtask which is not requested yet
			for i, st := range subtasks {
				// Skip any subtasks already filling
				if st.requested {
					continue
				}
				// Found an incomplete storage chunk, schedule it
				storages.accounts = append(storages.accounts, account)
				storages.roots = append(storages.roots, st.root)
				storages.subtask = st
				s.tasks.accountTasks[accTaskID].SubTasks[account][i].requested = true
				break // Large contract chunks are downloaded individually
			}
			if storages.subtask != nil {
				break // Large contract chunks are downloaded individually
			}
		}
		if storages.subtask == nil {
			// No large contract required retrieval, but small ones available
			for account, root := range task.stateTasks {
				delete(task.stateTasks, account)

				storages.accounts = append(storages.accounts, account)
				storages.roots = append(storages.roots, root)

				if len(storages.accounts) >= storageSets {
					break
				}
			}
		}
		// If nothing was found, it means this task is actually already fully
		// retrieving, but large contracts are hard to detect. Skip to the next.
		if len(storages.accounts) == 0 {
			continue
		}
		if storages.subtask != nil {
			storages.origin = storages.subtask.Next
			storages.limit = storages.subtask.Last
		}
		storages.root = s.root
		storages.cap = cap
		s.tasks.addStorageTaskBundle(taskID, storages)
		s.requesting.addStorageTaskBundle(taskID, storages)
		count = len(storages.accounts)
		return
	}

	if len(storages.accounts) > 0 {
		count = len(storages.accounts)
		return
	}

	// Sync phase done, run heal phase
	// Iterate over pending tasks
	for (len(s.tasks.healer) > 0 && len(s.tasks.healer[0].hashes) > 0) || s.scheduler.Pending() > 0 {
		// If there are not enough trie tasks queued to fully assign, fill the
		// queue from the state sync scheduler. The trie synced schedules these
		// together with bytecodes, so we need to queue them combined.

		// index 0 keeps all tasks, later we split it into multiple batch
		if len(s.tasks.healer) == 0 {
			s.tasks.healer[0] = &healTask{
				trieTasks: make(map[string]common.Hash, 0),
				codeTasks: make(map[common.Hash]struct{}, 0),
			}
		}

		mPaths, mHashes, mCodes := s.scheduler.Missing(maxTrieRequestCount)
		for i, path := range mPaths {
			s.tasks.healer[0].trieTasks[path] = mHashes[i]
		}
		for _, hash := range mCodes {
			s.tasks.healer[0].codeTasks[hash] = struct{}{}
		}

		// If all the heal tasks are bytecodes or already downloading, bail
		if len(s.tasks.healer[0].trieTasks) == 0 {
			break
		}
		// Generate the network query and send it to the peer
		// if cap > maxTrieRequestCount {
		// 	cap = maxTrieRequestCount
		// }
		cap := int(float64(maxTrieRequestCount) / s.trienodeHealThrottle)
		if cap <= 0 {
			cap = 1
		}
		var (
			hashes   = make([]common.Hash, 0, cap)
			paths    = make([]string, 0, cap)
			pathsets = make([]*message.TrieNodePathSet, 0, cap)
		)
		for path, hash := range s.tasks.healer[0].trieTasks {
			delete(s.tasks.healer[0].trieTasks, path)

			paths = append(paths, path)
			hashes = append(hashes, hash)
			if len(paths) >= cap {
				break
			}
		}

		// Group requests by account hash
		paths, hashes, _, pathsets = sortByAccountPath(paths, hashes)

		// create a unique id for healer task
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.healer[taskID]; ok {
				continue
			}
			break
		}

		healtask = &healTask{
			id:          taskID,
			hashes:      hashes,
			paths:       paths,
			pathsets:    pathsets,
			root:        s.root,
			task:        s.tasks.healer[0],
			bytes:       maxRequestSize,
			byteCodeReq: false,
		}

		s.tasks.healer[taskID] = healtask
		s.requesting.addHealerTask(taskID, healtask)

		if len(hashes) > 0 {
			count = len(hashes)
			return
		}
	}

	// trying to get bytecodes
	// Iterate over pending tasks and try to find a peer to retrieve with
	for (len(s.tasks.healer) > 0 && len(s.tasks.healer[0].codeTasks) > 0) || s.scheduler.Pending() > 0 {
		// If there are not enough trie tasks queued to fully assign, fill the
		// queue from the state sync scheduler. The trie synced schedules these
		// together with trie nodes, so we need to queue them combined.

		mPaths, mHashes, mCodes := s.scheduler.Missing(maxTrieRequestCount)
		for i, path := range mPaths {
			s.tasks.healer[0].trieTasks[path] = mHashes[i]
		}
		for _, hash := range mCodes {
			s.tasks.healer[0].codeTasks[hash] = struct{}{}
		}

		// If all the heal tasks are trienodes or already downloading, bail
		if len(s.tasks.healer[0].codeTasks) == 0 {
			break
		}
		// Task pending retrieval, try to find an idle peer. If no such peer
		// exists, we probably assigned tasks for all (or they are stateless).
		// Abort the entire assignment mechanism.

		// Generate the network query and send it to the peer
		// if cap > maxCodeRequestCount {
		// 	cap = maxCodeRequestCount
		// }
		cap := maxCodeRequestCount
		hashes := make([]common.Hash, 0, cap)
		for hash := range s.tasks.healer[0].codeTasks {
			delete(s.tasks.healer[0].codeTasks, hash)

			hashes = append(hashes, hash)
			if len(hashes) >= cap {
				break
			}
		}

		// create a unique id for healer task
		var taskID uint64
		for {
			taskID = uint64(rand.Int63())
			if taskID == 0 {
				continue
			}
			if _, ok := s.tasks.healer[taskID]; ok {
				continue
			}
			break
		}

		codetask = &healTask{
			id:          taskID,
			hashes:      hashes,
			task:        s.tasks.healer[0],
			bytes:       maxRequestSize,
			byteCodeReq: true,
		}
		count = len(hashes)
		s.tasks.healer[taskID] = codetask
		s.requesting.addHealerTask(taskID, healtask)
	}

	return
}

// sortByAccountPath takes hashes and paths, and sorts them. After that, it generates
// the TrieNodePaths and merges paths which belongs to the same account path.
func sortByAccountPath(paths []string, hashes []common.Hash) ([]string, []common.Hash, []trie.SyncPath, []*message.TrieNodePathSet) {
	var syncPaths []trie.SyncPath
	for _, path := range paths {
		syncPaths = append(syncPaths, trie.NewSyncPath([]byte(path)))
	}
	n := &healRequestSort{paths, hashes, syncPaths}
	sort.Sort(n)
	pathsets := n.Merge()
	return n.paths, n.hashes, n.syncPaths, pathsets
}

// getBatchFromRetries get the block number batch to be requested from retries.
func (s *FullStateDownloadManager) getBatchFromRetries() (
	accounts []*accountTask,
	codes []*byteCodeTasksBundle,
	storages *storageTaskBundle,
	healtask *healTask,
	codetask *healTask,
	count int) {

	// over trie nodes as those can be written to disk and forgotten about.
	accounts = make([]*accountTask, 0)
	codes = make([]*byteCodeTasksBundle, 0)

	for _, task := range s.retries.accountTasks {
		// Stop when we've gathered enough requests
		// if len(accounts) == n {
		// 	return
		// }
		accounts = append(accounts, task)
		s.requesting.addAccountTask(task.id, task)
		s.retries.deleteAccountTask(task.id)
		return
	}

	if len(accounts) > 0 {
		count = len(accounts)
		return
	}

	for _, code := range s.retries.codeTasks {
		codes = append(codes, code)
		s.requesting.addCodeTask(code.id, code)
		s.retries.deleteCodeTask(code.id)
		return
	}

	if len(codes) > 0 {
		count = len(codes)
		return
	}

	if s.retries.storageTasks != nil && len(s.retries.storageTasks) > 0 {
		storages = &storageTaskBundle{
			id:       s.retries.storageTasks[0].id,
			accounts: s.retries.storageTasks[0].accounts,
			roots:    s.retries.storageTasks[0].roots,
			mainTask: s.retries.storageTasks[0].mainTask,
			subtask:  s.retries.storageTasks[0].subtask,
			limit:    s.retries.storageTasks[0].limit,
			origin:   s.retries.storageTasks[0].origin,
		}
		s.requesting.addStorageTaskBundle(storages.id, storages)
		s.retries.deleteStorageTaskBundle(storages.id)
		count = len(storages.accounts)
		return
	}

	if s.retries.healer != nil && len(s.retries.healer) > 0 {

		for id, task := range s.retries.healer {
			if !task.byteCodeReq {
				healtask = &healTask{
					id:          id,
					hashes:      task.hashes,
					paths:       task.paths,
					pathsets:    task.pathsets,
					root:        task.root,
					task:        task.task,
					byteCodeReq: task.byteCodeReq,
				}
				s.requesting.addHealerTask(id, task)
				s.retries.deleteHealerTask(id)
				count = len(task.hashes)
				return
			}
			if task.byteCodeReq {
				codetask = &healTask{
					id:          id,
					hashes:      task.hashes,
					paths:       task.paths,
					pathsets:    task.pathsets,
					root:        task.root,
					task:        task.task,
					byteCodeReq: task.byteCodeReq,
				}
				s.requesting.addHealerTask(id, task)
				s.retries.deleteHealerTask(id)
				count = len(task.hashes)
				return
			}
		}
	}

	count = 0
	return
}

// HandleRequestError handles the error result
func (s *FullStateDownloadManager) HandleRequestError(accounts []*accountTask,
	codes []*byteCodeTasksBundle,
	storages *storageTaskBundle,
	healtask *healTask,
	codetask *healTask,
	streamID sttypes.StreamID, err error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	if accounts != nil && len(accounts) > 0 {
		for _, task := range accounts {
			s.requesting.deleteAccountTask(task.id)
			s.retries.addAccountTask(task.id, task)
		}
	}

	if codes != nil && len(codes) > 0 {
		for _, code := range codes {
			s.requesting.deleteCodeTask(code.id)
			s.retries.addCodeTask(code.id, code)
		}
	}

	if storages != nil {
		s.requesting.addStorageTaskBundle(storages.id, storages)
		s.retries.deleteStorageTaskBundle(storages.id)
	}

	if healtask != nil {
		s.retries.addHealerTask(healtask.id, healtask)
		s.requesting.deleteHealerTask(healtask.id)
	}

	if codetask != nil {
		s.retries.addHealerTask(codetask.id, codetask)
		s.requesting.deleteHealerTask(codetask.id)
	}
}

// UnpackAccountRanges retrieves the accounts from the range packet and converts from slim
// wire representation to consensus format. The returned data is RLP encoded
// since it's expected to be serialized to disk without further interpretation.
//
// Note, this method does a round of RLP decoding and re-encoding, so only use it
// once and cache the results if need be. Ideally discard the packet afterwards
// to not double the memory use.
func (s *FullStateDownloadManager) UnpackAccountRanges(retAccounts []*message.AccountData) ([]common.Hash, [][]byte, error) {
	var (
		hashes   = make([]common.Hash, len(retAccounts))
		accounts = make([][]byte, len(retAccounts))
	)
	for i, acc := range retAccounts {
		val, err := FullAccountRLP(acc.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid account %x: %v", acc.Body, err)
		}
		hashes[i] = common.BytesToHash(acc.Hash)
		accounts[i] = val
	}
	return hashes, accounts, nil
}

// HandleAccountRequestResult handles get account ranges result
func (s *FullStateDownloadManager) HandleAccountRequestResult(task *accountTask,
	retAccounts []*message.AccountData,
	proof [][]byte,
	origin []byte,
	last []byte,
	loopID int,
	streamID sttypes.StreamID) error {

	hashes, accounts, err := s.UnpackAccountRanges(retAccounts)
	if err != nil {
		return err
	}

	size := common.StorageSize(len(hashes) * common.HashLength)
	for _, account := range accounts {
		size += common.StorageSize(len(account))
	}
	for _, node := range proof {
		size += common.StorageSize(len(node))
	}
	utils.Logger().Trace().
		Int("hashes", len(hashes)).
		Int("accounts", len(accounts)).
		Int("proofs", len(proof)).
		Interface("bytes", size).
		Msg("Delivering range of accounts")

	s.lock.Lock()
	defer s.lock.Unlock()

	// Response is valid, but check if peer is signalling that it does not have
	// the requested data. For account range queries that means the state being
	// retrieved was either already pruned remotely, or the peer is not yet
	// synced to our head.
	if len(hashes) == 0 && len(accounts) == 0 && len(proof) == 0 {
		utils.Logger().Debug().
			Interface("root", s.root).
			Msg("Peer rejected account range request")
		s.lock.Unlock()
		return nil
	}
	root := s.root
	s.lock.Unlock()

	// Reconstruct a partial trie from the response and verify it
	keys := make([][]byte, len(hashes))
	for i, key := range hashes {
		keys[i] = common.CopyBytes(key[:])
	}
	nodes := make(ProofList, len(proof))
	for i, node := range proof {
		nodes[i] = node
	}
	cont, err := trie.VerifyRangeProof(root, origin[:], last[:], keys, accounts, nodes.Set())
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("Account range failed proof")
		// Signal this request as failed, and ready for rescheduling
		return err
	}
	accs := make([]*types.StateAccount, len(accounts))
	for i, account := range accounts {
		acc := new(types.StateAccount)
		if err := rlp.DecodeBytes(account, acc); err != nil {
			panic(err) // We created these blobs, we must be able to decode them
		}
		accs[i] = acc
	}

	if err := s.processAccountResponse(task, hashes, accs, cont); err != nil {
		return err
	}

	return nil
}

// processAccountResponse integrates an already validated account range response
// into the account tasks.
func (s *FullStateDownloadManager) processAccountResponse(task *accountTask, // Task which this request is filling
	hashes []common.Hash, // Account hashes in the returned range
	accounts []*types.StateAccount, // Expanded accounts in the returned range
	cont bool, // Whether the account range has a continuation
) error {

	if _, ok := s.tasks.accountTasks[task.id]; ok {
		s.tasks.accountTasks[task.id].res = &accountResponse{
			task:     task,
			hashes:   hashes,
			accounts: accounts,
			cont:     cont,
		}
	}

	// Ensure that the response doesn't overflow into the subsequent task
	last := task.Last.Big()
	for i, hash := range hashes {
		// Mark the range complete if the last is already included.
		// Keep iteration to delete the extra states if exists.
		cmp := hash.Big().Cmp(last)
		if cmp == 0 {
			cont = false
			continue
		}
		if cmp > 0 {
			// Chunk overflown, cut off excess
			hashes = hashes[:i]
			accounts = accounts[:i]
			cont = false // Mark range completed
			break
		}
	}
	// Iterate over all the accounts and assemble which ones need further sub-
	// filling before the entire account range can be persisted.
	task.needCode = make([]bool, len(accounts))
	task.needState = make([]bool, len(accounts))
	task.needHeal = make([]bool, len(accounts))

	task.codeTasks = make(map[common.Hash]struct{})
	task.stateTasks = make(map[common.Hash]common.Hash)

	resumed := make(map[common.Hash]struct{})

	task.pend = 0
	for i, account := range accounts {
		// Check if the account is a contract with an unknown code
		if !bytes.Equal(account.CodeHash, types.EmptyCodeHash.Bytes()) {
			if !rawdb.HasCodeWithPrefix(s.db, common.BytesToHash(account.CodeHash)) {
				task.codeTasks[common.BytesToHash(account.CodeHash)] = struct{}{}
				task.needCode[i] = true
				task.pend++
			}
		}
		// Check if the account is a contract with an unknown storage trie
		if account.Root != types.EmptyRootHash {
			if !rawdb.HasTrieNode(s.db, hashes[i], nil, account.Root, s.scheme) {
				// If there was a previous large state retrieval in progress,
				// don't restart it from scratch. This happens if a sync cycle
				// is interrupted and resumed later. However, *do* update the
				// previous root hash.
				if subtasks, ok := task.SubTasks[hashes[i]]; ok {
					utils.Logger().Debug().Interface("account", hashes[i]).Interface("root", account.Root).Msg("Resuming large storage retrieval")
					for _, subtask := range subtasks {
						subtask.root = account.Root
					}
					task.needHeal[i] = true
					resumed[hashes[i]] = struct{}{}
				} else {
					task.stateTasks[hashes[i]] = account.Root
				}
				task.needState[i] = true
				task.pend++
			}
		}
	}
	// Delete any subtasks that have been aborted but not resumed. This may undo
	// some progress if a new peer gives us less accounts than an old one, but for
	// now we have to live with that.
	for hash := range task.SubTasks {
		if _, ok := resumed[hash]; !ok {
			utils.Logger().Debug().Interface("account", hash).Msg("Aborting suspended storage retrieval")
			delete(task.SubTasks, hash)
		}
	}
	// If the account range contained no contracts, or all have been fully filled
	// beforehand, short circuit storage filling and forward to the next task
	if task.pend == 0 {
		s.forwardAccountTask(task)
		return nil
	}
	// Some accounts are incomplete, leave as is for the storage and contract
	// task assigners to pick up and fill
	return nil
}

// HandleBytecodeRequestResult handles get bytecode result
// it is a callback method to invoke when a batch of contract
// bytes codes are received from a remote peer.
func (s *FullStateDownloadManager) HandleBytecodeRequestResult(task interface{}, // Task which this request is filling
	reqHashes []common.Hash, // Hashes of the bytecode to avoid double hashing
	bytecodes [][]byte, // Actual bytecodes to store into the database (nil = missing)
	loopID int,
	streamID sttypes.StreamID) error {

	s.lock.RLock()
	syncing := !s.snapped
	s.lock.RUnlock()

	if syncing {
		return s.onByteCodes(task.(*accountTask), bytecodes, reqHashes)
	}
	return s.onHealByteCodes(task.(*healTask), reqHashes, bytecodes)
}

// onByteCodes is a callback method to invoke when a batch of contract
// bytes codes are received from a remote peer in the syncing phase.
func (s *FullStateDownloadManager) onByteCodes(task *accountTask, bytecodes [][]byte, reqHashes []common.Hash) error {
	var size common.StorageSize
	for _, code := range bytecodes {
		size += common.StorageSize(len(code))
	}

	utils.Logger().Trace().Int("bytecodes", len(bytecodes)).Interface("bytes", size).Msg("Delivering set of bytecodes")

	s.lock.Lock()
	defer s.lock.Unlock()

	// Response is valid, but check if peer is signalling that it does not have
	// the requested data. For bytecode range queries that means the peer is not
	// yet synced.
	if len(bytecodes) == 0 {
		utils.Logger().Debug().Msg("Peer rejected bytecode request")
		return nil
	}

	// Cross reference the requested bytecodes with the response to find gaps
	// that the serving node is missing
	hasher := sha3.NewLegacyKeccak256().(crypto.KeccakState)
	hash := make([]byte, 32)

	codes := make([][]byte, len(reqHashes))
	for i, j := 0, 0; i < len(bytecodes); i++ {
		// Find the next hash that we've been served, leaving misses with nils
		hasher.Reset()
		hasher.Write(bytecodes[i])
		hasher.Read(hash)

		for j < len(reqHashes) && !bytes.Equal(hash, reqHashes[j][:]) {
			j++
		}
		if j < len(reqHashes) {
			codes[j] = bytecodes[i]
			j++
			continue
		}
		// We've either ran out of hashes, or got unrequested data
		utils.Logger().Warn().Int("count", len(bytecodes)-i).Msg("Unexpected bytecodes")
		// Signal this request as failed, and ready for rescheduling
		return errors.New("unexpected bytecode")
	}
	// Response validated, send it to the scheduler for filling
	if err := s.processBytecodeResponse(task, reqHashes, codes); err != nil {
		return err
	}

	return nil
}

// processBytecodeResponse integrates an already validated bytecode response
// into the account tasks.
func (s *FullStateDownloadManager) processBytecodeResponse(task *accountTask, // Task which this request is filling
	hashes []common.Hash, // Hashes of the bytecode to avoid double hashing
	bytecodes [][]byte, // Actual bytecodes to store into the database (nil = missing)
) error {
	batch := s.db.NewBatch()

	var (
		codes uint64
	)
	for i, hash := range hashes {
		code := bytecodes[i]

		// If the bytecode was not delivered, reschedule it
		if code == nil {
			task.codeTasks[hash] = struct{}{}
			continue
		}
		// Code was delivered, mark it not needed any more
		for j, account := range task.res.accounts {
			if task.needCode[j] && hash == common.BytesToHash(account.CodeHash) {
				task.needCode[j] = false
				task.pend--
			}
		}
		// Push the bytecode into a database batch
		codes++
		rawdb.WriteCode(batch, hash, code)
	}
	bytes := common.StorageSize(batch.ValueSize())
	if err := batch.Write(); err != nil {
		log.Crit("Failed to persist bytecodes", "err", err)
	}
	s.bytecodeSynced += codes
	s.bytecodeBytes += bytes

	utils.Logger().Debug().Interface("count", codes).Float64("bytes", float64(bytes)).Msg("Persisted set of bytecodes")

	// If this delivery completed the last pending task, forward the account task
	// to the next chunk
	if task.pend == 0 {
		s.forwardAccountTask(task)
		return nil
	}
	// Some accounts are still incomplete, leave as is for the storage and contract
	// task assigners to pick up and fill.

	return nil
}

// estimateRemainingSlots tries to determine roughly how many slots are left in
// a contract storage, based on the number of keys and the last hash. This method
// assumes that the hashes are lexicographically ordered and evenly distributed.
func estimateRemainingSlots(hashes int, last common.Hash) (uint64, error) {
	if last == (common.Hash{}) {
		return 0, errors.New("last hash empty")
	}
	space := new(big.Int).Mul(math.MaxBig256, big.NewInt(int64(hashes)))
	space.Div(space, last.Big())
	if !space.IsUint64() {
		// Gigantic address space probably due to too few or malicious slots
		return 0, errors.New("too few slots for estimation")
	}
	return space.Uint64() - uint64(hashes), nil
}

// Unpack retrieves the storage slots from the range packet and returns them in
// a split flat format that's more consistent with the internal data structures.
func (s *FullStateDownloadManager) UnpackStorages(slots [][]*message.StorageData) ([][]common.Hash, [][][]byte) {
	var (
		hashset = make([][]common.Hash, len(slots))
		slotset = make([][][]byte, len(slots))
	)
	for i, slots := range slots {
		hashset[i] = make([]common.Hash, len(slots))
		slotset[i] = make([][]byte, len(slots))
		for j, slot := range slots {
			hashset[i][j] = common.BytesToHash(slot.Hash)
			slotset[i][j] = slot.Body
		}
	}
	return hashset, slotset
}

// HandleStorageRequestResult handles get storages result when ranges of storage slots
// are received from a remote peer.
func (s *FullStateDownloadManager) HandleStorageRequestResult(mainTask *accountTask,
	subTask *storageTask,
	reqAccounts []common.Hash,
	roots []common.Hash,
	origin common.Hash,
	limit common.Hash,
	receivedSlots [][]*message.StorageData,
	proof [][]byte,
	loopID int,
	streamID sttypes.StreamID) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	hashes, slots := s.UnpackStorages(receivedSlots)

	// Gather some trace stats to aid in debugging issues
	var (
		hashCount int
		slotCount int
		size      common.StorageSize
	)
	for _, hashset := range hashes {
		size += common.StorageSize(common.HashLength * len(hashset))
		hashCount += len(hashset)
	}
	for _, slotset := range slots {
		for _, slot := range slotset {
			size += common.StorageSize(len(slot))
		}
		slotCount += len(slotset)
	}
	for _, node := range proof {
		size += common.StorageSize(len(node))
	}

	utils.Logger().Trace().
		Int("accounts", len(hashes)).
		Int("hashes", hashCount).
		Int("slots", slotCount).
		Int("proofs", len(proof)).
		Interface("size", size).
		Msg("Delivering ranges of storage slots")

	s.lock.Lock()
	defer s.lock.Unlock()

	// Reject the response if the hash sets and slot sets don't match, or if the
	// peer sent more data than requested.
	if len(hashes) != len(slots) {
		utils.Logger().Warn().
			Int("hashset", len(hashes)).
			Int("slotset", len(slots)).
			Msg("Hash and slot set size mismatch")
		return errors.New("hash and slot set size mismatch")
	}
	if len(hashes) > len(reqAccounts) {
		utils.Logger().Warn().
			Int("hashset", len(hashes)).
			Int("requested", len(reqAccounts)).
			Msg("Hash set larger than requested")
		return errors.New("hash set larger than requested")
	}
	// Response is valid, but check if peer is signalling that it does not have
	// the requested data. For storage range queries that means the state being
	// retrieved was either already pruned remotely, or the peer is not yet
	// synced to our head.
	if len(hashes) == 0 && len(proof) == 0 {
		utils.Logger().Debug().Msg("Peer rejected storage request")
		return nil
	}

	// Reconstruct the partial tries from the response and verify them
	var cont bool

	// If a proof was attached while the response is empty, it indicates that the
	// requested range specified with 'origin' is empty. Construct an empty state
	// response locally to finalize the range.
	if len(hashes) == 0 && len(proof) > 0 {
		hashes = append(hashes, []common.Hash{})
		slots = append(slots, [][]byte{})
	}
	for i := 0; i < len(hashes); i++ {
		// Convert the keys and proofs into an internal format
		keys := make([][]byte, len(hashes[i]))
		for j, key := range hashes[i] {
			keys[j] = common.CopyBytes(key[:])
		}
		nodes := make(ProofList, 0, len(proof))
		if i == len(hashes)-1 {
			for _, node := range proof {
				nodes = append(nodes, node)
			}
		}
		var err error
		if len(nodes) == 0 {
			// No proof has been attached, the response must cover the entire key
			// space and hash to the origin root.
			_, err = trie.VerifyRangeProof(roots[i], nil, nil, keys, slots[i], nil)
			if err != nil {
				utils.Logger().Warn().Err(err).Msg("Storage slots failed proof")
				return err
			}
		} else {
			// A proof was attached, the response is only partial, check that the
			// returned data is indeed part of the storage trie
			proofdb := nodes.Set()

			cont, err = trie.VerifyRangeProof(roots[i], origin[:], limit[:], keys, slots[i], proofdb)
			if err != nil {
				utils.Logger().Warn().Err(err).Msg("Storage range failed proof")
				return err
			}
		}
	}

	if err := s.processStorageResponse(mainTask, subTask, reqAccounts, roots, hashes, slots, cont); err != nil {
		return err
	}

	return nil
}

// processStorageResponse integrates an already validated storage response
// into the account tasks.
func (s *FullStateDownloadManager) processStorageResponse(mainTask *accountTask, // Task which this response belongs to
	subTask *storageTask, // Task which this response is filling
	accounts []common.Hash, // Account hashes requested, may be only partially filled
	roots []common.Hash, // Storage roots requested, may be only partially filled
	hashes [][]common.Hash, // Storage slot hashes in the returned range
	storageSlots [][][]byte, // Storage slot values in the returned range
	cont bool, // Whether the last storage range has a continuation
) error {
	batch := ethdb.HookedBatch{
		Batch: s.db.NewBatch(),
		OnPut: func(key []byte, value []byte) {
			s.storageBytes += common.StorageSize(len(key) + len(value))
		},
	}
	var (
		slots           int
		oldStorageBytes = s.storageBytes
	)
	// Iterate over all the accounts and reconstruct their storage tries from the
	// delivered slots
	for i, account := range accounts {
		// If the account was not delivered, reschedule it
		if i >= len(hashes) {
			mainTask.stateTasks[account] = roots[i]
			continue
		}
		// State was delivered, if complete mark as not needed any more, otherwise
		// mark the account as needing healing
		for j, hash := range mainTask.res.hashes {
			if account != hash {
				continue
			}
			acc := mainTask.res.accounts[j]

			// If the packet contains multiple contract storage slots, all
			// but the last are surely complete. The last contract may be
			// chunked, so check it's continuation flag.
			if subTask == nil && mainTask.needState[j] && (i < len(hashes)-1 || !cont) {
				mainTask.needState[j] = false
				mainTask.pend--
			}
			// If the last contract was chunked, mark it as needing healing
			// to avoid writing it out to disk prematurely.
			if subTask == nil && !mainTask.needHeal[j] && i == len(hashes)-1 && cont {
				mainTask.needHeal[j] = true
			}
			// If the last contract was chunked, we need to switch to large
			// contract handling mode
			if subTask == nil && i == len(hashes)-1 && cont {
				// If we haven't yet started a large-contract retrieval, create
				// the subtasks for it within the main account task
				if tasks, ok := mainTask.SubTasks[account]; !ok {
					var (
						keys    = hashes[i]
						chunks  = uint64(storageConcurrency)
						lastKey common.Hash
					)
					if len(keys) > 0 {
						lastKey = keys[len(keys)-1]
					}
					// If the number of slots remaining is low, decrease the
					// number of chunks. Somewhere on the order of 10-15K slots
					// fit into a packet of 500KB. A key/slot pair is maximum 64
					// bytes, so pessimistically maxRequestSize/64 = 8K.
					//
					// Chunk so that at least 2 packets are needed to fill a task.
					if estimate, err := estimateRemainingSlots(len(keys), lastKey); err == nil {
						if n := estimate / (2 * (maxRequestSize / 64)); n+1 < chunks {
							chunks = n + 1
						}
						utils.Logger().Debug().
							Int("initiators", len(keys)).
							Interface("tail", lastKey).
							Uint64("remaining", estimate).
							Uint64("chunks", chunks).
							Msg("Chunked large contract")
					} else {
						utils.Logger().Debug().
							Int("initiators", len(keys)).
							Interface("tail", lastKey).
							Uint64("chunks", chunks).
							Msg("Chunked large contract")
					}
					r := newHashRange(lastKey, chunks)

					// Our first task is the one that was just filled by this response.
					batch := ethdb.HookedBatch{
						Batch: s.db.NewBatch(),
						OnPut: func(key []byte, value []byte) {
							s.storageBytes += common.StorageSize(len(key) + len(value))
						},
					}
					ownerAccount := account // local assignment for stacktrie writer closure
					// options := trie.NewStackTrieOptions()
					writeFn := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
						rawdb.WriteTrieNode(batch, ownerAccount, path, hash, blob, s.scheme)
					}
					tasks = append(tasks, &storageTask{
						Next:     common.Hash{},
						Last:     r.End(),
						root:     acc.Root,
						genBatch: batch,
						genTrie:  trie.NewStackTrie(writeFn),
					})
					for r.Next() {
						batch := ethdb.HookedBatch{
							Batch: s.db.NewBatch(),
							OnPut: func(key []byte, value []byte) {
								s.storageBytes += common.StorageSize(len(key) + len(value))
							},
						}
						// options := trie.NewStackTrieOptions()
						writeFn := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
							rawdb.WriteTrieNode(batch, ownerAccount, path, hash, blob, s.scheme)
						}
						tasks = append(tasks, &storageTask{
							Next:     r.Start(),
							Last:     r.End(),
							root:     acc.Root,
							genBatch: batch,
							genTrie:  trie.NewStackTrie(writeFn),
						})
					}
					for _, task := range tasks {
						utils.Logger().Debug().
							Interface("from", task.Next).
							Interface("last", task.Last).
							Interface("root", acc.Root).
							Interface("account", account).
							Msg("Created storage sync task")
					}
					mainTask.SubTasks[account] = tasks

					// Since we've just created the sub-tasks, this response
					// is surely for the first one (zero origin)
					subTask = tasks[0]
				}
			}
			// If we're in large contract delivery mode, forward the subtask
			if subTask != nil {
				// Ensure the response doesn't overflow into the subsequent task
				last := subTask.Last.Big()
				// Find the first overflowing key. While at it, mark res as complete
				// if we find the range to include or pass the 'last'
				index := sort.Search(len(hashes[i]), func(k int) bool {
					cmp := hashes[i][k].Big().Cmp(last)
					if cmp >= 0 {
						cont = false
					}
					return cmp > 0
				})
				if index >= 0 {
					// cut off excess
					hashes[i] = hashes[i][:index]
					storageSlots[i] = storageSlots[i][:index]
				}
				// Forward the relevant storage chunk (even if created just now)
				if cont {
					subTask.Next = incHash(hashes[i][len(hashes[i])-1])
				} else {
					subTask.done = true
				}
			}
		}
		// Iterate over all the complete contracts, reconstruct the trie nodes and
		// push them to disk. If the contract is chunked, the trie nodes will be
		// reconstructed later.
		slots += len(hashes[i])

		if i < len(hashes)-1 || subTask == nil {
			// no need to make local reassignment of account: this closure does not outlive the loop
			// options := trie.NewStackTrieOptions()
			writeFn := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
				rawdb.WriteTrieNode(batch, account, path, hash, blob, s.scheme)
			}
			tr := trie.NewStackTrie(writeFn)
			for j := 0; j < len(hashes[i]); j++ {
				tr.Update(hashes[i][j][:], storageSlots[i][j])
			}
			tr.Commit()
		}
		// Persist the received storage segments. These flat state maybe
		// outdated during the sync, but it can be fixed later during the
		// snapshot generation.
		for j := 0; j < len(hashes[i]); j++ {
			rawdb.WriteStorageSnapshot(batch, account, hashes[i][j], storageSlots[i][j])

			// If we're storing large contracts, generate the trie nodes
			// on the fly to not trash the gluing points
			if i == len(hashes)-1 && subTask != nil {
				subTask.genTrie.Update(hashes[i][j][:], storageSlots[i][j])
			}
		}
	}
	// Large contracts could have generated new trie nodes, flush them to disk
	if subTask != nil {
		if subTask.done {
			root, _ := subTask.genTrie.Commit()
			if root == subTask.root {
				// If the chunk's root is an overflown but full delivery, clear the heal request
				for i, account := range mainTask.res.hashes {
					if account == accounts[len(accounts)-1] {
						mainTask.needHeal[i] = false
					}
				}
			}
		}
		if subTask.genBatch.ValueSize() > ethdb.IdealBatchSize || subTask.done {
			if err := subTask.genBatch.Write(); err != nil {
				log.Error("Failed to persist stack slots", "err", err)
			}
			subTask.genBatch.Reset()
		}
	}
	// Flush anything written just now and update the stats
	if err := batch.Write(); err != nil {
		log.Crit("Failed to persist storage slots", "err", err)
	}
	s.storageSynced += uint64(slots)

	utils.Logger().Debug().
		Int("accounts", len(hashes)).
		Int("slots", slots).
		Interface("bytes", s.storageBytes-oldStorageBytes).
		Msg("Persisted set of storage slots")

	// If this delivery completed the last pending task, forward the account task
	// to the next chunk
	if mainTask.pend == 0 {
		s.forwardAccountTask(mainTask)
		return nil
	}
	// Some accounts are still incomplete, leave as is for the storage and contract
	// task assigners to pick up and fill.

	return nil
}

// HandleTrieNodeHealRequestResult handles get trie nodes heal result when a batch of trie nodes
// are received from a remote peer.
func (s *FullStateDownloadManager) HandleTrieNodeHealRequestResult(task *healTask, // Task which this request is filling
	reqPaths []string,
	reqHashes []common.Hash,
	trienodes [][]byte,
	loopID int,
	streamID sttypes.StreamID) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	var size common.StorageSize
	for _, node := range trienodes {
		size += common.StorageSize(len(node))
	}

	utils.Logger().Trace().
		Int("trienodes", len(trienodes)).
		Interface("bytes", size).
		Msg("Delivering set of healing trienodes")

	// Response is valid, but check if peer is signalling that it does not have
	// the requested data. For bytecode range queries that means the peer is not
	// yet synced.
	if len(trienodes) == 0 {
		utils.Logger().Debug().Msg("Peer rejected trienode heal request")
		return nil
	}

	// Cross reference the requested trienodes with the response to find gaps
	// that the serving node is missing
	var (
		hasher = sha3.NewLegacyKeccak256().(crypto.KeccakState)
		hash   = make([]byte, 32)
		nodes  = make([][]byte, len(reqHashes))
		fills  uint64
	)
	for i, j := 0, 0; i < len(trienodes); i++ {
		// Find the next hash that we've been served, leaving misses with nils
		hasher.Reset()
		hasher.Write(trienodes[i])
		hasher.Read(hash)

		for j < len(reqHashes) && !bytes.Equal(hash, reqHashes[j][:]) {
			j++
		}
		if j < len(reqHashes) {
			nodes[j] = trienodes[i]
			fills++
			j++
			continue
		}
		// We've either ran out of hashes, or got unrequested data
		utils.Logger().Warn().Int("count", len(trienodes)-i).Msg("Unexpected healing trienodes")

		// Signal this request as failed, and ready for rescheduling
		return errors.New("unexpected healing trienode")
	}
	// Response validated, send it to the scheduler for filling
	s.trienodeHealPend.Add(fills)
	defer func() {
		s.trienodeHealPend.Add(^(fills - 1))
	}()

	if err := s.processTrienodeHealResponse(task, reqPaths, reqHashes, nodes); err != nil {
		return err
	}

	return nil
}

// processTrienodeHealResponse integrates an already validated trienode response
// into the healer tasks.
func (s *FullStateDownloadManager) processTrienodeHealResponse(task *healTask, // Task which this request is filling
	paths []string, // Paths of the trie nodes
	hashes []common.Hash, // Hashes of the trie nodes to avoid double hashing
	nodes [][]byte, // Actual trie nodes to store into the database (nil = missing)
) error {
	var (
		start = time.Now()
		fills int
	)
	for i, hash := range hashes {
		node := nodes[i]

		// If the trie node was not delivered, reschedule it
		if node == nil {
			task.trieTasks[paths[i]] = hashes[i]
			continue
		}
		fills++

		// Push the trie node into the state syncer
		s.trienodeHealSynced++
		s.trienodeHealBytes += common.StorageSize(len(node))

		err := s.scheduler.ProcessNode(trie.NodeSyncResult{Path: paths[i], Data: node})
		switch err {
		case nil:
		case trie.ErrAlreadyProcessed:
			s.trienodeHealDups++
		case trie.ErrNotRequested:
			s.trienodeHealNops++
		default:
			utils.Logger().Err(err).Interface("hash", hash).Msg("Invalid trienode processed")
		}
	}
	s.commitHealer(false)

	// Calculate the processing rate of one filled trie node
	rate := float64(fills) / (float64(time.Since(start)) / float64(time.Second))

	// Update the currently measured trienode queueing and processing throughput.
	//
	// The processing rate needs to be updated uniformly independent if we've
	// processed 1x100 trie nodes or 100x1 to keep the rate consistent even in
	// the face of varying network packets. As such, we cannot just measure the
	// time it took to process N trie nodes and update once, we need one update
	// per trie node.
	//
	// Naively, that would be:
	//
	//   for i:=0; i<fills; i++ {
	//     healRate = (1-measurementImpact)*oldRate + measurementImpact*newRate
	//   }
	//
	// Essentially, a recursive expansion of HR = (1-MI)*HR + MI*NR.
	//
	// We can expand that formula for the Nth item as:
	//   HR(N) = (1-MI)^N*OR + (1-MI)^(N-1)*MI*NR + (1-MI)^(N-2)*MI*NR + ... + (1-MI)^0*MI*NR
	//
	// The above is a geometric sequence that can be summed to:
	//   HR(N) = (1-MI)^N*(OR-NR) + NR
	s.trienodeHealRate = gomath.Pow(1-trienodeHealRateMeasurementImpact, float64(fills))*(s.trienodeHealRate-rate) + rate

	pending := s.trienodeHealPend.Load()
	if time.Since(s.trienodeHealThrottled) > time.Second {
		// Periodically adjust the trie node throttler
		if float64(pending) > 2*s.trienodeHealRate {
			s.trienodeHealThrottle *= trienodeHealThrottleIncrease
		} else {
			s.trienodeHealThrottle /= trienodeHealThrottleDecrease
		}
		if s.trienodeHealThrottle > maxTrienodeHealThrottle {
			s.trienodeHealThrottle = maxTrienodeHealThrottle
		} else if s.trienodeHealThrottle < minTrienodeHealThrottle {
			s.trienodeHealThrottle = minTrienodeHealThrottle
		}
		s.trienodeHealThrottled = time.Now()

		utils.Logger().Debug().
			Float64("rate", s.trienodeHealRate).
			Uint64("pending", pending).
			Float64("throttle", s.trienodeHealThrottle).
			Msg("Updated trie node heal throttler")
	}

	return nil
}

// HandleByteCodeHealRequestResult handles get byte codes heal result
func (s *FullStateDownloadManager) HandleByteCodeHealRequestResult(task *healTask, // Task which this request is filling
	hashes []common.Hash, // Hashes of the bytecode to avoid double hashing
	codes [][]byte, // Actual bytecodes to store into the database (nil = missing)
	loopID int,
	streamID sttypes.StreamID) error {

	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.processBytecodeHealResponse(task, hashes, codes); err != nil {
		return err
	}

	return nil
}

// onHealByteCodes is a callback method to invoke when a batch of contract
// bytes codes are received from a remote peer in the healing phase.
func (s *FullStateDownloadManager) onHealByteCodes(task *healTask,
	reqHashes []common.Hash,
	bytecodes [][]byte) error {

	var size common.StorageSize
	for _, code := range bytecodes {
		size += common.StorageSize(len(code))
	}

	utils.Logger().Trace().
		Int("bytecodes", len(bytecodes)).
		Interface("bytes", size).
		Msg("Delivering set of healing bytecodes")

	s.lock.Lock()
	s.lock.Unlock()

	// Response is valid, but check if peer is signalling that it does not have
	// the requested data. For bytecode range queries that means the peer is not
	// yet synced.
	if len(bytecodes) == 0 {
		utils.Logger().Debug().Msg("Peer rejected bytecode heal request")
		return nil
	}

	// Cross reference the requested bytecodes with the response to find gaps
	// that the serving node is missing
	hasher := sha3.NewLegacyKeccak256().(crypto.KeccakState)
	hash := make([]byte, 32)

	codes := make([][]byte, len(reqHashes))
	for i, j := 0, 0; i < len(bytecodes); i++ {
		// Find the next hash that we've been served, leaving misses with nils
		hasher.Reset()
		hasher.Write(bytecodes[i])
		hasher.Read(hash)

		for j < len(reqHashes) && !bytes.Equal(hash, reqHashes[j][:]) {
			j++
		}
		if j < len(reqHashes) {
			codes[j] = bytecodes[i]
			j++
			continue
		}
		// We've either ran out of hashes, or got unrequested data
		utils.Logger().Warn().Int("count", len(bytecodes)-i).Msg("Unexpected healing bytecodes")

		// Signal this request as failed, and ready for rescheduling
		return errors.New("unexpected healing bytecode")
	}

	if err := s.processBytecodeHealResponse(task, reqHashes, codes); err != nil {
		return err
	}

	return nil
}

// processBytecodeHealResponse integrates an already validated bytecode response
// into the healer tasks.
func (s *FullStateDownloadManager) processBytecodeHealResponse(task *healTask, // Task which this request is filling
	hashes []common.Hash, // Hashes of the bytecode to avoid double hashing
	codes [][]byte, // Actual bytecodes to store into the database (nil = missing)
) error {
	for i, hash := range hashes {
		node := codes[i]

		// If the trie node was not delivered, reschedule it
		if node == nil {
			task.codeTasks[hash] = struct{}{}
			continue
		}
		// Push the trie node into the state syncer
		s.bytecodeHealSynced++
		s.bytecodeHealBytes += common.StorageSize(len(node))

		err := s.scheduler.ProcessCode(trie.CodeSyncResult{Hash: hash, Data: node})
		switch err {
		case nil:
		case trie.ErrAlreadyProcessed:
			s.bytecodeHealDups++
		case trie.ErrNotRequested:
			s.bytecodeHealNops++
		default:
			log.Error("Invalid bytecode processed", "hash", hash, "err", err)
		}
	}
	s.commitHealer(false)

	return nil
}

// onHealState is a callback method to invoke when a flat state(account
// or storage slot) is downloaded during the healing stage. The flat states
// can be persisted blindly and can be fixed later in the generation stage.
// Note it's not concurrent safe, please handle the concurrent issue outside.
func (s *FullStateDownloadManager) onHealState(paths [][]byte, value []byte) error {
	if len(paths) == 1 {
		var account types.StateAccount
		if err := rlp.DecodeBytes(value, &account); err != nil {
			return nil // Returning the error here would drop the remote peer
		}
		blob := s.SlimAccountRLP(account)
		rawdb.WriteAccountSnapshot(s.stateWriter, common.BytesToHash(paths[0]), blob)
		s.accountHealed += 1
		s.accountHealedBytes += common.StorageSize(1 + common.HashLength + len(blob))
	}
	if len(paths) == 2 {
		rawdb.WriteStorageSnapshot(s.stateWriter, common.BytesToHash(paths[0]), common.BytesToHash(paths[1]), value)
		s.storageHealed += 1
		s.storageHealedBytes += common.StorageSize(1 + 2*common.HashLength + len(value))
	}
	if s.stateWriter.ValueSize() > ethdb.IdealBatchSize {
		s.stateWriter.Write() // It's fine to ignore the error here
		s.stateWriter.Reset()
	}
	return nil
}
