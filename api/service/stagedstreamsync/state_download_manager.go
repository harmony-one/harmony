package stagedstreamsync

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/utils"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/sha3"
)

// codeTask represents a single byte code download task, containing a set of
// peers already attempted retrieval from to detect stalled syncs and abort.
type codeTask struct {
	attempts map[sttypes.StreamID]int
}

// trieTask represents a single trie node download task, containing a set of
// peers already attempted retrieval from to detect stalled syncs and abort.
type trieTask struct {
	hash     common.Hash
	path     [][]byte
	attempts map[sttypes.StreamID]int
}

type task struct {
	trieTasks map[string]*trieTask      // Set of trie node tasks currently queued for retrieval, indexed by path
	codeTasks map[common.Hash]*codeTask // Set of byte code tasks currently queued for retrieval, indexed by hash
}

func newTask() *task {
	return &task{
		trieTasks: make(map[string]*trieTask),
		codeTasks: make(map[common.Hash]*codeTask),
	}
}

func (t *task) addCodeTask(h common.Hash, ct *codeTask) {
	t.codeTasks[h] = &codeTask{
		attempts: ct.attempts,
	}
}

func (t *task) getCodeTask(h common.Hash) *codeTask {
	if task, ok := t.codeTasks[h]; ok {
		return task
	}
	return nil
}

func (t *task) addNewCodeTask(h common.Hash) {
	t.codeTasks[h] = &codeTask{
		attempts: make(map[sttypes.StreamID]int),
	}
}

func (t *task) deleteCodeTask(hash common.Hash) {
	if _, ok := t.codeTasks[hash]; ok {
		delete(t.codeTasks, hash)
	}
}

func (t *task) deleteCodeTaskAttempts(h common.Hash, stID sttypes.StreamID) {
	if task, ok := t.codeTasks[h]; ok {
		if _, ok := task.attempts[stID]; ok {
			delete(t.codeTasks[h].attempts, stID)
		}
	}
}

func (t *task) addTrieTask(path string, tt *trieTask) {
	t.trieTasks[path] = &trieTask{
		hash:     tt.hash,
		path:     tt.path,
		attempts: tt.attempts,
	}
}

func (t *task) getTrieTask(path string) *trieTask {
	if task, ok := t.trieTasks[path]; ok {
		return task
	}
	return nil
}

func (t *task) addNewTrieTask(hash common.Hash, path string) {
	t.trieTasks[path] = &trieTask{
		hash:     hash,
		path:     trie.NewSyncPath([]byte(path)),
		attempts: make(map[sttypes.StreamID]int),
	}
}

func (t *task) deleteTrieTask(path string) {
	if _, ok := t.trieTasks[path]; ok {
		delete(t.trieTasks, path)
	}
}

func (t *task) deleteTrieTaskAttempts(path string, stID sttypes.StreamID) {
	if task, ok := t.trieTasks[path]; ok {
		if _, ok := task.attempts[stID]; ok {
			delete(t.trieTasks[path].attempts, stID)
		}
	}
}

// StateDownloadManager is the helper structure for get blocks request management
type StateDownloadManager struct {
	bc core.BlockChain
	tx kv.RwTx

	protocol    syncProtocol
	root        common.Hash        // State root currently being synced
	sched       *trie.Sync         // State trie sync scheduler defining the tasks
	keccak      crypto.KeccakState // Keccak256 hasher to verify deliveries with
	concurrency int
	logger      zerolog.Logger
	lock        sync.Mutex

	numUncommitted   int
	bytesUncommitted int

	tasks      *task
	requesting *task
	processing *task
	retries    *task
}

func newStateDownloadManager(tx kv.RwTx,
	bc core.BlockChain,
	concurrency int,
	logger zerolog.Logger) *StateDownloadManager {

	return &StateDownloadManager{
		bc:          bc,
		tx:          tx,
		keccak:      sha3.NewLegacyKeccak256().(crypto.KeccakState),
		concurrency: concurrency,
		logger:      logger,
		tasks:       newTask(),
		requesting:  newTask(),
		processing:  newTask(),
		retries:     newTask(),
	}
}

func (s *StateDownloadManager) setRootHash(root common.Hash) {
	s.root = root
	s.sched = state.NewStateSync(root, s.bc.ChainDb(), nil, rawdb.HashScheme)
}

