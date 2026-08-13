package norm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

// slotKeyFor derives a deterministic non-zero serialized BLS public key
// from a byte seed (SanityCheck requires >= 1 slot key; the bytes need not
// be a valid curve point for RLP/round-trip vectors).
func slotKeyFor(seed byte) bls_cosi.SerializedPublicKey {
	var pk bls_cosi.SerializedPublicKey
	pk[0] = seed
	pk[1] = 0x01
	return pk
}

// Test anchor geometry (mirrors mainnet shape at small numbers).
const (
	tTarget       = uint64(44)
	tEpoch        = uint64(3)
	tEpochFirst   = uint64(33)
	tEpochLast    = uint64(48)
	tSnapshotBase = uint64(31) // EpochLastBlock(2)-1
	tBoundary     = uint64(32) // carries ss<3>
)

func testAnchor() Anchor {
	return Anchor{
		Network: "localnet", Shard: 0,
		TargetHeight: tTarget, Epoch: tEpoch,
		EpochFirst: tEpochFirst, EpochLast: tEpochLast,
		SnapshotBase: tSnapshotBase, BoundaryHeight: tBoundary,
		ConfigSHA256Hex: "00",
	}
}

func addr(n byte) common.Address {
	var a common.Address
	a[19] = n
	return a
}

// builder assembles a synthetic source in an in-memory DB.
type builder struct {
	t         *testing.T
	mem       ethdb.Database
	sdb       state.Database
	st        *state.DB
	root      common.Hash
	ssBytes   []byte
	listAddrs []common.Address
	creation  map[common.Address]uint64
}

func newBuilder(t *testing.T) *builder {
	mem := rawdb.NewMemoryDatabase()
	sdb := state.NewDatabase(mem)
	st, err := state.New(common.Hash{}, sdb, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &builder{t: t, mem: mem, sdb: sdb, st: st, ssBytes: []byte("shard-state-epoch-3"), creation: map[common.Address]uint64{}}
}

// wrapper writes a validator wrapper into the target state with the given
// creation height and delegators (index 0 is always the validator itself).
func (b *builder) wrapper(a common.Address, creation uint64, delegators ...common.Address) *staking.ValidatorWrapper {
	w := staketest.GetDefaultValidatorWrapperWithAddr(a, []bls_cosi.SerializedPublicKey{slotKeyFor(a[19])})
	w.Validator.CreationHeight = new(big.Int).SetUint64(creation)
	// index 0 self-delegation already present; append others.
	for _, d := range delegators {
		w.Delegations = append(w.Delegations, staking.NewDelegation(d, big.NewInt(1000)))
	}
	if err := b.st.UpdateValidatorWrapper(a, &w); err != nil {
		b.t.Fatalf("update wrapper %s: %v", a.Hex(), err)
	}
	b.st.SetValidatorFlag(a)
	b.st.SetBalance(a, big.NewInt(1_000_000))
	b.creation[a] = creation
	return &w
}

// commit finalizes the state trie and records the root.
func (b *builder) commit() {
	root, err := b.st.Commit(false)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := b.sdb.TrieDB().Commit(root, false); err != nil {
		b.t.Fatal(err)
	}
	b.root = root
}

func (b *builder) put(key, value []byte) { _ = b.mem.Put(key, value) }

func (b *builder) writeList(addrs []common.Address) {
	b.listAddrs = addrs
	raw, err := rlp.EncodeToBytes(addrs)
	if err != nil {
		b.t.Fatal(err)
	}
	b.put([]byte("validator-list"), raw)
}

func (b *builder) writeRawList(raw []byte) { b.put([]byte("validator-list"), raw) }

func (b *builder) writeDVL(delegator common.Address, idxs staking.DelegationIndexes) {
	raw, err := rlp.EncodeToBytes(idxs)
	if err != nil {
		b.t.Fatal(err)
	}
	b.put(dvlKey(delegator), raw)
}

func (b *builder) writeSnapshotOf(a common.Address, epoch uint64, w *staking.ValidatorWrapper) {
	raw, err := rlp.EncodeToBytes(w)
	if err != nil {
		b.t.Fatal(err)
	}
	b.put(snapshotKey(a, new(big.Int).SetUint64(epoch)), raw)
}

func (b *builder) writeSnapshotRaw(key, value []byte) { b.put(key, value) }

func (b *builder) writeSS(epoch uint64, value []byte) {
	b.put(shardStateKey(new(big.Int).SetUint64(epoch)), value)
}

func (b *builder) writeBlkRwd(number uint64, value []byte) {
	b.put(blkRwdKey(number), value)
}

// sources builds norm.Sources over the committed state (structural-only:
// Hist=nil). The boundary header carries b.ssBytes.
func (b *builder) sources() Sources {
	target, err := state.New(b.root, b.sdb, nil)
	if err != nil {
		b.t.Fatalf("open target state: %v", err)
	}
	return Sources{
		Raw:     b.mem,
		Target:  target,
		Hist:    nil,
		Headers: &stubHeaders{ss: b.ssBytes},
	}
}

// idx builds a DelegationIndex.
func idx(validator common.Address, i, blockNum uint64) staking.DelegationIndex {
	return staking.DelegationIndex{ValidatorAddress: validator, Index: i, BlockNum: new(big.Int).SetUint64(blockNum)}
}

// staketestWrapperValue returns a default wrapper bound to a with a
// self-delegation at index 0 (for raw snapshot bytes in vectors).
func staketestWrapperValue(a common.Address) staking.ValidatorWrapper {
	w := staketest.GetDefaultValidatorWrapperWithAddr(a, []bls_cosi.SerializedPublicKey{slotKeyFor(a[19])})
	w.Validator.CreationHeight = new(big.Int).SetUint64(tSnapshotBase)
	return w
}

type stubHeaders struct{ ss []byte }

func (s *stubHeaders) HeaderByNumber(height uint64) (*block.Header, error) {
	h := blockfactory.NewFactory(params.LocalnetChainConfig).NewHeader(big.NewInt(int64(tEpoch)))
	h.SetShardState(s.ss)
	return h, nil
}

// findFinding returns the first finding with the given code.
func findFinding(res *Result, code string) *reportFindingView {
	for i := range res.Findings {
		if res.Findings[i].Code == code {
			return &reportFindingView{res.Findings[i].Severity == "fatal", string(res.Findings[i].Class)}
		}
	}
	return nil
}

type reportFindingView struct {
	fatal bool
	class string
}
