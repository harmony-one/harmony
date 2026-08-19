// Package fixture generates the metadata acceptance kit: deterministic
// localnet twin chains with REAL BLS commit certificates from the
// well-known public dev keys (.hmy/*.key, empty passphrase), driven through
// the stock worker + processor + InsertChain path so they are replay-grade
// by construction (plan WS7).
//
// Block timestamps are fully deterministic (a fixed base plus a fixed step,
// below the localnet TimestampValidationEpoch) so a committed golden .hmr
// digest is stable across regenerations. Never used in production; the dev
// keys are public test keys and leak no secrets.
package fixture

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	coregenesis "github.com/harmony-one/harmony/core/genesis"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	recoveryanchor "github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/utils"
	pkgworker "github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// LocalnetTestKeyHex is the committed localnet-only funded test key
// (core/genesis.go).
const LocalnetTestKeyHex = "1f84c95ac16e6a50f08d44c7bde7aff8742212fda6e4321fde48bf83bef266dc"

const baseTime = 1_700_000_000

// Spec drives one deterministic chain generation.
type Spec struct {
	Dir     string
	KeysDir string
	Blocks  uint64 // total blocks to produce past genesis

	CreateValidatorAt uint64 // pre-target create-validator (0 = never)
	DelegateAt        uint64 // native delegate from the test account (0 = never)

	// Post-target ops (produce the removals the audit reconciles).
	PostCreateValidatorAt uint64
	PostDelegateAt        uint64

	// PostTopUpAt repeats the PostDelegateAt delegation (same delegator,
	// same validator, larger stake): it produces NO new dvl index
	// (addDelegationIndex appends only when the pair is absent), so the
	// audit must classify it attempted-but-not-metadata-producing.
	PostTopUpAt uint64

	// Precompile (0xfc) delegation matrix (WS6 acceptance): at
	// FundPrecompileAt (pre-target) a deterministic fixture EOA is funded
	// and two tiny forwarder contracts are deployed with balance — proxy
	// (forwards calldata to 0xfc, propagates failure) and reverter (same,
	// then always REVERTs). The post-target heights then produce one 0xfc
	// Delegate of each classification:
	//   direct   EOA→0xfc, new pair            → metadata-producing
	//   nested   EOA→proxy→0xfc (contract
	//            self-delegation), new pair    → metadata-producing
	//   reverted EOA→reverter→0xfc then REVERT → StakeMsgs-visible, NOT producing
	//   top-up   direct repeat of the same pair→ visible, NOT producing
	FundPrecompileAt   uint64
	PrecompileDirectAt uint64
	PrecompileNestedAt uint64
	PrecompileRevertAt uint64
	PrecompileTopUpAt  uint64

	// Additional native staking directives (WS6 directive-matrix
	// acceptance) on the PRE-target validator, which exists and holds
	// delegations by the target so these succeed immediately (no reward
	// maturity required): EditValidator (deployer edits its own validator),
	// Undelegate (the test account undelegates part of its block-26
	// delegation). PrecompileUndelegateAt undelegates the precompile EOA's
	// own delegation through 0xfc (requires a prior 0xfc/native Delegate
	// from that EOA).
	EditValidatorAt        uint64
	UndelegateAt           uint64
	PrecompileUndelegateAt uint64

	// CollectRewardsAt (native, test account) and PrecompileCollectRewardsAt
	// (0xfc, precompile EOA) collect accrued delegation rewards. Rewards only
	// exist once the pre-target validator is ELECTED and its signing is paid
	// out: the block-22 validator wins a shard-0 slot in the epoch-3 election
	// (blocks 37+ are signed with the full committee including its key), and
	// localnet pays aggregated rewards every 16 blocks (at heights ≡15 mod
	// 16), so the first funded payout lands at block 47. Both delegations
	// must predate the epoch-3 snapshot (block 36) to receive a share, and
	// collections must be scheduled at heights ≥48 with Blocks ≥ them.
	CollectRewardsAt           uint64
	PrecompileCollectRewardsAt uint64

	// IncomingReceiptAt (post-target) makes the block at that height carry a
	// GENUINE, fully-verifiable incoming cross-shard receipt from shard 1
	// (see makeShard1IncomingReceipt): the CXReceiptsProof's source header is
	// signed by shard-1's dev committee, the merkle proof verifies, and the
	// receipt is APPLIED to the proposal state so the block re-executes to its
	// root. Block insertion writes the spent marker, so on the abandoned
	// branch the audit's pass 1 sees the receipt as already spent
	// (incoming-receipts pollution) and pass 2 masks the marker, after which
	// the full proof re-verifies cleanly — the spent-marker analog of the
	// crosslink pollution path.
	IncomingReceiptAt uint64

	// PreCrossLinkShard1At (pre-target) and CrossLinkShard1At (post-target)
	// make those blocks carry GENUINE, validly-signed shard-1 crosslinks (see
	// makeShard1CrossLink) referencing shard-1 blocks 3 and 4 respectively.
	// Block insertion writes the crosslink markers and advances the shard-1
	// continuity pointer to 4. On the abandoned branch the audit therefore:
	//   - retains the pre-target crosslink (shard-1 block 3) as sPre;
	//   - sees the post-target crosslink (block 4) as an already-exist
	//     crosslink in pass 1 (pollution-suspect) and masks it in pass 2;
	//   - derives the pre-target continuity pointer (3) uniquely from the
	//     stored pointer (4) via the invariant solver.
	// This exercises the shard-1 crosslink subset, pass-two pollution
	// clearing, and the pointer solver end-to-end. Set both for the full path.
	PreCrossLinkShard1At uint64
	CrossLinkShard1At    uint64
}

