package chain

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/trie"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	types2 "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

type fakeReader struct {
	core.FakeChainReader
}

func makeTestAddr(item interface{}) common.Address {
	s := fmt.Sprintf("harmony-one-%v", item)
	return common.BytesToAddress([]byte(s))
}

var (
	validator1 = makeTestAddr("validator1")
	validator2 = makeTestAddr("validator2")
	delegator1 = makeTestAddr("delegator1")
	delegator2 = makeTestAddr("delegator2")
	delegator3 = makeTestAddr("delegator3")
)

var (
	defaultDesc = staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}

	defaultCommissionRates = staking.CommissionRates{
		Rate:          numeric.NewDecWithPrec(1, 1),
		MaxRate:       numeric.NewDecWithPrec(9, 1),
		MaxChangeRate: numeric.NewDecWithPrec(5, 1),
	}
)

func (cr *fakeReader) ReadValidatorList() ([]common.Address, error) {
	return []common.Address{validator1, validator2}, nil
}

func getDatabase() *state.DB {
	database := rawdb.NewMemoryDatabase()
	gspec := core.Genesis{Factory: blockfactory.ForTest}
	genesis := gspec.MustCommit(database)
	chain, _ := core.NewBlockChain(database, nil, nil, nil, vm.Config{}, nil)
	db, _ := chain.StateAt(genesis.Root())
	return db
}

func generateBLSKeyAndSig() (bls.SerializedPublicKey, bls.SerializedSignature) {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub bls.SerializedPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig bls.SerializedSignature
	copy(shardSig[:], sig.Serialize())

	return shardPub, shardSig
}

func sampleWrapper(address common.Address) *staking.ValidatorWrapper {
	pub, _ := generateBLSKeyAndSig()
	v := staking.Validator{
		Address:              address,
		SlotPubKeys:          []bls.SerializedPublicKey{pub},
		LastEpochInCommittee: new(big.Int),
		MinSelfDelegation:    staketest.DefaultMinSelfDel,
		MaxTotalDelegation:   staketest.DefaultMaxTotalDel,
		Commission: staking.Commission{
			CommissionRates: defaultCommissionRates,
			UpdateHeight:    big.NewInt(100),
		},
		Description:    defaultDesc,
		CreationHeight: big.NewInt(100),
	}
}

func makeBlockForTest(epoch int64, index int) *types.Block {
	h := blockfactory.NewTestHeader()

	h.SetEpoch(big.NewInt(epoch))
	h.SetNumber(big.NewInt(doubleSignBlockNumber))
	h.SetViewID(big.NewInt(doubleSignViewID))
	h.SetRoot(common.BigToHash(big.NewInt(int64(index))))

	return types.NewBlockWithHeader(h)
}

func (bc *fakeBlockChain) CurrentBlock() *types.Block {
	return &bc.currentBlock
}
func (bc *fakeBlockChain) CurrentHeader() *block.Header {
	return bc.currentBlock.Header()
}
func (bc *fakeBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block    { return nil }
func (bc *fakeBlockChain) GetHeader(hash common.Hash, number uint64) *block.Header  { return nil }
func (bc *fakeBlockChain) GetHeaderByHash(hash common.Hash) *block.Header           { return nil }
func (bc *fakeBlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts        { return nil }
func (bc *fakeBlockChain) ContractCode(hash common.Hash) ([]byte, error)            { return []byte{}, nil }
func (bc *fakeBlockChain) ValidatorCode(hash common.Hash) ([]byte, error)           { return []byte{}, nil }
func (bc *fakeBlockChain) ShardID() uint32                                          { return 0 }
func (bc *fakeBlockChain) ReadShardState(epoch *big.Int) (*shard.State, error)      { return nil, nil }
func (bc *fakeBlockChain) TrieDB() *trie.Database                                   { return nil }
func (bc *fakeBlockChain) TrieNode(hash common.Hash) ([]byte, error)                { return []byte{}, nil }
func (bc *fakeBlockChain) WriteCommitSig(blockNum uint64, lastCommits []byte) error { return nil }
func (bc *fakeBlockChain) GetHeaderByNumber(number uint64) *block.Header            { return nil }
func (bc *fakeBlockChain) ReadValidatorList() ([]common.Address, error)             { return nil, nil }
func (bc *fakeBlockChain) ReadCommitSig(blockNum uint64) ([]byte, error)            { return nil, nil }
func (bc *fakeBlockChain) ReadBlockRewardAccumulator(uint64) (*big.Int, error)      { return nil, nil }
func (bc *fakeBlockChain) ValidatorCandidates() []common.Address                    { return nil }
func (cr *fakeBlockChain) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorSnapshotAtEpoch(epoch *big.Int, offender common.Address) (*types2.ValidatorSnapshot, error) {
	return &types2.ValidatorSnapshot{
		Validator: makeDefaultValidatorWrapper(),
		Epoch:     epoch,
	}, nil
}
func (bc *fakeBlockChain) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *fakeBlockChain) Config() *params.ChainConfig {
	return params.LocalnetChainConfig
}
func (cr *fakeBlockChain) StateAt(root common.Hash) (*state.DB, error) {
	return nil, nil
}
func (cr *fakeBlockChain) Snapshots() *snapshot.Tree {
	return nil
}
func (bc *fakeBlockChain) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return nil, nil
}
func (bc *fakeBlockChain) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	return nil, nil
}

//
// Fake header for testing
//

func makeFakeHeader() *block.Header {
	h := blockfactory.NewTestHeader()
	h.SetCoinbase(leaderAddr)
	return h
}

//
// Utilities for testing
//

func makeTestAddress(item interface{}) common.Address {
	s := fmt.Sprintf("harmony.one.%v", item)
	return common.BytesToAddress([]byte(s))
}

func makeVoteData(kp blsKeyPair, block *types.Block) slash.Vote {
	return slash.Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{kp.Pub()},
		BlockHeaderHash: block.Hash(),
		Signature:       kp.Sign(block),
	}
}
