package stagedstreamsync

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/sha3"
)

// trieTask represents a single trie node download task, containing a set of
// peers already attempted retrieval from to detect stalled syncs and abort.
type trieTask struct {
	hash     common.Hash
	path     [][]byte
	attempts map[string]struct{}
}

// codeTask represents a single byte code download task, containing a set of
// peers already attempted retrieval from to detect stalled syncs and abort.
type codeTask struct {
	attempts map[string]struct{}
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
	return t.codeTasks[h]
}

func (t *task) addNewCodeTask(h common.Hash) {
	t.codeTasks[h] = &codeTask{
		attempts: make(map[string]struct{}),
	}
}

func (t *task) deleteCodeTask(hash common.Hash) {
	delete(t.codeTasks, hash)
}

func (t *task) addTrieTask(hash common.Hash, path string) {
	t.trieTasks[path] = &trieTask{
		hash:     hash,
		path:     trie.NewSyncPath([]byte(path)),
		attempts: make(map[string]struct{}),
	}
}

func (t *task) setTrieTask(path string, tt *trieTask) {
	t.trieTasks[path] = &trieTask{
		hash:     tt.hash,
		path:     tt.path,
		attempts: tt.attempts,
	}
}

func (t *task) getTrieTask(path string) *trieTask {
	return t.trieTasks[path]
}

func (t *task) deleteTrieTask(path string) {
	delete(t.trieTasks, path)
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

	tasks      *task
	requesting *task
	processing *task
	retries    *task
}

func newStateDownloadManager(tx kv.RwTx,
	bc core.BlockChain,
	root common.Hash,
	concurrency int,
	logger zerolog.Logger) *StateDownloadManager {

	return &StateDownloadManager{
		bc:          bc,
		tx:          tx,
		root:        root,
		sched:       state.NewStateSync(root, bc.ChainDb(), nil, rawdb.HashScheme),
		keccak:      sha3.NewLegacyKeccak256().(crypto.KeccakState),
		concurrency: concurrency,
		logger:      logger,
		tasks:       newTask(),
		requesting:  newTask(),
		processing:  newTask(),
		retries:     newTask(),
	}
}