// Chain is an open fixture chain.
type Chain struct {
	DB      ethdb.Database
	Bc      core.BlockChain
	Keys    []bls2.PrivateKeyWrapper
	Slots   shard.SlotList
	KeysDir string

	ValidatorAddr     common.Address // pre-target validator (contract-deployer account)
	PostValidatorAddr common.Address // post-target validator (test account)

	// Precompile matrix actors (populated when the Spec schedules them).
	PrecompileEOA common.Address // deterministic fixture EOA (0xfc caller)
	ProxyAddr     common.Address // forwarder contract (nested delegate)
	ReverterAddr  common.Address // forwarder that always REVERTs after the 0xfc call

	precompileKey *ecdsa.PrivateKey
	extraKeysDir  string
}

// Open opens (or initializes) the localnet chain database.
func Open(dir, keysDir string) (*Chain, error) {
	shardingconfig.InitLocalnetConfig(
		nodeconfig.GetDefaultLocalnetBlocksPerEpoch(),
		nodeconfig.GetDefaultLocalnetBlocksPerEpochV2(),
	)
	shard.Schedule = shardingconfig.LocalnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Localnet)
	nodeconfig.SetShardingSchedule(shardingconfig.LocalnetSchedule)
	utils.SetLogVerbosity(2)

	db, err := rawdb.NewLevelDBDatabase(dir, 64, 512, "", false)
	if err != nil {
		return nil, fmt.Errorf("fixture: open db: %w", err)
	}
	if rawdb.ReadCanonicalHash(db, 0) == (common.Hash{}) {
		gi := &core.GenesisInitializer{NetworkType: nodeconfig.Localnet}
		if err := gi.InitChainDB(db, 0); err != nil {
			db.Close()
			return nil, fmt.Errorf("fixture: init genesis: %w", err)
		}
	}
	cfg := nodeconfig.GetShardConfig(0).GetNetworkType().ChainConfig()
	// beacon shard-0 chain-id semantics (shardchains.go:130-133).
	cfg.EthCompatibleChainID = big.NewInt(cfg.EthCompatibleShard0ChainID.Int64())
	bc, err := core.NewBlockChainWithOptions(
		db, nil, nil,
		&core.CacheConfig{Disabled: true, Preimages: true, SnapshotLimit: 0},
		&cfg, chain.NewEngine(), vm.Config{}, core.Options{},
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("fixture: open chain: %w", err)
	}
	c := &Chain{
		DB: db, Bc: bc, KeysDir: keysDir,
		ValidatorAddr: crypto.PubkeyToAddress(coregenesis.ContractDeployerKey.PublicKey),
		extraKeysDir:  filepath.Join(filepath.Dir(dir), "fixture-keys"),
	}
	testKey, _ := crypto.HexToECDSA(LocalnetTestKeyHex)
	c.PostValidatorAddr = crypto.PubkeyToAddress(testKey.PublicKey)
	// Deterministic fixture-only 0xfc-caller EOA (never a real key).
	precKey, err := crypto.ToECDSA(crypto.Keccak256([]byte("hmy-metadata-fixture-precompile-eoa")))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("fixture: precompile eoa key: %w", err)
	}
	c.precompileKey = precKey
	c.PrecompileEOA = crypto.PubkeyToAddress(precKey.PublicKey)
	if err := c.loadCommittee(c.Bc.CurrentBlock().Epoch()); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

// loadCommittee loads the shard-0 committee for the given epoch (the
// committee that signs blocks of that epoch). At an epoch boundary the
// block being produced belongs to the NEW epoch, whose shard state was
// written at the last block of the prior epoch.
func (c *Chain) loadCommittee(epoch *big.Int) error {
	ss, err := rawdb.ReadShardState(c.DB, epoch)
	if err != nil {
		return fmt.Errorf("fixture: read shard state epoch %d: %w", epoch, err)
	}
	comm, err := ss.FindCommitteeByID(0)
	if err != nil {
		return err
	}
	keys, err := c.loadKeysForSlots(comm.Slots)
	if err != nil {
		return err
	}
	c.Slots = comm.Slots
	c.Keys = keys
	return nil
}

// loadKeysForSlots loads the BLS secret keys backing a committee's slots,
// trying the repo dev keys (KeysDir/<pub>.key, empty passphrase) first and
// falling back to the fixture-only *.hex secrets under extraKeysDir. Used for
// both the shard-0 signer committee and, for the crosslink fixture, another
// shard's committee whose dev keys sign a genuine cross-shard crosslink.
func (c *Chain) loadKeysForSlots(slots shard.SlotList) ([]bls2.PrivateKeyWrapper, error) {
	var keys []bls2.PrivateKeyWrapper
	for _, slot := range slots {
		keyFile := filepath.Join(c.KeysDir, slot.BLSPublicKey.Hex()+".key")
		sec, err := blsgen.LoadBLSKeyWithPassPhrase(keyFile, "")
		if err != nil {
			raw, rerr := os.ReadFile(filepath.Join(c.extraKeysDir, slot.BLSPublicKey.Hex()+".hex"))
			if rerr != nil {
				return nil, fmt.Errorf("fixture: load committee key %s: %w (no fixture key either: %v)", keyFile, err, rerr)
			}
			sec = &bls_core.SecretKey{}
			if err := sec.DeserializeHexStr(strings.TrimSpace(string(raw))); err != nil {
				return nil, fmt.Errorf("fixture: decode fixture key %s: %w", slot.BLSPublicKey.Hex(), err)
			}
		}
		keys = append(keys, bls2.WrapperFromPrivateKey(sec))
	}
	return keys, nil
}

