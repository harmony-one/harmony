package explorer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	"github.com/harmony-one/harmony/internal/tikv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/core"
	core2 "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy/tracers"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/rs/zerolog"
)

const (
	numWorker        = 8
	changedSaveCount = 100
)

// ErrExplorerNotReady is the error when querying explorer db data when
// explorer db is doing migration and unavailable
var ErrExplorerNotReady = errors.New("explorer db not ready")

type Bitmap interface {
	Clone() *roaring64.Bitmap
	MarshalBinary() ([]byte, error)
	CheckedAdd(x uint64) bool
	Contains(x uint64) bool
}

type ThreadSafeBitmap struct {
	b  Bitmap
	mu sync.Mutex
}

func (a *ThreadSafeBitmap) Clone() *roaring64.Bitmap {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.b.Clone()
}

func (a *ThreadSafeBitmap) CheckedAdd(x uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.b.CheckedAdd(x)
}

func (a *ThreadSafeBitmap) Contains(x uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.b.Contains(x)
}

func (a *ThreadSafeBitmap) MarshalBinary() ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.b.MarshalBinary()
}

func NewThreadSafeBitmap(bitmap Bitmap) Bitmap {
	return &ThreadSafeBitmap{
		b:  bitmap,
		mu: sync.Mutex{},
	}
}

type (
	storage struct {
		db database
		bc core.BlockChain
		rb Bitmap //*roaring64.Bitmap

		// TODO: optimize this with priority queue
		tm      *taskManager
		resultC chan blockResult
		resultT chan *traceResult

		available *abool.AtomicBool
		closeC    chan struct{}
		log       zerolog.Logger
	}

	blockResult struct {
		btc batch
		bn  uint64
	}

	traceResult struct {
		btc  batch
		data *tracers.TraceBlockStorage
	}
)

func newExplorerDB(hc *harmonyconfig.HarmonyConfig, dbPath string) (database, error) {
	if hc.General.RunElasticMode {
		// init the storage using tikv
		dbPath = fmt.Sprintf("explorer_tikv_%d", hc.General.ShardID)
		readOnly := hc.TiKV.Role == tikv.RoleReader
		utils.Logger().Info().Msg("explorer storage in tikv: " + dbPath)
		return newExplorerTiKv(hc.TiKV.PDAddr, dbPath, readOnly)
	} else {
		// or leveldb
		utils.Logger().Info().Msg("explorer storage folder: " + dbPath)
		return newExplorerLvlDB(dbPath)
	}
}

func newStorage(hc *harmonyconfig.HarmonyConfig, bc core.BlockChain, dbPath string) (*storage, error) {
	db, err := newExplorerDB(hc, dbPath)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new explorer database")
		return nil, err
	}

	// load checkpoint roaring bitmap from storage
	// roaring bitmap is a very high compression bitmap, in our scene, 1 million blocks use almost 1kb storage
	bitmap, err := readCheckpointBitmap(db)
	if err != nil {
		return nil, err
	}

	safeBitmap := NewThreadSafeBitmap(bitmap)
	return &storage{
		db:        db,
		bc:        bc,
		rb:        safeBitmap,
		tm:        newTaskManager(safeBitmap),
		resultC:   make(chan blockResult, numWorker),
		resultT:   make(chan *traceResult, numWorker),
		available: abool.New(),
		closeC:    make(chan struct{}),
		log:       utils.Logger().With().Str("module", "explorer storage").Logger(),
	}, nil
}

func (s *storage) Start() {
	go s.run()
}

func (s *storage) Close() {
	_ = writeCheckpointBitmap(s.db, s.rb)
	close(s.closeC)
}

func (s *storage) DumpTraceResult(data *tracers.TraceBlockStorage) {
	s.tm.AddNewTraceTask(data)
}

func (s *storage) DumpNewBlock(b *types.Block) {
	s.tm.AddNewTask(b)
}

func (s *storage) DumpCatchupBlock(b *types.Block) {
	for s.tm.HasPendingTasks() {
		time.Sleep(100 * time.Millisecond)
	}
	s.tm.AddCatchupTask(b)
}

func (s *storage) GetAddresses(size int, startAddress oneAddress) ([]string, error) {
	if !s.available.IsSet() {
		return nil, ErrExplorerNotReady
	}
	addrs, err := getAddressesInRange(s.db, startAddress, size)
	if err != nil {
		return nil, err
	}
	display := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		display = append(display, string(addr))
	}
	return display, nil
}

func (s *storage) GetNormalTxsByAddress(addr string) ([]common.Hash, []TxType, error) {
	if !s.available.IsSet() {
		return nil, nil, ErrExplorerNotReady
	}
	return getNormalTxnHashesByAccount(s.db, oneAddress(addr))
}

func (s *storage) GetStakingTxsByAddress(addr string) ([]common.Hash, []TxType, error) {
	if !s.available.IsSet() {
		return nil, nil, ErrExplorerNotReady
	}
	return getStakingTxnHashesByAccount(s.db, oneAddress(addr))
}

