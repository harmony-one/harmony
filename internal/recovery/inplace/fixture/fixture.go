// Package fixture builds the deterministic preflight test fixtures: a small
// shard-0 LevelDB carrying a BLS-signed header chain (with real, verifiable
// certificates), the epoch-boundary shard state, and a state trie populated
// with EOA / contract / validator / legacy-code / crafted flag-edge
// accounts.
//
// SECURITY NOTE - fixture-only secrets: the committee BLS secret keys are
// fixed small nonzero scalars (secret i = 32-byte little-endian of i+1),
// exploiting that the pinned SecretKey.SetLittleEndian performs no modular
// reduction. They exist so fixtures are byte-reproducible; they must never
// be used outside tests.
package fixture

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

// Variant selects a build-time state mutation (cases that cannot be created
// by post-hoc key surgery because trie hashes must remain consistent).
type Variant int

const (
	// VariantBase is the passing fixture.
	VariantBase Variant = iota
	// VariantBadAccountLeaf plants an undecodable account leaf value.
	VariantBadAccountLeaf
	// VariantBadStorageLeaf plants a storage leaf whose value is not an RLP
	// byte string.
	VariantBadStorageLeaf
	// VariantFlaggedEmptyCode plants a canonical IsValidator flag on an
	// account with the empty code hash (walker must FAIL).
	VariantFlaggedEmptyCode
	// VariantManyAnomalies plants >20 decoded-zero flag accounts (anomaly
	// truncation row: 20 examples, correct total/by_kind/omitted).
	VariantManyAnomalies
	// VariantWrapperUnbound plants a flagged account whose code is a valid
	// wrapper bound to a DIFFERENT address (walker must FAIL on the
	// address binding).
	VariantWrapperUnbound
)

// ManyAnomaliesCount is the number of decoded-zero accounts planted by
// VariantManyAnomalies (in addition to the base fixture's one).
const ManyAnomaliesCount = 25

// Chain geometry (localnet schedule, 16 blocks/epoch, epoch 3 is staking).
const (
	Epoch          = 3
	BoundaryHeight = 36 // EpochLastBlock(2)
	TargetHeight   = 44
	ChildHeight    = 45
	NumEOA         = 1000
	shard0Slots    = 9 // 6 harmony (nil stake) + 3 external
	shard0Harmony  = 6
)

// Manifest reports what was built, for test assertions and mutations.
type Manifest struct {
	Dir string

	StateRoot  common.Hash
	TargetHash common.Hash
	ChildHash  common.Hash
	Hashes     map[uint64]common.Hash // height -> hash, Boundary..Child

	CertPayload []byte // aggregate signature || bitmap for the target block

	// Committee secrets in slot order (fixture-only scalars, shard 0).
	Secrets []*bls_core.SecretKey
	// PubKeys are the wrappers in committee slot order.
	PubKeys []bls_cosi.PublicKeyWrapper

	ContractAddr     common.Address // multi-node storage + c-namespace code
	Contract2Addr    common.Address // second contract (shares nothing)
	LegacyCodeAddr   common.Address // code stored ONLY at the legacy bare-hash key
	ValidatorAddrs   []common.Address
	FlagZeroAddr     common.Address // crafted decoded-zero flag leaf
	FlagOddAddr      common.Address // crafted non-canonical non-zero flag value
	DualClassAddr    common.Address // unflagged account sharing the FlagOdd wrapper code hash
	FlaggedEmptyAddr common.Address // only in VariantFlaggedEmptyCode
	BadLeafAddr      common.Address // variant-dependent crafted account

	LegacyCodeHash      common.Hash
	ContractCodeHash    common.Hash
	ValidatorCodeHashes []common.Hash // vc-namespace wrapper code hashes (slot order)
	OddWrapperCodeHash  common.Hash   // FlagOddAddr's (and DualClassAddr's) code hash

	committee *shard.State
}

// addr derives a deterministic address from a label.
func addr(label string) common.Address {
	return common.BytesToAddress(crypto.Keccak256([]byte("hmy-preflight-fixture/" + label))[12:])
}

