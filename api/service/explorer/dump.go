package explorer

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

type DumpHelper interface {
	Dump(block *types.Block)
	UseStorage(s *Storage)
}

type ExplorerDumpHelper struct {
	storage       *Storage
	pendingBlockC chan *types.Block
	closeC        chan struct{}
}

type TempResult struct {
	block *types.Block
	addrs map[string][]byte
}

func NewExplorerDumpHelper() *ExplorerDumpHelper {
	return &ExplorerDumpHelper{
		storage:       nil,
		pendingBlockC: make(chan *types.Block),
		closeC:        make(chan struct{}),
	}
}

func (helper *ExplorerDumpHelper) Start() {
	go helper.run()
}

func (helper *ExplorerDumpHelper) Stop() {
	close(helper.closeC)
}

func (helper *ExplorerDumpHelper) UseStorage(s *Storage) {
	helper.storage = s
}

func (helper *ExplorerDumpHelper) Dump(block *types.Block) {
	helper.pendingBlockC <- block
}

func (helper *ExplorerDumpHelper) run() {
	computeC := make(chan *types.Block, 1)
	sendToCompute := func(b *types.Block) {
		select {
		case computeC <- b:
		default:
		}
	}
	commitC := make(chan TempResult, 1)
	sendToCommit := func(res TempResult) {
		select {
		case commitC <- res:
		default:
		}
	}
	for {
		select {
		case _, ok := <-helper.closeC:
			if !ok {
				return
			}

		case block := <-helper.pendingBlockC:
			sendToCompute(block)

		case block := <-computeC:
			go func(b *types.Block) {
				result := helper.computeAccountsTransactionsMapForBlock(b)
				if len(result) > 0 {
					var temp *TempResult = new(TempResult)
					temp.block = b
					temp.addrs = result
					sendToCommit(*temp)
				}
			}(block)

		case res := <-commitC:
			go func(res TempResult) {
				err := helper.updateTxRecordsToStorage(res.addrs)
				if err != nil {
					utils.Logger().Error().Err(err).Msg("cannot dump address")
				} else {
					blockCheckpoint := GetCheckpointKey(res.block.Header().Number())
					// save checkpoint of block dumped
					storage.GetDB().Put([]byte(blockCheckpoint), []byte{}, nil)
				}

			}(res)

		}

	}
}

func (helper *ExplorerDumpHelper) computeAccountsTransactionsMapForBlock(block *types.Block) map[string][]byte {
	var addresses map[string][]byte
	addresses = make(map[string][]byte)
	// Skip dump for redundant blocks with lower block number than the checkpoint block number
	blockCheckpoint := GetCheckpointKey(block.Header().Number())
	if _, err := helper.storage.GetDB().Get([]byte(blockCheckpoint), nil); err == nil {
		return addresses
	}

	if block == nil {
		return addresses
	}

	acntsTxns, acntsStakingTxns := computeAccountsTransactionsMapForBlock(block)

	for address, txRecords := range acntsTxns {
		addr, bytes := helper.convertAddressTxRecordsToRLP(address, txRecords, false /* isStaking */)
		if len(addr) > 0 {
			addresses[addr] = bytes[:]
		}
	}
	for address, txRecords := range acntsStakingTxns {
		addr, bytes := helper.convertAddressTxRecordsToRLP(address, txRecords, true /* isStaking */)
		if len(addr) > 0 {
			addresses[addr] = bytes[:]
		}
	}

	return addresses

}

func (helper *ExplorerDumpHelper) convertAddressTxRecordsToRLP(addr string, txRecords TxRecords, isStaking bool) (string, []byte) {
	var address Address
	key := GetAddressKey(addr)

	if data, err := helper.storage.GetDB().Get([]byte(key), nil); err == nil {
		if err = rlp.DecodeBytes(data, &address); err != nil {
			utils.Logger().Error().
				Bool("isStaking", isStaking).Err(err).Msg("Failed due to error")
		}
	}

	address.ID = addr
	if isStaking {
		address.StakingTXs = append(address.StakingTXs, txRecords...)
	} else {
		address.TXs = append(address.TXs, txRecords...)
	}
	encoded, err := rlp.EncodeToBytes(address)
	if err != nil {
		addr = ""
		utils.Logger().Error().
			Bool("isStaking", isStaking).Err(err).Msg("cannot encode address")
	}
	return addr, encoded
}

func (helper *ExplorerDumpHelper) updateTxRecordsToStorage(txResults map[string][]byte) error {
	for address, encoded := range txResults {
		key := GetAddressKey(address)
		err := helper.storage.GetDB().Put([]byte(key), encoded, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
