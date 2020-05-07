package slash

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

const (
	fakeChainErrEpoch = 1
)

var (
	errFakeChainUnexpectEpoch = errors.New("epoch not expected")
)

type fakeBlockChain struct {
	config         params.ChainConfig
	currentBlock   types.Block
	superCommittee shard.State
	snapshots      map[common.Address]staking.ValidatorWrapper
}

func (bc *fakeBlockChain) Config() *params.ChainConfig {
	return &bc.config
}

func (bc *fakeBlockChain) CurrentBlock() *types.Block {
	return &bc.currentBlock
}

func (bc *fakeBlockChain) ReadShardState(epoch *big.Int) (*shard.State, error) {
	if epoch.Cmp(big.NewInt(fakeChainErrEpoch)) == 0 {
		return nil, errFakeChainUnexpectEpoch
	}
	return &bc.superCommittee, nil
}

func (bc *fakeBlockChain) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*staking.ValidatorSnapshot, error) {
	vw, ok := bc.snapshots[addr]
	if !ok {
		return nil, errors.New("missing snapshot")
	}
	return &staking.ValidatorSnapshot{
		Validator: &vw,
		Epoch:     new(big.Int).Set(epoch),
	}, nil
}

type fakeStateDB struct {
	balances  map[common.Address]*big.Int
	vWrappers map[common.Address]*staking.ValidatorWrapper
}

func newFakeStateDB() *fakeStateDB {
	return &fakeStateDB{
		balances:  make(map[common.Address]*big.Int),
		vWrappers: make(map[common.Address]*staking.ValidatorWrapper),
	}
}

func (sdb *fakeStateDB) AddBalance(addr common.Address, amount *big.Int) {
	if _, ok := sdb.balances[addr]; !ok {
		sdb.balances[addr] = common.Big0
	}
	prevBalance := sdb.balances[addr]
	sdb.balances[addr] = new(big.Int).Add(prevBalance, amount)
}

func (sdb *fakeStateDB) ValidatorWrapper(addr common.Address) (*staking.ValidatorWrapper, error) {
	vw, ok := sdb.vWrappers[addr]
	if !ok {
		return nil, fmt.Errorf("address vWrapper not exist")
	}
	return vw, nil
}

func (sdb *fakeStateDB) copy() *fakeStateDB {
	cp := &fakeStateDB{
		balances:  make(map[common.Address]*big.Int),
		vWrappers: make(map[common.Address]*staking.ValidatorWrapper),
	}
	for addr, bal := range sdb.balances {
		cp.balances[addr] = new(big.Int).Set(bal)
	}
	for addr, vw := range sdb.vWrappers {
		vCpy := *vw
		cp.vWrappers[addr] = &vCpy
	}
	return cp
}

func (sdb *fakeStateDB) assertBalance(addr common.Address, balance *big.Int) error {
	val, ok := sdb.balances[addr]
	if !ok {
		return fmt.Errorf("address %x not exist", addr)
	}
	if val.Cmp(balance) != 0 {
		return fmt.Errorf("balance not expected %v / %v", val, balance)
	}
	return nil
}

func (sdb *fakeStateDB) assertEqual(exp *fakeStateDB) error {
	if len(sdb.balances) != len(exp.balances) {
		return fmt.Errorf("balance map size not equal")
	}
	if len(sdb.vWrappers) != len(exp.vWrappers) {
		return fmt.Errorf("vWrapper map size not equal")
	}
	for addr, b1 := range sdb.balances {
		b2, ok := exp.balances[addr]
		if !ok {
			return fmt.Errorf("address %x balance not exist in exp", addr)
		}
		if b1.Cmp(b2) != 0 {
			return fmt.Errorf("balance of %x not expected: %v / %v", addr, b1, b2)
		}
	}
	for addr, vw1 := range sdb.vWrappers {
		vw2, ok := exp.vWrappers[addr]
		if !ok {
			return fmt.Errorf("address %v vWrapper not exist in exp", addr)
		}
		if !reflect.DeepEqual(vw1, vw2) {
			return fmt.Errorf("vWrapper of %x not expected", addr)
		}
	}
	return nil
}