// secretScalar builds the fixture-only secret for slot index i (little-endian
// of i+1, or base+i for other shards).
func secretScalar(n uint64) *bls_core.SecretKey {
	var buf [32]byte
	for i := 0; i < 8; i++ {
		buf[i] = byte(n >> (8 * i))
	}
	sec := &bls_core.SecretKey{}
	if err := sec.SetLittleEndian(buf[:]); err != nil {
		panic(fmt.Sprintf("fixture secret scalar: %v", err))
	}
	return sec
}

// Build creates the fixture database in dir (which must not exist or be
// empty) and returns the manifest. Deterministic for a given variant.
func Build(dir string, variant Variant) (*Manifest, error) {
	shardingconfig.InitLocalnetConfig(16, 16)
	shard.Schedule = shardingconfig.LocalnetSchedule

	db, err := rawdb.NewLevelDBDatabase(dir, 64, 128, "", false)
	if err != nil {
		return nil, fmt.Errorf("open fixture db: %w", err)
	}

	m := &Manifest{Dir: dir, Hashes: make(map[uint64]common.Hash)}

	build := func() error {
		if err := m.buildCommittee(); err != nil {
			return err
		}
		if err := m.buildState(db, variant); err != nil {
			return err
		}
		if err := m.buildChain(db); err != nil {
			return err
		}
		// Flush the memtable/journal into table files: the corruption and
		// relocation test rows operate on .ldb files, and real validator
		// DBs hold their data in tables.
		if err := db.Compact(nil, nil); err != nil {
			return fmt.Errorf("compact fixture: %w", err)
		}
		return nil
	}
	if err := build(); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("close fixture: %w", err)
	}
	if err := canonicalize(dir); err != nil {
		return nil, fmt.Errorf("canonicalize fixture: %w", err)
	}
	return m, nil
}