// fillTasks fills the tasks to send to the remote peer.
func (s *StateDownloadManager) fillTasks(n int) error {
	if fill := n - (len(s.tasks.trieTasks) + len(s.tasks.codeTasks)); fill > 0 {
		paths, hashes, codes := s.sched.Missing(fill)
		for i, path := range paths {
			s.tasks.addNewTrieTask(hashes[i], path)
		}
		for _, hash := range codes {
			s.tasks.addNewCodeTask(hash)
		}
	}
	return nil
}

// getNextBatch returns objects with a maximum of n state download
// tasks to send to the remote peer.
func (s *StateDownloadManager) GetNextBatch() (nodes []common.Hash, paths []string, codes []common.Hash, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	cap := StatesPerRequest

	nodes, paths, codes = s.getBatchFromRetries(cap)
	nItems := len(nodes) + len(codes)
	cap -= nItems

	if cap > 0 {
		// Refill available tasks from the scheduler.
		if s.sched.Pending() == 0 {
			return
		}

		if err = s.commit(false); err != nil {
			return
		}

		if err = s.fillTasks(cap); err != nil {
			return
		}
		newNodes, newPaths, newCodes := s.getBatchFromUnprocessed(cap)
		nodes = append(nodes, newNodes...)
		paths = append(paths, newPaths...)
		codes = append(codes, newCodes...)
	}
	return
}

func (s *StateDownloadManager) commit(force bool) error {
	if !force && s.bytesUncommitted < ethdb.IdealBatchSize {
		return nil
	}
	start := time.Now()
	b := s.bc.ChainDb().NewBatch()
	if err := s.sched.Commit(b); err != nil {
		return err
	}
	if err := b.Write(); err != nil {
		return fmt.Errorf("DB write error: %v", err)
	}
	s.updateStats(s.numUncommitted, 0, 0, time.Since(start))
	s.numUncommitted = 0
	s.bytesUncommitted = 0
	return nil
}