// makeShard1CrossLink builds a GENUINE, validly-signed crosslink for shard 1
// at the given epoch: it reads shard-1's committee for that epoch from the
// beacon shard state and aggregate-signs the commit payload with that
// committee's dev secret keys (full bitmap → quorum). engine.VerifyCrossLink
// therefore accepts it. The referenced shard-1 block need not exist in the
// beacon DB — VerifyCrossLink authenticates only the signature over
// (epoch, hash, blockNum, viewID). Used to drive the audit's shard-1
// crosslink subset extraction and pass-2 pollution masking through the real
// two-pass loop (blockNum must be > 1 and epoch must be a crosslink epoch).
func (c *Chain) makeShard1CrossLink(epoch *big.Int, blockNum, viewID uint64, hash common.Hash) (*types.CrossLink, error) {
	ss, err := rawdb.ReadShardState(c.DB, epoch)
	if err != nil {
		return nil, fmt.Errorf("fixture: read shard state epoch %d for shard-1 crosslink: %w", epoch, err)
	}
	comm, err := ss.FindCommitteeByID(1)
	if err != nil {
		return nil, fmt.Errorf("fixture: no shard-1 committee at epoch %d: %w", epoch, err)
	}
	keys, err := c.loadKeysForSlots(comm.Slots)
	if err != nil {
		return nil, err
	}
	pubs := make([]bls2.PublicKeyWrapper, len(keys))
	for i, k := range keys {
		pubs[i] = *k.Pub
	}
	mask := bls2.NewMask(pubs)
	payload := consensus_sig.ConstructCommitPayload(
		c.Bc.Config(), epoch, hash, blockNum, viewID,
	)
	var agg bls_core.Sign
	for i, k := range keys {
		if err := mask.SetBit(i, true); err != nil {
			return nil, err
		}
		agg.Add(k.Pri.SignHash(payload))
	}
	var sig [96]byte
	copy(sig[:], agg.Serialize())
	return &types.CrossLink{
		HashF:        hash,
		BlockNumberF: new(big.Int).SetUint64(blockNum),
		ViewIDF:      new(big.Int).SetUint64(viewID),
		SignatureF:   sig,
		BitmapF:      mask.Mask(),
		ShardIDF:     1,
		EpochF:       new(big.Int).Set(epoch),
	}, nil
}