// canonicalize rewrites the freshly built database into a byte-reproducible
// canonical form. LevelDB embeds per-write sequence numbers in its tables,
// so the physical bytes depend on the write ORDER, which upstream map
// iteration makes nondeterministic even for identical logical content.
// Re-inserting every key-value pair in sorted key order makes the sequence
// numbers (and hence the tables and the manifest) a pure function of the
// content; the timestamped LOG file is dropped. Two generations of the same
// variant are byte-identical afterwards.
func canonicalize(dir string) error {
	src, err := leveldb.OpenFile(dir, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	tmp := dir + ".canonical"
	if err := os.RemoveAll(tmp); err != nil {
		src.Close()
		return err
	}
	dst, err := leveldb.OpenFile(tmp, &opt.Options{ErrorIfExist: true})
	if err != nil {
		src.Close()
		return fmt.Errorf("open canonical: %w", err)
	}

	it := src.NewIterator(nil, nil)
	batch := new(leveldb.Batch)
	n := 0
	for it.Next() {
		batch.Put(append([]byte(nil), it.Key()...), append([]byte(nil), it.Value()...))
		n++
		if n%1024 == 0 {
			if err := dst.Write(batch, nil); err != nil {
				it.Release()
				src.Close()
				dst.Close()
				return err
			}
			batch.Reset()
		}
	}
	it.Release()
	if err := it.Error(); err != nil {
		src.Close()
		dst.Close()
		return fmt.Errorf("iterate source: %w", err)
	}
	if err := dst.Write(batch, nil); err != nil {
		src.Close()
		dst.Close()
		return err
	}
	if err := src.Close(); err != nil {
		dst.Close()
		return err
	}
	if err := dst.CompactRange(util.Range{}); err != nil {
		dst.Close()
		return fmt.Errorf("compact canonical: %w", err)
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(tmp, "LOG")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

func (m *Manifest) buildCommittee() error {
	stake := numeric.NewDec(20000)
	var slots0 shard.SlotList
	for i := 0; i < shard0Slots; i++ {
		sec := secretScalar(uint64(i + 1))
		pub := sec.GetPublicKey()
		wrapper := bls_cosi.PublicKeyWrapper{Object: pub}
		if err := wrapper.Bytes.FromLibBLSPublicKey(pub); err != nil {
			return err
		}
		m.Secrets = append(m.Secrets, sec)
		m.PubKeys = append(m.PubKeys, wrapper)
		slot := shard.Slot{
			EcdsaAddress: addr(fmt.Sprintf("slot0-%d", i)),
			BLSPublicKey: wrapper.Bytes,
		}
		if i >= shard0Harmony {
			s := stake
			slot.EffectiveStake = &s // external, stake-weighted
		}
		slots0 = append(slots0, slot)
	}
	var slots1 shard.SlotList
	for i := 0; i < 3; i++ {
		sec := secretScalar(uint64(101 + i))
		pub := sec.GetPublicKey()
		var ser bls_cosi.SerializedPublicKey
		if err := ser.FromLibBLSPublicKey(pub); err != nil {
			return err
		}
		slots1 = append(slots1, shard.Slot{
			EcdsaAddress: addr(fmt.Sprintf("slot1-%d", i)),
			BLSPublicKey: ser,
		})
	}
	m.committee = &shard.State{
		Epoch: big.NewInt(Epoch),
		Shards: []shard.Committee{
			{ShardID: 0, Slots: slots0},
			{ShardID: 1, Slots: slots1},
		},
	}
	return nil
}

func (m *Manifest) buildState(db ethdb.Database, variant Variant) error {
	sdb := state.NewDatabase(db)
	st, err := state.New(common.Hash{}, sdb, nil)
	if err != nil {
		return err
	}

	// ~1k EOAs for multi-level tries.
	for i := 0; i < NumEOA; i++ {
		a := addr(fmt.Sprintf("eoa-%d", i))
		st.SetBalance(a, big.NewInt(int64(i)*1_000_000_007+13))
		st.SetNonce(a, uint64(i%7))
	}

	// Contract with a multi-node storage trie and c-namespace code.
	m.ContractAddr = addr("contract-1")
	contractCode := deterministicCode("contract-1-code", 2048)
	m.ContractCodeHash = crypto.Keccak256Hash(contractCode)
	st.SetCode(m.ContractAddr, contractCode, false)
	for j := 0; j < 96; j++ {
		key := crypto.Keccak256Hash([]byte(fmt.Sprintf("slot-%d", j)))
		val := crypto.Keccak256Hash([]byte(fmt.Sprintf("value-%d", j)))
		st.SetState(m.ContractAddr, key, val)
	}
	m.Contract2Addr = addr("contract-2")
	st.SetCode(m.Contract2Addr, deterministicCode("contract-2-code", 512), false)
	st.SetState(m.Contract2Addr, common.HexToHash("0x01"), common.HexToHash("0x02"))

	// Legacy bare-hash code account: created normally, then relocated to
	// the legacy location by key surgery after commit.
	m.LegacyCodeAddr = addr("legacy-code")
	legacyCode := deterministicCode("legacy-code-bytes", 300)
	m.LegacyCodeHash = crypto.Keccak256Hash(legacyCode)
	st.SetCode(m.LegacyCodeAddr, legacyCode, false)

	// Validator accounts: canonical flag + vc-namespace wrapper code.
	for i := 0; i < 2; i++ {
		a := addr(fmt.Sprintf("validator-%d", i))
		m.ValidatorAddrs = append(m.ValidatorAddrs, a)
		w := staketest.GetDefaultValidatorWrapperWithAddr(a, []bls_cosi.SerializedPublicKey{m.PubKeys[i].Bytes})
		wBytes, err := rlp.EncodeToBytes(&w)
		if err != nil {
			return err
		}
		m.ValidatorCodeHashes = append(m.ValidatorCodeHashes, crypto.Keccak256Hash(wBytes))
		if err := st.UpdateValidatorWrapper(a, &w); err != nil {
			return fmt.Errorf("update validator wrapper: %w", err)
		}
		st.SetValidatorFlag(a)
		st.SetBalance(a, big.NewInt(1_000_000))
	}

	root, err := st.Commit(false)
	if err != nil {
		return err
	}
	if err := sdb.TrieDB().Commit(root, false); err != nil {
		return err
	}

	// Post-commit crafted accounts via direct trie manipulation (stock
	// SetState cannot write these shapes).
	root, err = m.craftAccounts(db, sdb, root, variant)
	if err != nil {
		return err
	}

	// Legacy-code relocation surgery: move the blob from the c-namespace
	// key to the bare-hash legacy key.
	cKey := append([]byte("c"), m.LegacyCodeHash.Bytes()...)
	blob, err := db.Get(cKey)
	if err != nil {
		return fmt.Errorf("read code for legacy relocation: %w", err)
	}
	if err := db.Put(m.LegacyCodeHash.Bytes(), blob); err != nil {
		return err
	}
	if err := db.Delete(cKey); err != nil {
		return err
	}

	m.StateRoot = root
	return nil
}

// craftAccounts writes the flag-edge accounts by direct trie manipulation:
//   - FlagZeroAddr: IsValidatorKey leaf whose RLP decodes to zero (0x80) -
//     presence-testing would call it flagged; decode-testing keeps it
//     unflagged (anomaly, passing)
//   - FlagOddAddr: non-canonical non-zero flag value (0x02) - flagged +
//     anomaly, wrapper code required and provided
//   - variant-specific FAIL shapes
func (m *Manifest) craftAccounts(db ethdb.Database, sdb state.Database, root common.Hash, variant Variant) (common.Hash, error) {
	triedb := sdb.TrieDB()

	put := func(root common.Hash, address common.Address, tweak func(st *trie.StateTrie) (common.Hash, []byte, error)) (common.Hash, error) {
		addrHash := crypto.Keccak256Hash(address.Bytes())
		storageTrie, err := trie.NewStateTrie(trie.StorageTrieID(root, addrHash, common.Hash{}), triedb)
		if err != nil {
			return common.Hash{}, err
		}
		storageRoot, codeHash, err := tweak(storageTrie)
		if err != nil {
			return common.Hash{}, err
		}
		accountTrie, err := trie.NewStateTrie(trie.StateTrieID(root), triedb)
		if err != nil {
			return common.Hash{}, err
		}
		acct := &ethtypes.StateAccount{
			Nonce:    1,
			Balance:  big.NewInt(42),
			Root:     storageRoot,
			CodeHash: codeHash,
		}
		if err := accountTrie.TryUpdateAccount(address, acct); err != nil {
			return common.Hash{}, err
		}
		// collectLeaf=false: triedb.Update decodes collected account leaves
		// for reference tracking, which the crafted shapes would trip; the
		// fixture flushes to disk immediately and needs no references.
		newRoot, nodes := accountTrie.Commit(false)
		if nodes != nil {
			if err := triedb.Update(trie.NewWithNodeSet(nodes)); err != nil {
				return common.Hash{}, err
			}
		}
		if err := triedb.Commit(newRoot, false); err != nil {
			return common.Hash{}, err
		}
		return newRoot, nil
	}

	commitStorage := func(st *trie.StateTrie) (common.Hash, error) {
		newRoot, nodes := st.Commit(false)
		if nodes != nil {
			if err := triedb.Update(trie.NewWithNodeSet(nodes)); err != nil {
				return common.Hash{}, err
			}
		}
		// Flush the storage nodes to disk directly: with collectLeaf=false
		// on the account commit there is no leaf-derived reference from the
		// account trie to this storage root, so the account-root Commit
		// would not reach these nodes.
		if err := triedb.Commit(newRoot, false); err != nil {
			return common.Hash{}, err
		}
		return newRoot, nil
	}

	emptyCodeHash := crypto.Keccak256(nil)

	// Decoded-zero flag leaf: storage value RLP 0x80 (empty byte string).
	m.FlagZeroAddr = addr("flag-decoded-zero")
	var err error
	root, err = put(root, m.FlagZeroAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
		zeroVal, _ := rlp.EncodeToBytes([]byte{})
		if err := st.TryUpdate(staking.IsValidatorKey.Bytes(), zeroVal); err != nil {
			return common.Hash{}, nil, err
		}
		// A second slot so the trie is not single-leaf.
		other, _ := rlp.EncodeToBytes([]byte{0x33})
		if err := st.TryUpdate(common.HexToHash("0x07").Bytes(), other); err != nil {
			return common.Hash{}, nil, err
		}
		r, err := commitStorage(st)
		return r, emptyCodeHash, err
	})
	if err != nil {
		return common.Hash{}, err
	}

	// Non-canonical non-zero flag value: flagged + anomaly; must carry a
	// valid address-bound wrapper (vc namespace).
	m.FlagOddAddr = addr("flag-noncanonical")
	oddWrapper := staketest.GetDefaultValidatorWrapperWithAddr(m.FlagOddAddr, nil)
	oddBytes, err := rlp.EncodeToBytes(&oddWrapper)
	if err != nil {
		return common.Hash{}, err
	}
	oddCodeHash := crypto.Keccak256Hash(oddBytes)
	if err := db.Put(append([]byte("vc"), oddCodeHash.Bytes()...), oddBytes); err != nil {
		return common.Hash{}, err
	}
	root, err = put(root, m.FlagOddAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
		odd, _ := rlp.EncodeToBytes([]byte{0x02})
		if err := st.TryUpdate(staking.IsValidatorKey.Bytes(), odd); err != nil {
			return common.Hash{}, nil, err
		}
		r, err := commitStorage(st)
		return r, oddCodeHash.Bytes(), err
	})
	if err != nil {
		return common.Hash{}, err
	}
	m.OddWrapperCodeHash = oddCodeHash

	// Dual-class code: an unflagged account referencing the same wrapper
	// code hash (contract class + wrapper-shaped anomaly + dual-class
	// anomaly; all passing).
	m.DualClassAddr = addr("dual-class-contract")
	root, err = put(root, m.DualClassAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
		benign, _ := rlp.EncodeToBytes([]byte{0x44})
		if err := st.TryUpdate(common.HexToHash("0x21").Bytes(), benign); err != nil {
			return common.Hash{}, nil, err
		}
		r, err := commitStorage(st)
		return r, oddCodeHash.Bytes(), err
	})
	if err != nil {
		return common.Hash{}, err
	}

	switch variant {
	case VariantBadAccountLeaf:
		// Plant a garbage account leaf value directly in the account trie.
		m.BadLeafAddr = addr("bad-account-leaf")
		accountTrie, err := trie.NewStateTrie(trie.StateTrieID(root), triedb)
		if err != nil {
			return common.Hash{}, err
		}
		if err := accountTrie.TryUpdate(m.BadLeafAddr.Bytes(), []byte{0xde, 0xad, 0xbe, 0xef}); err != nil {
			return common.Hash{}, err
		}
		newRoot, nodes := accountTrie.Commit(false)
		if nodes != nil {
			if err := triedb.Update(trie.NewWithNodeSet(nodes)); err != nil {
				return common.Hash{}, err
			}
		}
		if err := triedb.Commit(newRoot, false); err != nil {
			return common.Hash{}, err
		}
		root = newRoot
	case VariantBadStorageLeaf:
		// Storage leaf value that is not an RLP byte string (an RLP list).
		m.BadLeafAddr = addr("bad-storage-leaf")
		root, err = put(root, m.BadLeafAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
			listVal, _ := rlp.EncodeToBytes([]interface{}{[]byte{0x01}, []byte{0x02}})
			if err := st.TryUpdate(common.HexToHash("0x11").Bytes(), listVal); err != nil {
				return common.Hash{}, nil, err
			}
			r, err := commitStorage(st)
			return r, emptyCodeHash, err
		})
		if err != nil {
			return common.Hash{}, err
		}
	case VariantFlaggedEmptyCode:
		// Canonical flag, empty code hash: the walker must FAIL.
		m.FlaggedEmptyAddr = addr("flagged-empty-code")
		root, err = put(root, m.FlaggedEmptyAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
			canonical, _ := rlp.EncodeToBytes(staking.IsValidator.Bytes())
			if err := st.TryUpdate(staking.IsValidatorKey.Bytes(), canonical); err != nil {
				return common.Hash{}, nil, err
			}
			r, err := commitStorage(st)
			return r, emptyCodeHash, err
		})
		if err != nil {
			return common.Hash{}, err
		}
	case VariantManyAnomalies:
		for i := 0; i < ManyAnomaliesCount; i++ {
			a := addr(fmt.Sprintf("many-anomalies-%d", i))
			root, err = put(root, a, func(st *trie.StateTrie) (common.Hash, []byte, error) {
				zeroVal, _ := rlp.EncodeToBytes([]byte{})
				if err := st.TryUpdate(staking.IsValidatorKey.Bytes(), zeroVal); err != nil {
					return common.Hash{}, nil, err
				}
				r, err := commitStorage(st)
				return r, emptyCodeHash, err
			})
			if err != nil {
				return common.Hash{}, err
			}
		}
	case VariantWrapperUnbound:
		// Flagged account whose (hash-consistent) code is a wrapper bound
		// to a different address.
		m.BadLeafAddr = addr("wrapper-unbound")
		foreignWrapper := staketest.GetDefaultValidatorWrapperWithAddr(addr("some-other-validator"), nil)
		foreignBytes, err2 := rlp.EncodeToBytes(&foreignWrapper)
		if err2 != nil {
			return common.Hash{}, err2
		}
		foreignHash := crypto.Keccak256Hash(foreignBytes)
		if err2 := db.Put(append([]byte("vc"), foreignHash.Bytes()...), foreignBytes); err2 != nil {
			return common.Hash{}, err2
		}
		root, err = put(root, m.BadLeafAddr, func(st *trie.StateTrie) (common.Hash, []byte, error) {
			canonical, _ := rlp.EncodeToBytes(staking.IsValidator.Bytes())
			if err := st.TryUpdate(staking.IsValidatorKey.Bytes(), canonical); err != nil {
				return common.Hash{}, nil, err
			}
			r, err := commitStorage(st)
			return r, foreignHash.Bytes(), err
		})
		if err != nil {
			return common.Hash{}, err
		}
	}
	return root, nil
}