func (s *storage) GetTraceResultByHash(hash common.Hash) (json.RawMessage, error) {
	if !s.available.IsSet() {
		return nil, ErrExplorerNotReady
	}
	traceStorage := &tracers.TraceBlockStorage{
		Hash: hash,
	}
	err := traceStorage.FromDB(func(key []byte) ([]byte, error) {
		return getTraceResult(s.db, key)
	})
	if err != nil {
		return nil, err
	}
	return traceStorage.ToJson()
}

func (s *storage) run() {
	if is, err := isVersionV100(s.db); !is || err != nil {
		s.available.UnSet()
		err := s.migrateToV100()
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to migrate explorer DB!")
			fmt.Println("Failed to migrate explorer DB:", err)
			os.Exit(1)
		}
	}
	s.available.Set()
	go s.loop()
}

func (s *storage) loop() {
	s.makeWorkersAndStart()
	for {
		select {
		case res := <-s.resultC:
			s.log.Info().Uint64("block number", res.bn).Msg("writing explorer DB")
			if err := res.btc.Write(); err != nil {
				s.log.Error().Err(err).Msg("explorer db failed to write")
			}
		case res := <-s.resultT:
			s.log.Info().Str("block hash", res.data.Hash.Hex()).Msg("writing trace into explorer DB")
			if err := res.btc.Write(); err != nil {
				s.log.Error().Err(err).Msg("explorer db failed to write trace data")
			}

		case <-s.closeC:
			return
		}
	}
}

type taskManager struct {
	blocksHP []*types.Block // blocks with high priorities
	blocksLP []*types.Block // blocks with low priorities
	lock     sync.Mutex

	rb             Bitmap
	rbChangedCount int

	C chan struct{}
	T chan *traceResult
}

func newTaskManager(bitmap Bitmap) *taskManager {
	return &taskManager{
		rb: bitmap,
		C:  make(chan struct{}, numWorker),
		T:  make(chan *traceResult, numWorker),
	}
}

func (tm *taskManager) AddNewTask(b *types.Block) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	tm.blocksHP = append(tm.blocksHP, b)
	select {
	case tm.C <- struct{}{}:
	default:
	}
}

func (tm *taskManager) AddNewTraceTask(data *tracers.TraceBlockStorage) {
	tm.T <- &traceResult{
		data: data,
	}
}

func (tm *taskManager) AddCatchupTask(b *types.Block) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	tm.blocksLP = append(tm.blocksLP, b)
	select {
	case tm.C <- struct{}{}:
	default:
	}
}

func (tm *taskManager) HasPendingTasks() bool {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	return len(tm.blocksLP) != 0 || len(tm.blocksHP) != 0
}

func (tm *taskManager) PullTask() *types.Block {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if len(tm.blocksHP) != 0 {
		b := tm.blocksHP[0]
		tm.blocksHP = tm.blocksHP[1:]
		return b
	}
	if len(tm.blocksLP) != 0 {
		b := tm.blocksLP[0]
		tm.blocksLP = tm.blocksLP[1:]
		return b
	}
	return nil
}

// markBlockDone mark block processed done when explorer computed one block
func (tm *taskManager) markBlockDone(btc batch, blockNum uint64) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if tm.rb.CheckedAdd(blockNum) {
		tm.rbChangedCount++

		// every 100 change write once
		if tm.rbChangedCount == changedSaveCount {
			tm.rbChangedCount = 0
			_ = writeCheckpointBitmap(btc, tm.rb)
		}
	}
}

// markBlockDone check block is processed done
func (tm *taskManager) hasBlockDone(blockNum uint64) bool {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	return tm.rb.Contains(blockNum)
}

func (s *storage) makeWorkersAndStart() {
	workers := make([]*blockComputer, 0, numWorker)
	for i := 0; i != numWorker; i++ {
		workers = append(workers, &blockComputer{
			tm:      s.tm,
			db:      s.db,
			bc:      s.bc,
			resultC: s.resultC,
			resultT: s.resultT,
			closeC:  s.closeC,
			log:     s.log.With().Int("worker", i).Logger(),
		})
	}
	for _, worker := range workers {
		go worker.loop()
	}
}

type blockComputer struct {
	tm      *taskManager
	db      database
	bc      blockChainTxIndexer
	resultC chan blockResult
	resultT chan *traceResult
	closeC  chan struct{}
	log     zerolog.Logger
}