// updateStats bumps the various state sync progress counters and displays a log
// message for the user to see.
func (s *StateDownloadManager) updateStats(written, duplicate, unexpected int, duration time.Duration) {
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
func (s *StateDownloadManager) getBatchFromUnprocessed(n int) (nodes []common.Hash, paths []string, codes []common.Hash) {
	// over trie nodes as those can be written to disk and forgotten about.
	nodes = make([]common.Hash, 0, n)
	paths = make([]string, 0, n)
	codes = make([]common.Hash, 0, n)

	for hash, t := range s.tasks.codeTasks {
		// Stop when we've gathered enough requests
		if len(nodes)+len(codes) == n {
			break
		}
		codes = append(codes, hash)
		s.requesting.addCodeTask(hash, t)
		s.tasks.deleteCodeTask(hash)
	}
	for path, t := range s.tasks.trieTasks {
		// Stop when we've gathered enough requests
		if len(nodes)+len(codes) == n {
			break
		}
		nodes = append(nodes, t.hash)
		paths = append(paths, path)
		s.requesting.addTrieTask(path, t)
		s.tasks.deleteTrieTask(path)
	}
	return nodes, paths, codes
}

// getBatchFromRetries get the block number batch to be requested from retries.
func (s *StateDownloadManager) getBatchFromRetries(n int) ([]common.Hash, []string, []common.Hash) {
	// over trie nodes as those can be written to disk and forgotten about.
	nodes := make([]common.Hash, 0, n)
	paths := make([]string, 0, n)
	codes := make([]common.Hash, 0, n)

	for hash, t := range s.retries.codeTasks {
		// Stop when we've gathered enough requests
		if len(nodes)+len(codes) == n {
			break
		}
		codes = append(codes, hash)
		s.requesting.addCodeTask(hash, t)
		s.retries.deleteCodeTask(hash)
	}
	for path, t := range s.retries.trieTasks {
		// Stop when we've gathered enough requests
		if len(nodes)+len(codes) == n {
			break
		}
		nodes = append(nodes, t.hash)
		paths = append(paths, path)
		s.requesting.addTrieTask(path, t)
		s.retries.deleteTrieTask(path)
	}
	return nodes, paths, codes
}

// HandleRequestError handles the error result
func (s *StateDownloadManager) HandleRequestError(codeHashes []common.Hash, triePaths []string, streamID sttypes.StreamID, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// add requested code hashes to retries
	for _, h := range codeHashes {
		task := s.requesting.getCodeTask(h)
		s.retries.addCodeTask(h, task)
		s.requesting.deleteCodeTask(h)
	}

	// add requested trie paths to retries
	for _, path := range triePaths {
		task := s.requesting.getTrieTask(path)
		s.retries.addTrieTask(path, task)
		s.requesting.deleteTrieTask(path)
	}
}

// HandleRequestResult handles get trie paths and code hashes result
func (s *StateDownloadManager) HandleRequestResult(codeHashes []common.Hash, triePaths []string, response [][]byte, loopID int, streamID sttypes.StreamID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Collect processing stats and update progress if valid data was received
	duplicate, unexpected, successful := 0, 0, 0

	for _, blob := range response {
		hash, err := s.processNodeData(codeHashes, triePaths, blob)
		switch err {
		case nil:
			s.numUncommitted++
			s.bytesUncommitted += len(blob)
			successful++
		case trie.ErrNotRequested:
			unexpected++
		case trie.ErrAlreadyProcessed:
			duplicate++
		default:
			return fmt.Errorf("invalid state node %s: %v", hash.TerminalString(), err)
		}
	}

	for _, path := range triePaths {
		task := s.requesting.getTrieTask(path)
		if task == nil {
			// it is already removed from requesting
			// either it has been completed and deleted by processNodeData or it does not exist
			continue
		}
		// If the node did deliver something, missing items may be due to a protocol
		// limit or a previous timeout + delayed delivery. Both cases should permit
		// the node to retry the missing items (to avoid single-peer stalls).
		if len(response) > 0 { //TODO: if timeout also do same
			s.requesting.deleteTrieTaskAttempts(path, streamID)
		} else if task.attempts[streamID] >= MaxTriesToFetchNodeData {
			// If we've requested the node too many times already, it may be a malicious
			// sync where nobody has the right data. Abort.
			return fmt.Errorf("trie node %s failed with peer %s (%d tries)", task.hash.TerminalString(), streamID, task.attempts[streamID])
		}
		// Missing item, place into the retry queue.
		s.retries.addTrieTask(path, task)
		s.requesting.deleteTrieTask(path)
	}

	for _, hash := range codeHashes {
		task := s.requesting.getCodeTask(hash)
		if task == nil {
			// it is already removed from requesting
			// either it has been completed and deleted by processNodeData or it does not exist
			continue
		}
		// If the node did deliver something, missing items may be due to a protocol
		// limit or a previous timeout + delayed delivery. Both cases should permit
		// the node to retry the missing items (to avoid single-peer stalls).
		if len(response) > 0 { //TODO: if timeout also do same
			s.requesting.deleteCodeTaskAttempts(hash, streamID) //TODO: do we need delete attempts???
		} else if task.attempts[streamID] >= MaxTriesToFetchNodeData {
			// If we've requested the node too many times already, it may be a malicious
			// sync where nobody has the right data. Abort.
			return fmt.Errorf("byte code %s failed with peer %s (%d tries)", hash.TerminalString(), streamID, task.attempts[streamID])
		}
		// Missing item, place into the retry queue.
		s.retries.addCodeTask(hash, task)
		s.requesting.deleteCodeTask(hash)
	}

	return nil
}

// processNodeData tries to inject a trie node data blob delivered from a remote
// peer into the state trie, returning whether anything useful was written or any
// error occurred.
//
// If multiple requests correspond to the same hash, this method will inject the
// blob as a result for the first one only, leaving the remaining duplicates to
// be fetched again.
func (s *StateDownloadManager) processNodeData(codeHashes []common.Hash, triePaths []string, responseData []byte) (common.Hash, error) {
	var hash common.Hash
	s.keccak.Reset()
	s.keccak.Write(responseData)
	s.keccak.Read(hash[:])

	//TODO: remove from requesting
	if _, present := s.requesting.codeTasks[hash]; present {
		err := s.sched.ProcessCode(trie.CodeSyncResult{
			Hash: hash,
			Data: responseData,
		})
		s.requesting.deleteCodeTask(hash)
		return hash, err
	}
	for _, path := range triePaths {
		task := s.requesting.getTrieTask(path)
		if task == nil {
			// this shouldn't happen while the path is given from triPaths and triPaths
			// are given from requesting queue
			continue
		}
		if task.hash == hash {
			err := s.sched.ProcessNode(trie.NodeSyncResult{
				Path: path,
				Data: responseData,
			})
			s.requesting.deleteTrieTask(path)
			return hash, err
		}
	}
	return common.Hash{}, trie.ErrNotRequested
}