func deterministicCode(label string, size int) []byte {
	out := make([]byte, 0, size)
	seed := crypto.Keccak256([]byte(label))
	for len(out) < size {
		out = append(out, seed...)
		seed = crypto.Keccak256(seed)
	}
	return out[:size]
}

// buildChain writes headers Boundary..Child with real BLS certificates,
// canonical + reverse mappings, empty bodies, the boundary ss record, the
// exact target block-sig record, and the head pointers.
func (m *Manifest) buildChain(db ethdb.Database) error {
	config := params.LocalnetChainConfig
	factory := blockfactory.NewFactory(config)

	ssBytes, err := shard.EncodeWrapper(*m.committee, true)
	if err != nil {
		return err
	}

	type built struct {
		header *block.Header
		hash   common.Hash
	}
	var prev *built
	for n := uint64(BoundaryHeight); n <= uint64(ChildHeight); n++ {
		epoch := shardingconfig.LocalnetSchedule.CalcEpochNumber(n)
		h := factory.NewHeader(epoch)
		h.SetNumber(new(big.Int).SetUint64(n))
		h.SetShardID(0)
		h.SetViewID(new(big.Int).SetUint64(n))
		h.SetTime(new(big.Int).SetUint64(1_700_000_000 + 2*n))
		h.SetTxHash(types.EmptyRootHash)
		h.SetReceiptHash(types.EmptyRootHash)
		h.SetIncomingReceiptHash(types.EmptyRootHash)
		h.SetCoinbase(addr("leader"))
		if n == uint64(BoundaryHeight) {
			h.SetShardState(ssBytes)
		}
		if n == uint64(TargetHeight) {
			h.SetRoot(m.StateRoot)
		}
		if prev != nil {
			h.SetParentHash(prev.hash)
			sigAndBitmap, err := m.SignPayload(prev.header)
			if err != nil {
				return err
			}
			var sig [96]byte
			copy(sig[:], sigAndBitmap[:96])
			h.SetLastCommitSignature(sig)
			h.SetLastCommitBitmap(sigAndBitmap[96:])
		}
		hash := h.Hash()
		m.Hashes[n] = hash
		if err := rawdb.WriteHeader(db, h); err != nil {
			return err
		}
		if err := rawdb.WriteCanonicalHash(db, hash, n); err != nil {
			return err
		}
		body, err := types.NewBodyForMatchingHeader(h)
		if err != nil {
			return err
		}
		if err := rawdb.WriteBody(db, hash, n, body); err != nil {
			return err
		}
		prev = &built{header: h, hash: hash}
	}

	m.TargetHash = m.Hashes[TargetHeight]
	m.ChildHash = m.Hashes[ChildHeight]

	// The boundary ss record: byte-identical to the boundary header's
	// ShardState (as all production write sites store it).
	if err := rawdb.WriteShardStateBytes(db, big.NewInt(Epoch), ssBytes); err != nil {
		return err
	}
	// Exact block-sig record for the target (certificate source A; the
	// child header carries the same bytes as source B).
	certPayload, err := m.SignPayloadAt(m.TargetHash, TargetHeight)
	if err != nil {
		return err
	}
	m.CertPayload = certPayload
	if err := rawdb.WriteBlockCommitSig(db, TargetHeight, certPayload); err != nil {
		return err
	}
	// Head pointers at the child (upward sample lands on the target).
	if err := rawdb.WriteHeadHeaderHash(db, m.ChildHash); err != nil {
		return err
	}
	return rawdb.WriteHeadBlockHash(db, m.ChildHash)
}

