// Package fixture generates localnet-derived test chains with REAL BLS
// commit certificates from the well-known public dev keys (.hmy/*.key,
// empty passphrase) — the committed fixture kit for the recovery tool tests
// (plan WS8). Blocks are produced through the stock worker + processor and
// inserted through the stock ValidateNewBlock/InsertChain path, so fixture
// chains are replay-grade by construction.
//
// Never used in production; fixtures are localnet-only and leak no secrets
// (the dev keys are public test keys).
package fixture

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"
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
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/utils"
	pkgworker "github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/crypto"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
)

// LocalnetTestKeyHex is the committed localnet-only funded test key
// (core/genesis.go:138).
const LocalnetTestKeyHex = "1f84c95ac16e6a50f08d44c7bde7aff8742212fda6e4321fde48bf83bef266dc"

// FaucetContractBinary deploys a small contract with constructor storage
// and a mapping written on call — coverage for code + storage-trie paths
// (same fixture contract as test/chain/main.go).
const FaucetContractBinary = "0x60806040526706f05b59d3b2000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a578063b69ef8a81461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058206b894c1f3badf3b26a7a2768ab8141b1e6fa1c1ddc4622f4f44a7d5041edc9350029"

// Params configures chain generation.
type Params struct {
	Dir     string // chain database directory (created if missing)
	KeysDir string // repo .hmy directory holding <blspub>.key files
	Blocks  uint64 // total blocks to produce (beyond current head)
	TxEvery uint64 // send a funded transfer every N blocks (0 = never)
	// DeployContractAt deploys the faucet contract at this block number and
	// calls it two blocks later (0 = never). Creates code + storage.
	DeployContractAt uint64
	// CreateValidatorAt sends a create-validator staking transaction from
	// the funded contract-deployer account at this block (0 = never; the
	// block must be in the pre-staking epoch or later — localnet epoch 1+,
	// block 5+). The validator's generated BLS secret is persisted next to
	// the chain directory so the fixture can keep signing if the validator
	// is elected into the shard-0 committee at the next epoch boundary
	// (round 13 finding 9: populated validator list/delegations/snapshots).
	CreateValidatorAt uint64
	// DelegateAt sends a delegate staking transaction from the localnet
	// test account to the fixture validator (0 = never; requires a prior
	// CreateValidatorAt).
	DelegateAt uint64
}

// Chain is an open fixture chain.
type Chain struct {
	DB      ethdb.Database
	Bc      core.BlockChain
	Keys    []bls2.PrivateKeyWrapper // shard-0 committee secrets, slot order
	Slots   shard.SlotList
	KeysDir string

	// ValidatorAddr is the fixture validator's address once
	// CreateValidatorAt has run (the contract-deployer account).
	ValidatorAddr common.Address

	contractAddr common.Address
	// extraKeysDir persists fixture-generated BLS secrets (hex, one file
	// per <pub>.hex) so a reopened chain can sign for committees that
	// include the fixture validator.
	extraKeysDir string
}

