package explorer

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/core"
	core2 "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/rs/zerolog"
)

const (
	numWorker = 5
)

type storage struct {
	db database
	bc *core.BlockChain

	// TODO: optimize this with priority queue
	tm      *taskManager
	resultC chan batch

	available *abool.AtomicBool
	closeC    chan struct{}
	log       zerolog.Logger
}

func newStorage(bc *core.BlockChain, dbPath string) (*storage, error) {
	utils.Logger().Info().Msg("explorer storage folder: " + dbPath)
	db, err := newLvlDB(dbPath)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to create new database")
		return nil, err
	}
	return &storage{
		db:        db,
		bc:        bc,
		tm:        newTaskManager(),
		resultC:   make(chan batch, numWorker),
		available: abool.New(),
		closeC:    make(chan struct{}),
		log:       utils.Logger().With().Str("module", "explorer storage").Logger(),
	}, nil
}

func (s *storage) Start() {
	go s.run()
}

func (s *storage) Stop() {
	close(s.closeC)
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
		case btc := <-s.resultC:
			if err := btc.Write(); err != nil {
				s.log.Error().Err(err).Msg("explorer db failed to write")
			}

		case <-s.closeC:
			return
		}
	}
}

func (s *storage) makeWorkersAndStart() {
	workers := make([]*blockComputer, 0, numWorker)
	for i := 0; i != numWorker; i++ {
		workers = append(workers, &blockComputer{
			tm:      s.tm,
			db:      s.db,
			bc:      s.bc,
			resultC: s.resultC,
			closeC:  s.closeC,
			log:     s.log.With().Int("worker", i).Logger(),
		})
	}
	for _, worker := range workers {
		go worker.loop()
	}
}

type taskManager struct {
	blocksHP []*types.Block // blocks with high priorities
	blocksLP []*types.Block // blocks with low priorities
	lock     sync.Mutex

	C chan struct{}
}

func newTaskManager() *taskManager {
	return &taskManager{
		C: make(chan struct{}, numWorker),
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

func (tm *taskManager) AddCatchupTask(b *types.Block) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	tm.blocksLP = append(tm.blocksLP, b)
	select {
	case tm.C <- struct{}{}:
	default:
	}
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

type blockComputer struct {
	tm      *taskManager
	db      database
	bc      blockChainTxIndexer
	resultC chan batch
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
				case bc.resultC <- res:
				case <-bc.closeC:
					return
				}
			}

		case <-bc.closeC:
			return
		}
	}
}

func (bc *blockComputer) computeBlock(b *types.Block) (batch, error) {
	is, err := isBlockComputedInDB(bc.db, b.NumberU64())
	if err != nil {
		return nil, err
	}
	if is {
		return nil, nil
	}
	btc := bc.db.NewBatch()

	for _, tx := range b.Transactions() {
		bc.computeNormalTx(btc, b, tx)
	}
	for _, stk := range b.StakingTransactions() {
		bc.computeStakingTx(btc, b, stk)
	}
	_ = writeCheckpoint(btc, b.NumberU64())
	return btc, nil
}

func (bc *blockComputer) computeNormalTx(btc batch, b *types.Block, tx *types.Transaction) {
	ethFrom, _ := tx.SenderAddress()
	ethTo := *tx.To()
	from := ethToOneAddress(ethFrom)
	to := ethToOneAddress(ethTo)

	_ = writeAddressEntry(btc, from)
	_ = writeAddressEntry(btc, to)

	_, bn, index := bc.bc.ReadTxLookupEntry(tx.HashByType())
	_ = writeNormalTxnIndex(btc, normalTxnIndex{
		addr:        from,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     tx.HashByType(),
	}, txSent)
	_ = writeNormalTxnIndex(btc, normalTxnIndex{
		addr:        to,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     tx.HashByType(),
	}, txReceived)

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
	_ = writeNormalTxnIndex(btc, normalTxnIndex{
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
	_ = writeNormalTxnIndex(btc, normalTxnIndex{
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