// SignPayload builds the aggregate commit signature || full bitmap for the
// given header.
func (m *Manifest) SignPayload(h *block.Header) ([]byte, error) {
	return m.SignPayloadAt(h.Hash(), h.Number().Uint64())
}

// SignPayloadAt signs the commit payload for (hash, height) with the full
// committee (viewID = height by fixture convention).
func (m *Manifest) SignPayloadAt(hash common.Hash, height uint64) ([]byte, error) {
	epoch := shardingconfig.LocalnetSchedule.CalcEpochNumber(height)
	payload := signature.ConstructCommitPayload(params.LocalnetChainConfig, epoch, hash, height, height)
	return m.SignRaw(payload, nil)
}

// SignRaw signs an arbitrary payload with the committee; signers selects
// slot indices (nil = all).
func (m *Manifest) SignRaw(payload []byte, signers []int) ([]byte, error) {
	if signers == nil {
		signers = make([]int, len(m.Secrets))
		for i := range signers {
			signers[i] = i
		}
	}
	mask := bls_cosi.NewMask(m.PubKeys)
	agg := &bls_core.Sign{}
	for _, idx := range signers {
		sig := m.Secrets[idx].SignHash(payload)
		if sig == nil {
			return nil, fmt.Errorf("bls sign failed for slot %d", idx)
		}
		agg.Add(sig)
		if err := mask.SetBit(idx, true); err != nil {
			return nil, err
		}
	}
	out := append([]byte(nil), agg.Serialize()...)
	if len(out) != 96 {
		return nil, fmt.Errorf("aggregate signature is %d bytes, want 96", len(out))
	}
	return append(out, mask.Bitmap...), nil
}

// committee is kept on the manifest for signing helpers.
func (m *Manifest) Committee() *shard.State { return m.committee }

// ValidatorWrapperBytes returns RLP of a default wrapper bound to addr
// (helper for tests crafting code blobs).
func ValidatorWrapperBytes(a common.Address) ([]byte, common.Hash, error) {
	w := staketest.GetDefaultValidatorWrapperWithAddr(a, nil)
	b, err := rlp.EncodeToBytes(&w)
	if err != nil {
		return nil, common.Hash{}, err
	}
	return b, crypto.Keccak256Hash(b), nil
}