func (bc *blockComputer) loop() {
LOOP:
	for {
		select {
		case <-bc.tm.C:
			for {
				b := bc.tm.PullTask()
				if b == nil {
					goto LOOP
				}
				res, err := bc.computeBlock(b)
				if err != nil {
					bc.log.Error().Err(err).Str("hash", b.Hash().String()).
						Uint64("number", b.NumberU64()).
						Msg("explorer failed to handle block")
					continue
				}
				if res == nil {
					continue
				}
				select {
				case bc.resultC <- *res:
				case <-bc.closeC:
					return
				}
			}
		case traceResult := <-bc.tm.T:
			if exist, err := isTraceResultInDB(bc.db, traceResult.data.KeyDB()); exist || err != nil {
				continue
			}
			traceResult.btc = bc.db.NewBatch()
			traceResult.data.ToDB(func(key, value []byte) {
				if exist, err := isTraceResultInDB(bc.db, key); !exist && err == nil {
					_ = writeTraceResult(traceResult.btc, key, value)
				}
			})
			select {
			case bc.resultT <- traceResult:
			case <-bc.closeC:
				return
			}
		case <-bc.closeC:
			return
		}
	}
}

func (bc *blockComputer) computeBlock(b *types.Block) (*blockResult, error) {
	if bc.tm.hasBlockDone(b.NumberU64()) {
		return nil, nil
	}
	btc := bc.db.NewBatch()

	for _, tx := range b.Transactions() {
		bc.computeNormalTx(btc, b, tx)
	}
	for _, stk := range b.StakingTransactions() {
		bc.computeStakingTx(btc, b, stk)
	}
	bc.tm.markBlockDone(btc, b.NumberU64())
	return &blockResult{
		btc: btc,
		bn:  b.NumberU64(),
	}, nil
}

func (bc *blockComputer) computeNormalTx(btc batch, b *types.Block, tx *types.Transaction) {
	ethFrom, _ := tx.SenderAddress()
	from := ethToOneAddress(ethFrom)

	_ = writeAddressEntry(btc, from)

	_, bn, index := bc.bc.ReadTxLookupEntry(tx.HashByType())
	_ = writeNormalTxnIndex(btc, normalTxnIndex{
		addr:        from,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     tx.HashByType(),
	}, txSent)

	ethTo := tx.To()
	if ethTo != nil { // Skip for contract creation
		to := ethToOneAddress(*ethTo)
		_ = writeAddressEntry(btc, to)
		_ = writeNormalTxnIndex(btc, normalTxnIndex{
			addr:        to,
			blockNumber: bn,
			txnIndex:    index,
			txnHash:     tx.HashByType(),
		}, txReceived)
	}
	// Not very sure how this 1000 come from. But it is the logic before migration
	t := b.Time().Uint64() * 1000
	tr := &TxRecord{tx.HashByType(), time.Unix(int64(t), 0)}
	_ = writeTxn(btc, tx.HashByType(), tr)
}

func (bc *blockComputer) computeStakingTx(btc batch, b *types.Block, tx *staking.StakingTransaction) {
	ethFrom, _ := tx.SenderAddress()
	from := ethToOneAddress(ethFrom)
	_ = writeAddressEntry(btc, from)
	_, bn, index := bc.bc.ReadTxLookupEntry(tx.Hash())
	_ = writeStakingTxnIndex(btc, stakingTxnIndex{
		addr:        from,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     tx.Hash(),
	}, txSent)
	t := b.Time().Uint64() * 1000
	tr := &TxRecord{tx.Hash(), time.Unix(int64(t), 0)}
	_ = writeTxn(btc, tx.Hash(), tr)

	ethTo, _ := toFromStakingTx(tx, b)
	if ethTo == (common.Address{}) {
		return
	}
	to := ethToOneAddress(ethTo)
	_ = writeAddressEntry(btc, to)
	_ = writeStakingTxnIndex(btc, stakingTxnIndex{
		addr:        to,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     tx.Hash(),
	}, txReceived)
}

func ethToOneAddress(ethAddr common.Address) oneAddress {
	raw, _ := common2.AddressToBech32(ethAddr)
	return oneAddress(raw)
}

func toFromStakingTx(tx *staking.StakingTransaction, addressBlock *types.Block) (common.Address, error) {
	msg, err := core2.StakingToMessage(tx, addressBlock.Header().Number())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
		return common.Address{}, err
	}

	switch tx.StakingType() {
	case staking.DirectiveDelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return common.Address{}, err
		}
		if _, ok := stkMsg.(*staking.Delegate); !ok {
			return common.Address{}, core2.ErrInvalidMsgForStakingDirective
		}
		delegateMsg := stkMsg.(*staking.Delegate)
		if !bytes.Equal(msg.From().Bytes()[:], delegateMsg.DelegatorAddress.Bytes()[:]) {
			return common.Address{}, core2.ErrInvalidSender
		}
		return delegateMsg.ValidatorAddress, nil

	case staking.DirectiveUndelegate:
		stkMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return common.Address{}, err
		}
		if _, ok := stkMsg.(*staking.Undelegate); !ok {
			return common.Address{}, core2.ErrInvalidMsgForStakingDirective
		}
		undelegateMsg := stkMsg.(*staking.Undelegate)
		if !bytes.Equal(msg.From().Bytes()[:], undelegateMsg.DelegatorAddress.Bytes()[:]) {
			return common.Address{}, core2.ErrInvalidSender
		}
		return undelegateMsg.ValidatorAddress, nil
	default:
		return common.Address{}, nil
	}
}