func (c *Chain) makeCreateValidatorTx(validator common.Address, signer *ecdsaSigner, nonce uint64) (*staking.StakingTransaction, error) {
	// Deterministic fixture-only BLS secret (a fixed small scalar keyed by
	// the validator address) so the whole chain — and therefore the golden
	// .hmr — is byte-reproducible. Never used outside tests.
	blsSec := deterministicBLSSecret(validator)
	pub := bls2.SerializedPublicKey{}
	if err := pub.FromLibBLSPublicKey(blsSec.GetPublicKey()); err != nil {
		return nil, err
	}
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	var sig bls2.SerializedSignature
	copy(sig[:], blsSec.SignHash(msgHash[:]).Serialize())
	if err := os.MkdirAll(c.extraKeysDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(c.extraKeysDir, pub.Hex()+".hex"), []byte(blsSec.SerializeToHexStr()), 0o644); err != nil {
		return nil, err
	}
	one := big.NewInt(denominations.One)
	rate, _ := numeric.NewDecFromStr("0.1")
	maxRate, _ := numeric.NewDecFromStr("0.9")
	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCreateValidator, staking.CreateValidator{
			ValidatorAddress: validator,
			Description: staking.Description{
				Name: "fixture-validator", Identity: "fx-" + validator.Hex()[:8],
				Website: "https://fixture.invalid", SecurityContact: "fixture", Details: "metadata fixture validator",
			},
			CommissionRates:    staking.CommissionRates{Rate: rate, MaxRate: maxRate, MaxChangeRate: maxChangeRate},
			MinSelfDelegation:  new(big.Int).Mul(big.NewInt(10_000), one),
			MaxTotalDelegation: new(big.Int).Mul(big.NewInt(1_000_000), one),
			SlotPubKeys:        []bls2.SerializedPublicKey{pub},
			SlotKeySigs:        []bls2.SerializedSignature{sig},
			Amount:             new(big.Int).Mul(big.NewInt(100_000), one),
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
}

func (c *Chain) makeDelegateTx(delegator, validator common.Address, signer *ecdsaSigner, nonce uint64) (*staking.StakingTransaction, error) {
	one := big.NewInt(denominations.One)
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveDelegate, staking.Delegate{
			DelegatorAddress: delegator, ValidatorAddress: validator,
			Amount: new(big.Int).Mul(big.NewInt(1_000), one),
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
}

func (c *Chain) makeEditValidatorTx(validator common.Address, signer *ecdsaSigner, nonce uint64) (*staking.StakingTransaction, error) {
	// Minimal edit: bump the commission rate (always valid, no maturity or
	// election required). Produces a native EditValidator body entry.
	newRate, _ := numeric.NewDecFromStr("0.11")
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveEditValidator, staking.EditValidator{
			ValidatorAddress: validator,
			Description:      staking.Description{Name: "fixture-validator-edited"},
			CommissionRate:   &newRate,
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
}

func (c *Chain) makeUndelegateTx(delegator, validator common.Address, amount *big.Int, signer *ecdsaSigner, nonce uint64) (*staking.StakingTransaction, error) {
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveUndelegate, staking.Undelegate{
			DelegatorAddress: delegator, ValidatorAddress: validator, Amount: amount,
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
}

func (c *Chain) makeCollectRewardsTx(delegator common.Address, signer *ecdsaSigner, nonce uint64) (*staking.StakingTransaction, error) {
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCollectRewards, staking.CollectRewards{
			DelegatorAddress: delegator,
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
}

// fcAddr is the write-capable staking precompile (core/vm/contracts_write.go).
var fcAddr = common.BytesToAddress([]byte{252})

// forwarderRuntime is the shared forwarder body: copy the full calldata to
// memory and CALL 0xfc with it (no value — the delegation amount is an ABI
// argument deducted from the delegator's balance by the precompile).
//
//	CALLDATASIZE PUSH1 0 PUSH1 0 CALLDATACOPY
//	PUSH1 0 PUSH1 0 CALLDATASIZE PUSH1 0 PUSH1 0 PUSH1 0xfc GAS CALL
//
// leaving the CALL success flag on the stack (19 bytes).
var forwarderRuntime = []byte{
	0x36, 0x60, 0x00, 0x60, 0x00, 0x37,
	0x60, 0x00, 0x60, 0x00, 0x36, 0x60, 0x00, 0x60, 0x00, 0x60, 0xfc, 0x5a, 0xf1,
}

// proxyRuntime propagates the inner result: STOP on success, REVERT(0,0)
// on failure (ISZERO PUSH1 24 JUMPI STOP JUMPDEST PUSH1 0 PUSH1 0 REVERT).
var proxyRuntime = append(append([]byte(nil), forwarderRuntime...),
	0x15, 0x60, 0x18, 0x57, 0x00, 0x5b, 0x60, 0x00, 0x60, 0x00, 0xfd)

// reverterRuntime always REVERTs after the (successful) inner 0xfc call:
// the recorded op is StakeMsgs-visible but its state effect rolls back.
var reverterRuntime = append(append([]byte(nil), forwarderRuntime...),
	0x50, 0x60, 0x00, 0x60, 0x00, 0xfd) // POP REVERT(0,0)

// deployCode wraps a runtime blob in the minimal init code
// (PUSH1 len PUSH1 12 PUSH1 0 CODECOPY PUSH1 len PUSH1 0 RETURN).
func deployCode(runtime []byte) []byte {
	l := byte(len(runtime))
	init := []byte{0x60, l, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, l, 0x60, 0x00, 0xf3}
	return append(init, runtime...)
}

// delegateCalldata ABI-encodes Delegate(delegatorAddress, validatorAddress,
// amount) for the 0xfc staking precompile (staking/precompile.go ABI).
func delegateCalldata(delegator, validator common.Address, amount *big.Int) []byte {
	return tripletCalldata("Delegate(address,address,uint256)", delegator, validator, amount)
}

// undelegateCalldata ABI-encodes Undelegate(delegatorAddress,
// validatorAddress, amount) for the 0xfc staking precompile.
func undelegateCalldata(delegator, validator common.Address, amount *big.Int) []byte {
	return tripletCalldata("Undelegate(address,address,uint256)", delegator, validator, amount)
}

// collectRewardsCalldata ABI-encodes CollectRewards(delegatorAddress) for
// the 0xfc staking precompile.
func collectRewardsCalldata(delegator common.Address) []byte {
	sel := crypto.Keccak256([]byte("CollectRewards(address)"))[:4]
	data := make([]byte, 4+32)
	copy(data, sel)
	copy(data[4+12:4+32], delegator.Bytes())
	return data
}

func tripletCalldata(sig string, delegator, validator common.Address, amount *big.Int) []byte {
	sel := crypto.Keccak256([]byte(sig))[:4]
	data := make([]byte, 4+3*32)
	copy(data, sel)
	copy(data[4+12:4+32], delegator.Bytes())
	copy(data[36+12:36+32], validator.Bytes())
	amount.FillBytes(data[68:100])
	return data
}

func (c *Chain) signCommit(blk *types.Block) ([]byte, error) {
	pubs := make([]bls2.PublicKeyWrapper, len(c.Keys))
	for i, k := range c.Keys {
		pubs[i] = *k.Pub
	}
	mask := bls2.NewMask(pubs)
	payload := consensus_sig.ConstructCommitPayload(
		c.Bc.Config(), blk.Epoch(), blk.Hash(), blk.NumberU64(), blk.Header().ViewID().Uint64(),
	)
	var agg bls_core.Sign
	for i, k := range c.Keys {
		if err := mask.SetBit(i, true); err != nil {
			return nil, err
		}
		agg.Add(k.Pri.SignHash(payload))
	}
	return append(agg.Serialize(), mask.Mask()...), nil
}

// Generate produces spec.Blocks blocks with deterministic timestamps and
// the scheduled staking operations.
func (c *Chain) Generate(spec Spec) error {
	testKey, err := crypto.HexToECDSA(LocalnetTestKeyHex)
	if err != nil {
		return err
	}
	testSigner := &ecdsaSigner{key: testKey, addr: crypto.PubkeyToAddress(testKey.PublicKey)}
	deployerSigner := &ecdsaSigner{key: coregenesis.ContractDeployerKey, addr: c.ValidatorAddr}

	head := c.Bc.CurrentBlock()
	lastSig := head.GetCurrentCommitSig()
	if head.NumberU64() == 0 {
		lastSig = nil
	}
	sched := shard.Schedule

	for produced := uint64(0); produced < spec.Blocks; produced++ {
		number := c.Bc.CurrentBlock().NumberU64() + 1
		// The block being produced belongs to epoch(number); its signers
		// (whose bitmap the child's reward calc validates) are that epoch's
		// committee, which may differ from the parent's at a boundary.
		if err := c.loadCommittee(sched.CalcEpochNumber(number)); err != nil {
			return err
		}
		leaderSlot := c.Slots[0]
		leaderKey := c.Keys[0]

		worker := pkgworker.New(c.Bc, c.Bc)
		hdr := worker.GetCurrentHeader()
		hdr.SetTime(big.NewInt(baseTime + int64(number)*2)) // deterministic

		coinbase := leaderSlot.EcdsaAddress
		if c.Bc.Config().IsStaking(hdr.Epoch()) {
			coinbase = utils.GetAddressFromBLSPubKeyBytes(leaderSlot.BLSPublicKey[:])
		}
		if c.Bc.Config().IsVRF(hdr.Epoch()) {
			sk := vrf_bls.NewVRFSigner(leaderKey.Pri)
			parentHash := c.Bc.CurrentBlock().Hash()
			vrf, proof := sk.Evaluate(parentHash[:])
			if proof == nil {
				return fmt.Errorf("fixture: VRF generation failed at %d", number)
			}
			hdr.SetVrf(append(vrf[:], proof...))
		}

		st, err := c.Bc.State()
		if err != nil {
			return err
		}

		var pendingStaking staking.StakingTransactions
		if spec.CreateValidatorAt != 0 && number == spec.CreateValidatorAt {
			stx, err := c.makeCreateValidatorTx(c.ValidatorAddr, deployerSigner, st.GetNonce(c.ValidatorAddr))
			if err != nil {
				return fmt.Errorf("fixture: create-validator at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.DelegateAt != 0 && number == spec.DelegateAt {
			stx, err := c.makeDelegateTx(testSigner.addr, c.ValidatorAddr, testSigner, st.GetNonce(testSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: delegate at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.PostCreateValidatorAt != 0 && number == spec.PostCreateValidatorAt {
			stx, err := c.makeCreateValidatorTx(c.PostValidatorAddr, testSigner, st.GetNonce(testSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: post create-validator at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.PostDelegateAt != 0 && number == spec.PostDelegateAt {
			stx, err := c.makeDelegateTx(deployerSigner.addr, c.PostValidatorAddr, deployerSigner, st.GetNonce(deployerSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: post delegate at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.PostTopUpAt != 0 && number == spec.PostTopUpAt {
			stx, err := c.makeDelegateTx(deployerSigner.addr, c.PostValidatorAddr, deployerSigner, st.GetNonce(deployerSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: post top-up delegate at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.EditValidatorAt != 0 && number == spec.EditValidatorAt {
			stx, err := c.makeEditValidatorTx(c.ValidatorAddr, deployerSigner, st.GetNonce(deployerSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: edit-validator at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.UndelegateAt != 0 && number == spec.UndelegateAt {
			// The test account undelegates half of its block-26 delegation
			// to the pre-target validator (immediate; no maturity needed).
			half := new(big.Int).Mul(big.NewInt(500), big.NewInt(denominations.One))
			stx, err := c.makeUndelegateTx(testSigner.addr, c.ValidatorAddr, half, testSigner, st.GetNonce(testSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: undelegate at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if spec.CollectRewardsAt != 0 && number == spec.CollectRewardsAt {
			// Collects the test account's accrued delegation rewards (the
			// block-26 delegation shares the elected validator's block-47
			// aggregated payout — see the Spec comment).
			stx, err := c.makeCollectRewardsTx(testSigner.addr, testSigner, st.GetNonce(testSigner.addr))
			if err != nil {
				return fmt.Errorf("fixture: collect-rewards at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		pendingRegular := map[common.Address]types.Transactions{}
		gasPrice := big.NewInt(100e9)
		one := big.NewInt(denominations.One)
		addRegular := func(signer *ecdsaSigner, tx *types.Transaction) error {
			signed, err := types.SignTx(tx, types.NewEIP155Signer(c.Bc.Config().ChainID), signer.key)
			if err != nil {
				return err
			}
			pendingRegular[signer.addr] = append(pendingRegular[signer.addr], signed)
			return nil
		}
		precSigner := &ecdsaSigner{key: c.precompileKey, addr: c.PrecompileEOA}
		delegAmt := new(big.Int).Mul(big.NewInt(1_000), one)
		if spec.FundPrecompileAt != 0 && number == spec.FundPrecompileAt {
			// Fund the 0xfc-caller EOA and deploy the two forwarders with
			// balance (the precompile deducts the delegation amount from
			// the delegator — the proxy delegates its OWN balance).
			// STRICTLY DESCENDING gas prices: the block packs txs by
			// price, and equal prices across senders tie-break by map
			// iteration order — nondeterministic block bytes otherwise.
			fund := new(big.Int).Mul(big.NewInt(100_000), one)
			if err := addRegular(testSigner, types.NewTransaction(
				st.GetNonce(testSigner.addr), c.PrecompileEOA, 0, fund, 21_000, big.NewInt(103e9), nil)); err != nil {
				return fmt.Errorf("fixture: fund precompile eoa at %d: %w", number, err)
			}
			contractFund := new(big.Int).Mul(big.NewInt(10_000), one)
			dn := st.GetNonce(deployerSigner.addr)
			c.ProxyAddr = crypto.CreateAddress(deployerSigner.addr, dn)
			c.ReverterAddr = crypto.CreateAddress(deployerSigner.addr, dn+1)
			if err := addRegular(deployerSigner, types.NewContractCreation(
				dn, 0, contractFund, 3_000_000, big.NewInt(102e9), deployCode(proxyRuntime))); err != nil {
				return fmt.Errorf("fixture: deploy proxy at %d: %w", number, err)
			}
			if err := addRegular(deployerSigner, types.NewContractCreation(
				dn+1, 0, contractFund, 3_000_000, big.NewInt(102e9), deployCode(reverterRuntime))); err != nil {
				return fmt.Errorf("fixture: deploy reverter at %d: %w", number, err)
			}
		}
		if spec.PrecompileDirectAt != 0 && number == spec.PrecompileDirectAt {
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), fcAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				delegateCalldata(c.PrecompileEOA, c.ValidatorAddr, delegAmt))); err != nil {
				return fmt.Errorf("fixture: direct 0xfc delegate at %d: %w", number, err)
			}
		}
		if spec.PrecompileNestedAt != 0 && number == spec.PrecompileNestedAt {
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), c.ProxyAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				delegateCalldata(c.ProxyAddr, c.ValidatorAddr, delegAmt))); err != nil {
				return fmt.Errorf("fixture: nested 0xfc delegate at %d: %w", number, err)
			}
		}
		if spec.PrecompileRevertAt != 0 && number == spec.PrecompileRevertAt {
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), c.ReverterAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				delegateCalldata(c.ReverterAddr, c.ValidatorAddr, delegAmt))); err != nil {
				return fmt.Errorf("fixture: reverted 0xfc delegate at %d: %w", number, err)
			}
		}
		if spec.PrecompileTopUpAt != 0 && number == spec.PrecompileTopUpAt {
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), fcAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				delegateCalldata(c.PrecompileEOA, c.ValidatorAddr, delegAmt))); err != nil {
				return fmt.Errorf("fixture: top-up 0xfc delegate at %d: %w", number, err)
			}
		}
		if spec.PrecompileUndelegateAt != 0 && number == spec.PrecompileUndelegateAt {
			half := new(big.Int).Mul(big.NewInt(500), one)
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), fcAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				undelegateCalldata(c.PrecompileEOA, c.ValidatorAddr, half))); err != nil {
				return fmt.Errorf("fixture: 0xfc undelegate at %d: %w", number, err)
			}
		}
		if spec.PrecompileCollectRewardsAt != 0 && number == spec.PrecompileCollectRewardsAt {
			// Collects the precompile EOA's accrued rewards through 0xfc
			// (its pre-snapshot delegation shares the block-47 payout).
			if err := addRegular(precSigner, types.NewTransaction(
				st.GetNonce(c.PrecompileEOA), fcAddr, 0, big.NewInt(0), 2_000_000, gasPrice,
				collectRewardsCalldata(c.PrecompileEOA))); err != nil {
				return fmt.Errorf("fixture: 0xfc collect-rewards at %d: %w", number, err)
			}
		}
		if len(pendingStaking) > 0 || len(pendingRegular) > 0 {
			if err := worker.CommitTransactions(pendingRegular, pendingStaking, coinbase); err != nil {
				return fmt.Errorf("fixture: commit txs at %d: %w", number, err)
			}
		}
		if spec.IncomingReceiptAt != 0 && number == spec.IncomingReceiptAt {
			// Incoming receipts are applied after transactions (consistent
			// with the proposal order in the stock node). Shard-1 source
			// block 5 (arbitrary, distinct from the crosslink numbers).
			cxp, err := c.makeShard1IncomingReceipt(
				hdr.Epoch(), 5, 5, testSigner.addr,
				new(big.Int).Mul(big.NewInt(7), big.NewInt(denominations.One)))
			if err != nil {
				return fmt.Errorf("fixture: incoming receipt at %d: %w", number, err)
			}
			if err := worker.CommitReceipts([]*types.CXReceiptsProof{cxp}); err != nil {
				return fmt.Errorf("fixture: commit incoming receipt at %d: %w", number, err)
			}
			// Store the source shard's crosslink alongside, as mainnet does:
			// the beacon accepts an incoming receipt only after storing the
			// source block's crosslink, whose (signature, bitmap) pair comes
			// from the consensus message — NOT from the stored body. That
			// stored body is corrupted by the long-standing
			// CXReceiptsProof.Copy bug (CommitBitmap overwritten with a copy
			// of CommitSig at rawdb.WriteBlock time), so this crosslink is
			// the only surviving carrier of the true quorum bitmap and the
			// audit's restoration source (audit/legacybitmap.go).
			if err := c.writeShard1CrossLinkFor(cxp); err != nil {
				return fmt.Errorf("fixture: store crosslink for incoming receipt at %d: %w", number, err)
			}
		}

		var nextShardState *shard.State
		if sched.IsLastBlock(number) {
			nextShardState, err = c.Bc.SuperCommitteeForNextEpoch(c.Bc, hdr, false)
			if err != nil {
				return fmt.Errorf("fixture: next-epoch committee at %d: %w", number, err)
			}
		}

		var crossLinks types.CrossLinks
		// Shard-1 crosslinks: pre-target references shard-1 block 3, post-target
		// references shard-1 block 4 (both >1, crosslink-epoch). The hash is a
		// deterministic placeholder — VerifyCrossLink authenticates the
		// signature over (epoch, hash, blockNum, viewID), not block existence.
		if xl := shard1CrossLinkNum(spec, number); xl != 0 {
			clHash := crypto.Keccak256Hash([]byte(fmt.Sprintf("hmy-metadata-fixture-shard1-crosslink-%d", xl)))
			cl, err := c.makeShard1CrossLink(hdr.Epoch(), xl, xl, clHash)
			if err != nil {
				return fmt.Errorf("fixture: shard-1 crosslink at %d: %w", number, err)
			}
			crossLinks = types.CrossLinks{*cl}
		}

		commitSigs := make(chan []byte, 1)
		if len(lastSig) > 0 {
			commitSigs <- lastSig
		} else {
			commitSigs <- []byte{}
		}
		viewID := number
		blk, err := worker.FinalizeNewBlock(commitSigs, func() uint64 { return viewID }, coinbase, crossLinks, nextShardState)
		if err != nil {
			return fmt.Errorf("fixture: finalize %d: %w", number, err)
		}
		sig, err := c.signCommit(blk)
		if err != nil {
			return err
		}
		blk.SetCurrentCommitSig(sig)
		if err := c.Bc.ValidateNewBlock(blk, c.Bc); err != nil {
			return fmt.Errorf("fixture: ValidateNewBlock(%d): %w", number, err)
		}
		if _, err := c.Bc.InsertChain(types.Blocks{blk}, true); err != nil {
			return fmt.Errorf("fixture: InsertChain(%d): %w", number, err)
		}
		lastSig = sig
	}
	return nil
}

// makeShard1IncomingReceipt builds a GENUINE incoming cross-shard receipt
// proof from shard 1 to shard 0 that passes the FULL ValidateCXReceiptsProof
// chain: a real shard-1 source header whose OutgoingReceiptHash commits to
// the receipt root, a matching merkle proof, and a commit signature from
// shard-1's dev committee for the header's epoch (quorum bitmap). The
// receipt credits `to` with `amount`, and the caller must apply it to the
// proposal state (worker.CommitReceipts) so the block's root includes the
// credit and the audit's re-execution reproduces it exactly.
func (c *Chain) makeShard1IncomingReceipt(epoch *big.Int, srcBlockNum, viewID uint64, to common.Address, amount *big.Int) (*types.CXReceiptsProof, error) {
	receipts := types.CXReceipts{{
		TxHash:    crypto.Keccak256Hash([]byte("hmy-metadata-fixture-cx-tx")),
		From:      to,
		To:        &to,
		ShardID:   1,
		ToShardID: 0,
		Amount:    new(big.Int).Set(amount),
	}}
	shardRoot := types.DeriveSha(receipts)
	// Outgoing receipt hash per CXMerkleProof: keccak(be32(toShard) || root)
	// over the ordered destination-shard list (here just shard 0).
	sKey := []byte{0, 0, 0, 0}
	outHash := crypto.Keccak256Hash(append(sKey, shardRoot.Bytes()...))

	hdr := blockfactory.NewFactory(c.Bc.Config()).NewHeader(new(big.Int).Set(epoch))
	hdr.SetShardID(1)
	hdr.SetNumber(new(big.Int).SetUint64(srcBlockNum))
	hdr.SetViewID(new(big.Int).SetUint64(viewID))
	hdr.SetOutgoingReceiptHash(outHash)

	ss, err := rawdb.ReadShardState(c.DB, epoch)
	if err != nil {
		return nil, fmt.Errorf("fixture: read shard state epoch %d for incoming receipt: %w", epoch, err)
	}
	comm, err := ss.FindCommitteeByID(1)
	if err != nil {
		return nil, fmt.Errorf("fixture: no shard-1 committee at epoch %d: %w", epoch, err)
	}
	keys, err := c.loadKeysForSlots(comm.Slots)
	if err != nil {
		return nil, err
	}
	pubs := make([]bls2.PublicKeyWrapper, len(keys))
	for i, k := range keys {
		pubs[i] = *k.Pub
	}
	mask := bls2.NewMask(pubs)
	payload := consensus_sig.ConstructCommitPayload(
		c.Bc.Config(), epoch, hdr.Hash(), srcBlockNum, viewID,
	)
	var agg bls_core.Sign
	for i, k := range keys {
		if err := mask.SetBit(i, true); err != nil {
			return nil, err
		}
		agg.Add(k.Pri.SignHash(payload))
	}
	cxp := &types.CXReceiptsProof{
		Receipts: receipts,
		MerkleProof: &types.CXMerkleProof{
			BlockNum:      new(big.Int).SetUint64(srcBlockNum),
			BlockHash:     hdr.Hash(),
			ShardID:       1,
			CXReceiptHash: outHash,
			ShardIDs:      []uint32{0},
			CXShardHashes: []common.Hash{shardRoot},
		},
		Header:       hdr,
		CommitSig:    agg.Serialize(),
		CommitBitmap: mask.Mask(),
	}
	// Self-check against the live chain: the fixture must never emit a proof
	// that would not survive the full ValidateCXReceiptsProof chain.
	if err := core.NewBlockValidator(c.Bc).ValidateCXReceiptsProof(cxp); err != nil {
		return nil, fmt.Errorf("fixture: constructed incoming receipt does not verify: %w", err)
	}
	return cxp, nil
}

// writeShard1CrossLinkFor persists the shard-1 crosslink record for the
// incoming receipt's source block, carrying the proof's TRUE commit
// signature material. See the call site in Generate for why this must come
// from the in-memory proof rather than the stored body.
func (c *Chain) writeShard1CrossLinkFor(cxp *types.CXReceiptsProof) error {
	var sig [96]byte
	copy(sig[:], cxp.CommitSig)
	cl := &types.CrossLink{
		HashF:        cxp.Header.Hash(),
		BlockNumberF: new(big.Int).Set(cxp.Header.Number()),
		ViewIDF:      new(big.Int).Set(cxp.Header.ViewID()),
		SignatureF:   sig,
		BitmapF:      append([]byte(nil), cxp.CommitBitmap...),
		ShardIDF:     cxp.Header.ShardID(),
		EpochF:       new(big.Int).Set(cxp.Header.Epoch()),
	}
	return rawdb.WriteCrossLinkShardBlock(
		c.DB, cl.ShardID(), cl.BlockNum(), cl.Serialize())
}

// shard1CrossLinkNum returns the shard-1 crosslink block number to propose at
// beacon height number (0 = none): pre-target block → shard-1 block 3,
// post-target block → shard-1 block 4 (see Spec.PreCrossLinkShard1At).
func shard1CrossLinkNum(spec Spec, number uint64) uint64 {
	switch {
	case spec.PreCrossLinkShard1At != 0 && number == spec.PreCrossLinkShard1At:
		return 3
	case spec.CrossLinkShard1At != 0 && number == spec.CrossLinkShard1At:
		return 4
	default:
		return 0
	}
}

// Finalize commits preimages and closes the database cleanly.
func (c *Chain) Finalize() error {
	head := c.Bc.CurrentBlock().NumberU64()
	if err := c.Bc.CommitPreimages(); err != nil {
		return fmt.Errorf("fixture: commit preimages: %w", err)
	}
	if _, _, err := rawdb.WritePreImageStartEndBlock(c.DB, 1, head); err != nil {
		return fmt.Errorf("fixture: preimage markers: %w", err)
	}
	if err := c.DB.Close(); err != nil {
		return fmt.Errorf("fixture: close: %w", err)
	}
	return nil
}

type ecdsaSigner struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

// deterministicBLSSecret derives a fixture-only BLS secret from a 32-byte
// little-endian scalar keyed by the address (the pinned SecretKey
// SetLittleEndian performs no modular reduction), so fixtures are
// byte-reproducible. Test-only; never a real key.
func deterministicBLSSecret(seed common.Address) *bls_core.SecretKey {
	var buf [32]byte
	h := crypto.Keccak256([]byte("hmy-metadata-fixture-bls/"), seed.Bytes())
	copy(buf[:], h)
	buf[31] = 0 // keep within the field for the pinned lib
	sec := &bls_core.SecretKey{}
	if err := sec.SetLittleEndian(buf[:]); err != nil {
		panic("fixture: bls scalar: " + err.Error())
	}
	return sec
}

// RepoKeysDir locates the repo's .hmy directory from the current working
// directory (test-time helper).
func RepoKeysDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ".hmy"
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".hmy")); err == nil {
			return filepath.Join(dir, ".hmy")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ".hmy"
}

// WriteAnchorConfig builds a localnet anchor config for the given target
// from a CLOSED fixture chain directory and writes it as JSON to path.
// knownBad lists heights expected to fail validity checks during the audit
// (empty for clean fixtures: they plant no exploit block).
func WriteAnchorConfig(dir string, target, auditEnd uint64, knownBad []uint64, path string) error {
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", true)
	if err != nil {
		return fmt.Errorf("fixture: open for anchor: %w", err)
	}
	defer db.Close()
	targetHash := rawdb.ReadCanonicalHash(db, target)
	childHash := rawdb.ReadCanonicalHash(db, target+1)
	sched := shard.Schedule
	epoch := sched.CalcEpochNumber(target).Uint64()
	cfg := recoveryanchor.Config{
		Schema:             recoveryanchor.Schema,
		Network:            "localnet",
		Shard:              0,
		TargetHeight:       target,
		TargetHash:         targetHash.Hex(),
		AbandonedChildHash: childHash.Hex(),
		Epoch:              epoch,
		EpochFirstBlock:    sched.EpochLastBlock(epoch-1) + 1,
		EpochLastBlock:     sched.EpochLastBlock(epoch),
		SnapshotBaseHeight: sched.EpochLastBlock(epoch-1) - 1,
		AuditEndHeight:     auditEnd,
		KnownBadBlocks:     knownBad,
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// Canonicalize rewrites a closed fixture database into a byte-reproducible
// canonical form (mirrors the preflight fixture, WS7): LevelDB embeds
// per-write sequence numbers in its tables, so the physical bytes depend on
// the write ORDER, which upstream map iteration makes nondeterministic even
// for identical logical content. Re-inserting every key-value pair in
// sorted key order makes the tables and manifest a pure function of the
// content; the timestamped LOG file is dropped. Two generations of the same
// spec are byte-identical afterwards (the committed kit is diffable).
func Canonicalize(dir string) error {
	src, err := leveldb.OpenFile(dir, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return fmt.Errorf("fixture: canonicalize open source: %w", err)
	}
	tmp := dir + ".canonical"
	if err := os.RemoveAll(tmp); err != nil {
		src.Close()
		return err
	}
	dst, err := leveldb.OpenFile(tmp, &opt.Options{ErrorIfExist: true})
	if err != nil {
		src.Close()
		return fmt.Errorf("fixture: canonicalize open dest: %w", err)
	}
	it := src.NewIterator(nil, nil)
	batch := new(leveldb.Batch)
	n := 0
	fail := func(err error) error {
		it.Release()
		src.Close()
		dst.Close()
		return err
	}
	for it.Next() {
		batch.Put(append([]byte(nil), it.Key()...), append([]byte(nil), it.Value()...))
		if n++; n%1024 == 0 {
			if err := dst.Write(batch, nil); err != nil {
				return fail(err)
			}
			batch.Reset()
		}
	}
	if err := it.Error(); err != nil {
		return fail(fmt.Errorf("fixture: canonicalize iterate: %w", err))
	}
	it.Release()
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
		return fmt.Errorf("fixture: canonicalize compact: %w", err)
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

// CopyDir snapshots a closed chain directory (twin/junk copies).
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
