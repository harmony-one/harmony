package explorer

import (
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/abool"
	"github.com/rs/zerolog"
)

func TestMigrationToV100(t *testing.T) {
	t.Skip("skipping migration with high disk io")
	fac := &migrationDBFactory{t: t}
	db := fac.makeDB(numAddresses)

	m := &migrationV100{
		db:                db,
		bc:                &migrationBlockChain{},
		btc:               db.NewBatch(),
		isMigrateFinished: abool.New(),
		log:               zerolog.New(os.Stdout),
		finishedC:         make(chan struct{}),
	}
	err := m.do()
	if err != nil {
		t.Fatal(err)
	}
}

const numAddresses = 3000

type migrationDBFactory struct {
	curAddrIndex int
	curTxIndex   int
	t            *testing.T
}

func (f *migrationDBFactory) makeDB(numAddr int) database {
	db := newTestLevelDB(f.t, 0)
	for i := 0; i != numAddr; i++ {
		addr := f.newAddress()
		addrInfo := &Address{ID: string(addr)}
		for txI := 0; txI != i/5; txI++ {
			addrInfo.TXs = append(addrInfo.TXs, f.newLegTx())
		}
		for stkI := 0; stkI != i/20; stkI++ {
			addrInfo.StakingTXs = append(addrInfo.StakingTXs, f.newLegTx())
		}
		key := LegGetAddressKey(addr)
		val, err := rlp.EncodeToBytes(addrInfo)
		if err != nil {
			f.t.Fatal(err)
		}
		if err := db.Put(key, val); err != nil {
			f.t.Fatal(err)
		}
	}
	return db
}

func (f *migrationDBFactory) newAddress() oneAddress {
	f.curAddrIndex++
	return makeOneAddress(f.curAddrIndex)
}

func (f *migrationDBFactory) newLegTx() *LegTxRecord {
	f.curTxIndex++
	return &LegTxRecord{
		Hash:      makeTestTxHash(f.curTxIndex).Hex(),
		Type:      LegSent,
		Timestamp: fmt.Sprintf("%v", f.curTxIndex),
	}
}

type migrationBlockChain struct{}

func (bc *migrationBlockChain) ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64) {
	index := txID.Big().Uint64()
	return common.Hash{}, index / 100, index % 100
}