// Open opens (or initializes) the fixture chain database and loads the
// shard-0 committee secrets for the CURRENT epoch.
func Open(dir, keysDir string) (*Chain, error) {
	if _, err := harness.InitSchedule("localnet"); err != nil {
		return nil, err
	}
	nodeconfig.SetNetworkType(nodeconfig.Localnet)
	utils.SetLogVerbosity(2) // warn: keep fixture generation quiet

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
	cfg, err := harness.ChainConfig("localnet", 0)
	if err != nil {
		db.Close()
		return nil, err
	}
	bc, err := core.NewBlockChainWithOptions(
		db, nil, nil,
		&core.CacheConfig{Disabled: true, Preimages: true, SnapshotLimit: 0},
		cfg, chain.NewEngine(), vm.Config{}, core.Options{},
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
	if err := c.loadCommittee(); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

func (c *Chain) loadCommittee() error {
	// The committee that signs the NEXT block (head+1): at an epoch
	// boundary this differs from the head's committee (elected staking
	// committees, localnet epoch 2+), so resolve the epoch from the next
	// block number, not from the head header.
	epoch := shard.Schedule.CalcEpochNumber(c.Bc.CurrentBlock().NumberU64() + 1)
	ss, err := rawdb.ReadShardState(c.DB, epoch)
	if err != nil {
		return fmt.Errorf("fixture: read shard state epoch %d: %w", epoch, err)
	}
	comm, err := ss.FindCommitteeByID(0)
	if err != nil {
		return err
	}
	c.Slots = comm.Slots
	c.Keys = nil
	keys, err := SlotKeys(comm.Slots, c.KeysDir, c.extraKeysDir)
	if err != nil {
		return err
	}
	c.Keys = keys
	return nil
}

// ExtraKeysDir returns the sidecar directory persisting fixture-generated
// BLS secrets for a chain directory.
func ExtraKeysDir(chainDir string) string {
	return filepath.Join(filepath.Dir(chainDir), "fixture-keys")
}

// SlotKeys loads the BLS secrets for the given committee slots, in slot
// order: repo dev keys from keysDir, fixture-generated validator keys from
// extraKeysDir (plain hex sidecars — fixtures are test-only).
func SlotKeys(slots shard.SlotList, keysDir, extraKeysDir string) ([]bls2.PrivateKeyWrapper, error) {
	var keys []bls2.PrivateKeyWrapper
	for _, slot := range slots {
		keyFile := filepath.Join(keysDir, slot.BLSPublicKey.Hex()+".key")
		sec, err := blsgen.LoadBLSKeyWithPassPhrase(keyFile, "")
		if err != nil {
			raw, rerr := os.ReadFile(filepath.Join(extraKeysDir, slot.BLSPublicKey.Hex()+".hex"))
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

// makeCreateValidatorTx builds the fixture validator's create-validator
// staking transaction from the contract-deployer account, generating a
// fresh BLS slot key and persisting its secret for later committee signing.
func (c *Chain) makeCreateValidatorTx(nonce uint64) (*staking.StakingTransaction, error) {
	// DETERMINISTIC slot key (not random): the stock reward path caches
	// voting-power rosters in a process-global LRU keyed by (epoch, shard)
	// (internal/chain/reward.go lookupVotingPower). Multiple independent
	// fixture chains generated in one test process must therefore elect
	// byte-identical committees, or one chain's cached roster poisons
	// another's reward computation. 31 bytes keeps the scalar below the
	// BLS12-381 group order.
	seed := crypto.Keccak256([]byte("harmony-recoverydb fixture validator BLS slot key"))[:31]
	blsSec := &bls_core.SecretKey{}
	if err := blsSec.SetLittleEndian(seed); err != nil {
		return nil, err
	}
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
	if err := os.WriteFile(filepath.Join(c.extraKeysDir, pub.Hex()+".hex"),
		[]byte(blsSec.SerializeToHexStr()), 0o644); err != nil {
		return nil, err
	}

	one := big.NewInt(denominations.One)
	rate, _ := numeric.NewDecFromStr("0.1")
	maxRate, _ := numeric.NewDecFromStr("0.9")
	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCreateValidator, staking.CreateValidator{
			ValidatorAddress: c.ValidatorAddr,
			Description: staking.Description{
				Name: "fixture-validator", Identity: "fixture",
				Website: "https://fixture.invalid", SecurityContact: "fixture",
				Details: "recoverydb test fixture validator",
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
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), coregenesis.ContractDeployerKey)
}

// makeDelegateTx builds a delegation from the localnet test account to the
// fixture validator.
func (c *Chain) makeDelegateTx(nonce uint64, delegator *ecdsa.PrivateKey) (*staking.StakingTransaction, error) {
	one := big.NewInt(denominations.One)
	maker := func() (staking.Directive, interface{}) {
		return staking.DirectiveDelegate, staking.Delegate{
			DelegatorAddress: crypto.PubkeyToAddress(delegator.PublicKey),
			ValidatorAddress: c.ValidatorAddr,
			Amount:           new(big.Int).Mul(big.NewInt(1_000), one),
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, 10_000_000, big.NewInt(100e9), maker)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(c.Bc.Config().ChainID), delegator)
}

// signCommit aggregates a full-committee commit certificate over the block.
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

// Generate produces p.Blocks new blocks on top of the current head.
func (c *Chain) Generate(p Params) error {
	testKey, err := crypto.HexToECDSA(LocalnetTestKeyHex)
	if err != nil {
		return err
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	head := c.Bc.CurrentBlock()
	lastSig := head.GetCurrentCommitSig()
	if head.NumberU64() == 0 {
		lastSig = nil
	} else if len(lastSig) == 0 {
		// A reopened chain's head block object carries no in-memory sig;
		// read the exact persisted block-sig-N key.
		sig, err := c.DB.Get(append([]byte("block-sig-"), uint64BE(head.NumberU64())...))
		if err != nil {
			return fmt.Errorf("fixture: read head commit sig: %w", err)
		}
		lastSig = sig
	}
	sched := shard.Schedule

	for produced := uint64(0); produced < p.Blocks; produced++ {
		number := c.Bc.CurrentBlock().NumberU64() + 1
		// Committee may change at epoch boundaries; reload from the shard
		// state the previous block established.
		if err := c.loadCommittee(); err != nil {
			return err
		}
		leaderSlot := c.Slots[0]
		leaderKey := c.Keys[0]

		worker := pkgworker.New(c.Bc, c.Bc)
		hdr := worker.GetCurrentHeader()

		// Post-staking, the coinbase is the address DERIVED from the
		// leader's BLS public key (internal/chain/engine.go:252-268);
		// pre-staking it is the slot's ECDSA address.
		coinbase := leaderSlot.EcdsaAddress
		if c.Bc.Config().IsStaking(hdr.Epoch()) {
			coinbase = utils.GetAddressFromBLSPubKeyBytes(leaderSlot.BLSPublicKey[:])
		}

		// VRF, signed by the coinbase slot's BLS key (VerifyVRF contract).
		if c.Bc.Config().IsVRF(hdr.Epoch()) {
			sk := vrf_bls.NewVRFSigner(leaderKey.Pri)
			parentHash := c.Bc.CurrentBlock().Hash()
			vrf, proof := sk.Evaluate(parentHash[:])
			if proof == nil {
				return fmt.Errorf("fixture: VRF generation failed at %d", number)
			}
			hdr.SetVrf(append(vrf[:], proof...))
		}

		// Optional funded transactions to exercise tx/receipt/lookup, code
		// and storage paths.
		var pending types.Transactions
		st, err := c.Bc.State()
		if err != nil {
			return err
		}
		nonce := st.GetNonce(testAddr)
		signer := types.NewEIP155Signer(c.Bc.Config().ChainID)
		if p.TxEvery > 0 && number%p.TxEvery == 0 {
			tx, err := types.SignTx(
				types.NewTransaction(nonce, testAddr, 0, big.NewInt(1000), 21000, big.NewInt(100e9), nil),
				signer, testKey,
			)
			if err != nil {
				return err
			}
			pending = append(pending, tx)
			nonce++
		}
		if p.DeployContractAt != 0 && number == p.DeployContractAt {
			tx, err := types.SignTx(
				types.NewContractCreation(nonce, 0, big.NewInt(1e18), 2_000_000, big.NewInt(100e9), common.FromHex(FaucetContractBinary)),
				signer, testKey,
			)
			if err != nil {
				return err
			}
			c.contractAddr = crypto.CreateAddress(testAddr, nonce)
			pending = append(pending, tx)
			nonce++
		}
		if p.DeployContractAt != 0 && number == p.DeployContractAt+2 {
			// request(address) writes the processed-mapping storage slot.
			data := append(crypto.Keccak256([]byte("request(address)"))[:4], common.LeftPadBytes(testAddr.Bytes(), 32)...)
			tx, err := types.SignTx(
				types.NewTransaction(nonce, c.contractAddr, 0, big.NewInt(0), 2_000_000, big.NewInt(100e9), data),
				signer, testKey,
			)
			if err != nil {
				return err
			}
			pending = append(pending, tx)
			nonce++
		}
		// Staking transactions (round 13 finding 9): a real validator with
		// a delegation, so validator-list/delegation/snapshot paths in the
		// verifier and compactor operate on populated data.
		var pendingStaking staking.StakingTransactions
		if p.CreateValidatorAt != 0 && number == p.CreateValidatorAt {
			stx, err := c.makeCreateValidatorTx(st.GetNonce(c.ValidatorAddr))
			if err != nil {
				return fmt.Errorf("fixture: create-validator tx at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}
		if p.DelegateAt != 0 && number == p.DelegateAt {
			// The delegator is the test account; its nonce accounts for any
			// normal transactions queued in this same block.
			stx, err := c.makeDelegateTx(nonce, testKey)
			if err != nil {
				return fmt.Errorf("fixture: delegate tx at %d: %w", number, err)
			}
			pendingStaking = append(pendingStaking, stx)
		}

		if len(pending) > 0 || len(pendingStaking) > 0 {
			txmap := map[common.Address]types.Transactions{testAddr: pending}
			if err := worker.CommitTransactions(txmap, pendingStaking, coinbase); err != nil {
				return fmt.Errorf("fixture: commit txs at %d: %w", number, err)
			}
		}

		// Next-epoch shard state at the epoch's last block.
		var nextShardState *shard.State
		if sched.IsLastBlock(number) {
			nextShardState, err = c.Bc.SuperCommitteeForNextEpoch(c.Bc, hdr, false)
			if err != nil {
				return fmt.Errorf("fixture: next-epoch committee at %d: %w", number, err)
			}
		}

		commitSigs := make(chan []byte, 1)
		if len(lastSig) > 0 {
			commitSigs <- lastSig
		} else {
			commitSigs <- []byte{}
		}
		viewID := number
		blk, err := worker.FinalizeNewBlock(
			commitSigs, func() uint64 { return viewID }, coinbase,
			nil, nextShardState,
		)
		if err != nil {
			return fmt.Errorf("fixture: finalize block %d: %w", number, err)
		}
		sig, err := c.signCommit(blk)
		if err != nil {
			return err
		}
		blk.SetCurrentCommitSig(sig)

		// Full validation during generation proves the fixture chain is
		// replay-grade (the same path replay-bundle runs).
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

// Finalize commits preimages and closes the database cleanly, leaving a
// full-archival source (the fixture analogue of the Aug 8 copy).
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

// CopyDir snapshots a closed chain directory (baseline checkpoints).
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

func uint64BE(n uint64) []byte {
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = byte(n)
		n >>= 8
	}
	return b
}

// RepoKeysDir locates the repo's .hmy directory from this source file
// (test-time helper).
func RepoKeysDir() string {
	// internal/recoverydb/fixture/fixture.go -> repo root is three levels up.
	_ = time.Now()
	wd, err := os.Getwd()
	if err != nil {
		return ".hmy"
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".hmy")); err == nil {
			return filepath.Join(dir, ".hmy")
		}
		dir = filepath.Dir(dir)
	}
	return ".hmy"
}
