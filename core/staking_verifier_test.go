package core

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/types"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	testBankKey, _    = crypto.GenerateKey()
	testBankAddress   = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds     = big.NewInt(8000000000000000000)
	validatorAddress  = common.Address(common2.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
	validatorAddress1 = common.Address(common2.MustBech32ToAddress("one1pf75h0t4am90z8uv3y0dgunfqp4lj8wr3t5rsp"))
	delegatorAddr     = common.Address(common2.MustBech32ToAddress("one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7"))
	testBlsPubKey1    = "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"
	testBlsPrvKey1    = "74f322a7378686b9d16ffb9171e762e926e9c0f8e3f95c2982ecb8a5338c1014"
	testBlsPubKey2    = "ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296"
	testBlsPrvKey2    = "54fc6962e78b353936ca99918566cb66bae3a8aa960127af4a866967b471736e"
	localStateDB, _   = state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	chainConfig       = params.TestChainConfig
	blockFactory      = blockfactory.ForTest
	localChain, _     = createNewBlockchain()
	batch             = localChain.db.NewBatch()
	// string with 140 characters
	longStr = "Helloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	// string > 140 characters
	longStr1 = "adsfwryuiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfhuewhfiuewhfe2"
	// string > 280 characters
	longStr2 = "adsfwryuiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfhuewhfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiue2"
	// string with 280 characters
	veryLongStr           = "adsfwryuiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfhuewhfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiuewhfehfiue"
	zeroPointFiveRate, _  = numeric.NewDecFromStr("0.5")
	zeroPointSixRate, _   = numeric.NewDecFromStr("0.6")
	oneDec, _             = numeric.NewDecFromStr("1")
	zeroDec, _            = numeric.NewDecFromStr("0")
	onePointZeroOneDec, _ = numeric.NewDecFromStr("1.01")
	zeroPointOneFive      = numeric.MustNewDecFromStr("0.15")
	zeroPointOneSix       = numeric.MustNewDecFromStr("0.16")
)

func createNewBlockchain() (*BlockChain, error) {
	var (
		database = ethdb.NewMemDatabase()
		gspec    = Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)
	genesis := gspec.MustCommit(database)
	_ = genesis
	return NewBlockChain(database, nil, gspec.Config, chain.Engine, vm.Config{}, nil)
}

func generateBlsKeySigPair(priKey, pubKey string) (shard.BlsPublicKey, shard.BLSSignature) {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(pubKey)
	pub := shard.BlsPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BlsVerificationStr)
	privateKey := &bls.SecretKey{}
	privateKey.DeserializeHexStr(priKey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig shard.BLSSignature
	copy(sig[:], signature.Serialize())
	return pub, sig
}

func createValidator() *staking.CreateValidator {
	desc := staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
	rate, _ := numeric.NewDecFromStr("0.1")
	maxRate := zeroPointFiveRate
	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
	commission := staking.CommissionRates{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
	}
	minSelfDel := big.NewInt(1e18)
	maxTotalDel := big.NewInt(9e18)
	pubKey, pubSig := generateBlsKeySigPair(testBLSPrvKey, testBLSPubKey)
	slotPubKeys := []shard.BlsPublicKey{pubKey}
	slotKeySigs := []shard.BLSSignature{pubSig}
	amount := big.NewInt(5e18)
	v := staking.CreateValidator{
		ValidatorAddress:   validatorAddress,
		Description:        desc,
		CommissionRates:    commission,
		MinSelfDelegation:  minSelfDel,
		MaxTotalDelegation: maxTotalDel,
		SlotPubKeys:        slotPubKeys,
		SlotKeySigs:        slotKeySigs,
		Amount:             amount,
	}
	return &v
}

// Test CV1: create validator
func TestCV1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV3: validator already exists
func TestCV3(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SetValidatorFlag(msg.ValidatorAddress)

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); !strings.Contains(err.Error(), errValidatorExist.Error()) {
		t.Error("expected", errValidatorExist, "got", err)
	}
}

// Test CV4: name longer than 140 characters
func TestCV4(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	msg.Name = longStr1
	namelengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Name), "maxNameLen", staking.MaxNameLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != namelengthCtxError.Message() {
		t.Error("expected", namelengthCtxError, "got", err)
	}
}

// Test CV5: identity longer than 140 characters
func TestCV5(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	msg.Identity = longStr1
	identitylengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Identity), "maxIdentityLen", staking.MaxIdentityLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != identitylengthCtxError.Message() {
		t.Error("expected", identitylengthCtxError, "got", err)
	}
}

// Test CV6: website longer than 140 characters
func TestCV6(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	msg.Website = longStr1
	websiteLengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Website), "maxWebsiteLen", staking.MaxWebsiteLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != websiteLengthCtxError.Message() {
		t.Error("expected", websiteLengthCtxError, "got", err)
	}
}

// Test CV7: security contact longer than 140 characters
func TestCV7(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	msg.SecurityContact = longStr1
	securityContactLengthError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.SecurityContact), "maxSecurityContactLen", staking.MaxSecurityContactLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != securityContactLengthError.Message() {
		t.Error("expected", securityContactLengthError, "got", err)
	}
}

// Test CV8: details longer than 280 characters
func TestCV8(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	msg.Details = longStr2
	detailsLenCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Details), "maxDetailsLen", staking.MaxDetailsLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err == nil {
		t.Errorf("Expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != detailsLenCtxError.Message() {
		t.Error("expected", detailsLenCtxError, "got", err)
	}
}

// Test CV9: name == 140 characters
func TestCV9(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// name length: 140 characters
	msg.Name = longStr
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV10: identity == 140 characters
func TestCV10(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// identity length: 140 characters
	msg.Identity = longStr
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV11: website == 140 characters
func TestCV11(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// website length: 140 characters
	msg.Website = longStr
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV12: security == 140 characters
func TestCV12(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// security contact length: 140 characters
	msg.SecurityContact = longStr
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV13: details == 280 characters
func TestCV13(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// details length: 280 characters
	msg.Details = veryLongStr
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV14: commission rate <= max rate & max change rate <= max rate
func TestCV14(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate == max rate &&  max change rate == max rate
	msg.CommissionRates.Rate = zeroPointFiveRate
	msg.CommissionRates.MaxChangeRate = zeroPointFiveRate
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV15: commission rate > max rate
func TestCV15(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate: 0.6 > max rate: 0.5
	msg.CommissionRates.Rate = zeroPointSixRate
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "commission rate and change rate can not be larger than max commission rate", "got", nil)
	}
}

// Test CV16: max change rate > max rate
func TestCV16(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max change rate: 0.6 > max rate: 0.5
	msg.CommissionRates.MaxChangeRate = zeroPointSixRate
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "commission rate and change rate can not be larger than max commission rate", "got", nil)
	}
}

// Test CV17: max rate == 1
func TestCV17(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max rate == 1
	msg.CommissionRates.MaxRate = oneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV18: max rate == 1 && max change rate == 1 && commission rate == 0
func TestCV18(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max rate == 1 && max change rate == 1 && commission rate == 0
	msg.CommissionRates.MaxRate = oneDec
	msg.CommissionRates.MaxChangeRate = oneDec
	msg.CommissionRates.Rate = oneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV19: commission rate == 0
func TestCV19(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate == 0
	msg.CommissionRates.Rate = zeroDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV20: max change rate == 0
func TestCV20(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate == 0
	msg.CommissionRates.MaxChangeRate = zeroDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV21: max change rate == 0 & max rate == 0 & commission rate == 0
func TestCV21(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max change rate == 0 & max rate == 0 & commission rate == 0
	msg.CommissionRates.MaxRate = zeroDec
	msg.CommissionRates.MaxChangeRate = zeroDec
	msg.CommissionRates.Rate = zeroDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV22: max change rate == 1 & max rate == 1
func TestCV22(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max change rate == 1 & max rate == 1
	msg.CommissionRates.MaxRate = oneDec
	msg.CommissionRates.MaxChangeRate = oneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV23: commission rate < 0
func TestCV23(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate < 0
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("-0.1")
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:-0.100000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV24: max rate < 0
func TestCV24(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max rate < 0
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("-0.001")
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:-0.001000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV25: max change rate < 0
func TestCV25(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max rate < 0
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("-0.001")
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:-0.001000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV26: commission rate > 1
func TestCV26(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// commission rate > 1
	msg.CommissionRates.Rate = onePointZeroOneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV27: max rate > 1
func TestCV27(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max rate > 1
	msg.CommissionRates.MaxRate = onePointZeroOneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV28: max change rate > 1
func TestCV28(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// max change rate > 1
	msg.CommissionRates.MaxChangeRate = onePointZeroOneDec
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV29: amount > MinSelfDelegation
func TestCV29(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// amount > MinSelfDelegation
	msg.Amount = big.NewInt(4e18)
	msg.MinSelfDelegation = big.NewInt(1e18)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV30: amount == MinSelfDelegation
func TestCV30(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// amount > MinSelfDelegation
	msg.Amount = big.NewInt(4e18)
	msg.MinSelfDelegation = big.NewInt(4e18)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV31: amount < MinSelfDelegation
func TestCV31(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// amount > MinSelfDelegation
	msg.Amount = big.NewInt(4e18)
	msg.MinSelfDelegation = big.NewInt(5e18)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "have 4000000000000000000 want 5000000000000000000: self delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test CV32: MaxTotalDelegation < MinSelfDelegation
func TestCV32(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// MaxTotalDelegation < MinSelfDelegation
	msg.MaxTotalDelegation = big.NewInt(2e18)
	msg.MinSelfDelegation = big.NewInt(3e18)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "max_total_delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test CV33: MinSelfDelegation < 1 ONE
func TestCV33(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// MinSelfDelegation < 1 ONE
	msg.MinSelfDelegation = big.NewInt(1e18 - 1)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "delegation-given 999999999999999999: min_self_delegation has to be greater than 1 ONE", "got", nil)
	}
}

// Test CV34: MinSelfDelegation not specified
func TestCV34(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// MinSelfDelegation not specified
	msg.MinSelfDelegation = nil
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "MinSelfDelegation can not be nil", "got", nil)
	}
}

// Test CV35: MinSelfDelegation < 0
func TestCV35(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// MinSelfDelegation < 0
	msg.MinSelfDelegation = big.NewInt(-1)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "delegation-given -1: min_self_delegation has to be greater than 1 ONE", "got", nil)
	}
}

// Test CV36: amount > MaxTotalDelegation
func TestCV36(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// amount > MaxTotalDelegation
	msg.Amount = big.NewInt(4e18)
	msg.MaxTotalDelegation = big.NewInt(3e18)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "total delegation can not be bigger than max_total_delegation", "got", nil)
	}
}

// Test CV39: MaxTotalDelegation < 0
func TestCV39(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	// MaxTotalDelegation < 0
	msg.MaxTotalDelegation = big.NewInt(-1)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "max_total_delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test CV40: two valid bls keys specified
func TestCV40(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	blsPubKey1, blsPubSig1 := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	blsPubKey2, blsPubSig2 := generateBlsKeySigPair(testBlsPrvKey2, testBlsPubKey2)
	msg.SlotPubKeys = []shard.BlsPublicKey{blsPubKey1, blsPubKey2}
	msg.SlotKeySigs = []shard.BLSSignature{blsPubSig1, blsPubSig2}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV41: no bls key specified
func TestCV41(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	msg.SlotPubKeys = []shard.BlsPublicKey{}
	msg.SlotKeySigs = []shard.BLSSignature{}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "need at least one slot key", "got", nil)
	}
}

// Test CV42: bls key signature not verifiable
func TestCV42(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	blsPubKey1, blsPubSig1 := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey2)
	msg.SlotPubKeys = []shard.BlsPublicKey{blsPubKey1}
	msg.SlotKeySigs = []shard.BLSSignature{blsPubSig1}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "bls keys and corresponding signatures could not be verified", "got", err)
	}
}

// Test CV43: duplicate bls keys (self)
func TestCV43(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	blsPubKey1, blsPubSig1 := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	msg.SlotPubKeys = []shard.BlsPublicKey{blsPubKey1, blsPubKey1}
	msg.SlotKeySigs = []shard.BLSSignature{blsPubSig1, blsPubSig1}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "slot keys can not have duplicates", "got", err)
	}
}

// Test CV44: duplicate bls keys (with other's key)
func TestCV44(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	blsPubKey1, blsPubSig1 := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	msg.SlotPubKeys = []shard.BlsPublicKey{blsPubKey1}
	msg.SlotKeySigs = []shard.BLSSignature{blsPubSig1}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	wrapper, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if err := statedb.UpdateValidatorWrapper(wrapper.Validator.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SetValidatorFlag(wrapper.Validator.Address)

	msg = createValidator()
	msg.ValidatorAddress = validatorAddress1
	msg.SlotPubKeys = []shard.BlsPublicKey{blsPubKey1}
	msg.SlotKeySigs = []shard.BLSSignature{blsPubSig1}
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV45: negative amount
func TestCV45(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	msg.Amount = big.NewInt(-5e18)
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "amount can not be negative", "got", err)
	}
}

// Test CV46: amount > balance
func TestCV46(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	msg.Amount = big.NewInt(6e18)
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))

	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err == nil {
		t.Error("expected", "insufficient balance to stake", "got", err)
	}
}

// Test CV47: specify small gas limit to cause "txn out of gas"
func TestCV47(t *testing.T) {
	// TODO:
}

func createValidatorAndCommit(statedb *state.DB) error {
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	wrapper, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(1), msg)
	if err != nil {
		return err
	}
	commitWrapper(statedb, wrapper)
	statedb.SubBalance(wrapper.Address, msg.Amount)
	return nil
}

func createValidatorAndCommitInActive(statedb *state.DB) error {
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	wrapper, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(1), msg)
	if err != nil {
		return err
	}
	wrapper.Validator.EPOSStatus = effective.Inactive
	commitWrapper(statedb, wrapper)
	statedb.SubBalance(wrapper.Address, msg.Amount)
	return nil
}

func commitWrapper(statedb *state.DB, wrapper *staking.ValidatorWrapper) error {
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		return err
	}
	statedb.SetValidatorFlag(wrapper.Address)
	if err := rawdb.WriteValidatorSnapshot(batch, wrapper, localChain.CurrentBlock().Epoch()); err != nil {
		return err
	}
	batch.Write()
	return nil
}

func createEditValidator() *staking.EditValidator {
	return &staking.EditValidator{
		ValidatorAddress: validatorAddress,
	}
}

// Test EV1: edit name
func TestEV1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Name = "SuperHero1"
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.Name != ev.Description.Name {
		t.Error("edit validator name failed")
	}
}

// Test EV2: edit identity
func TestEV2(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Identity = "YouWouldNotKnow1"
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.Identity != ev.Description.Identity {
		t.Error("edit validator identity failed")
	}
}

// Test EV3: edit website
func TestEV3(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Website = "Secret Website1"
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.Website != ev.Description.Website {
		t.Error("edit validator identity failed")
	}
}

// Test EV4: edit security contact
func TestEV4(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.SecurityContact = "LicenseToKill1"
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.SecurityContact != ev.Description.SecurityContact {
		t.Error("edit validator identity failed")
	}
}

// Test EV5: edit details
func TestEV5(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Details = "blah blah blah blah"
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.Details != ev.Description.Details {
		t.Error("edit validator identity failed")
	}
}

// Test EV6: edit description
func TestEV6(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description = staking.Description{
		Name:            "John",
		Identity:        "john",
		Website:         "john@harmony.one",
		SecurityContact: "leo",
		Details:         "john the validator",
	}
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	wrapper, err = statedb.ValidatorWrapper(ev.ValidatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if wrapper.Description.Name != ev.Description.Name &&
		wrapper.Description.Identity != ev.Description.Identity &&
		wrapper.Description.Website != ev.Description.Website &&
		wrapper.Description.SecurityContact != ev.Description.SecurityContact &&
		wrapper.Description.Details != ev.Description.Details {
		t.Error("edit validator identity failed")
	}
}

// Test EV7: name longer than 140 characters
func TestEV7(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Name = longStr1
	namelengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(ev.Name), "maxNameLen", staking.MaxNameLength)
	_, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != namelengthCtxError.Message() {
		t.Error("expected", namelengthCtxError, "got", err)
	}

}

// Test EV8: identity longer than 140 characters
func TestEV8(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Identity = longStr1

	identitylengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(ev.Identity), "maxIdentityLen", staking.MaxIdentityLength)
	_, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != identitylengthCtxError.Message() {
		t.Error("expected", identitylengthCtxError, "got", err)
	}
}

// Test EV9: website longer than 140 characters
func TestEV9(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Website = longStr1

	websitelengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(ev.Website), "maxWebsiteLen", staking.MaxWebsiteLength)
	_, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != websitelengthCtxError.Message() {
		t.Error("expected", websitelengthCtxError, "got", err)
	}
}

// Test EV10: security contact longer than 140 characters
func TestEV10(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.SecurityContact = longStr1

	securitycontactlengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(ev.SecurityContact), "maxSecurityContactLen", staking.MaxSecurityContactLength)
	_, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != securitycontactlengthCtxError.Message() {
		t.Error("expected", securitycontactlengthCtxError, "got", err)
	}
}

// Test EV11: details longer than 280 characters
func TestEV11(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.Description.Details = longStr2

	detailslengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(ev.SecurityContact), "maxDetailsLen", staking.MaxDetailsLength)
	_, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err == nil {
		t.Errorf("expected non null error")
	}
	ctxerr, ok := err.(ctxerror.CtxError)
	if !ok {
		t.Errorf("expected context aware error")
	}
	if ctxerr.Message() != detailslengthCtxError.Message() {
		t.Error("expected", detailslengthCtxError, "got", err)
	}
}

// Test EV12: edit commission rate
func TestEV12(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.CommissionRate = &zeroPointOneFive
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test EV13: commission rate change > max change rate (within the same epoch)
func TestEV13(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.CommissionRate = &zeroPointOneSix
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "change on commission rate can not be more than max change rate within the same epoch", "got", nil)
	}
}

// Test EV14: edit commission rate
func TestEV14(t *testing.T) {
	// TODO: cannot be unit tested
}

// Test EV15: commission rate > max rate  (repeatedly change comm-rate so it's > 0.5)
func TestEV15(t *testing.T) {
	// TODO: cannot be unit tested
}

// Test EV16: edit commission in next epoch
func TestEV16(t *testing.T) {
	// TODO: cannot be unit tested
}

// Test EV17: validator address != sender address
func TestEV17(t *testing.T) {
	// TODO: should be part of sign test
}

// Test EV18: editing on non-validator address
func TestEV18(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	ev := createEditValidator()
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "staking validator does not exist", "got", nil)
	}
}

// Test EV19: edit MinSelfDelegation only
func TestEV19(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MinSelfDelegation = big.NewInt(1e18)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test EV20: MinSelfDelegation < 1 ONE
func TestEV20(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MinSelfDelegation = big.NewInt(1e18 - 10)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "delegation-given 999999999999999990: min_self_delegation has to be greater than 1 ONE", "got", nil)
	}
}

// Test EV21: MinSelfDelegation > self delegation
func TestEV21(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MinSelfDelegation = big.NewInt(6e18)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "have 5000000000000000000 want 9000000000000000000: self delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test EV22: MinSelfDelegation < 0
func TestEV22(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MinSelfDelegation = big.NewInt(-1e18)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "delegation-given -1000000000000000000: min_self_delegation has to be greater than 1 ONE", "got", nil)
	}
}

// Test EV23: edit MaxTotalDelegation
func TestEV23(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MaxTotalDelegation = big.NewInt(5e18)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test EV24: amount > MaxTotalDelegation
func TestEV24(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MaxTotalDelegation = big.NewInt(4e18)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "total 5000000000000000000 max-total 4000000000000000000: total delegation can not be bigger than max_total_delegation", "got", nil)
	}
}

// Test EV25: MaxTotalDelegation < MinSelfDelegation
func TestEV25(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	ev.MaxTotalDelegation = big.NewInt(1e18 - 1)
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "max-total-delegation 999999999999999999 min-self-delegation 1000000000000000000: max_total_delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test EV26: add slot key
func TestEV26(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	pubKey, pubSig := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	ev.SlotKeyToAdd = &pubKey
	ev.SlotKeyToAddSig = &pubSig
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test EV27: add slot key that already exists
func TestEV27(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	pubKey, pubSig := generateBlsKeySigPair(testBLSPrvKey, testBLSPubKey)
	ev.SlotKeyToAdd = &pubKey
	ev.SlotKeyToAddSig = &pubSig
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "slot key to add already exists", "got", nil)
	}
}

// Test EV28: remove slot key
func TestEV28(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	pubKey, pubSig := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	ev.SlotKeyToAdd = &pubKey
	ev.SlotKeyToAddSig = &pubSig
	wrapper, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	ev.SlotKeyToAdd = nil
	ev.SlotKeyToAddSig = nil
	pubKey, _ = generateBlsKeySigPair(testBLSPrvKey, testBLSPubKey)
	ev.SlotKeyToRemove = &pubKey
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test EV29: slot key to remove not found
func TestEV29(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ev := createEditValidator()
	pubKey, _ := generateBlsKeySigPair(testBlsPrvKey1, testBlsPubKey1)
	ev.SlotKeyToRemove = &pubKey
	if _, err := VerifyAndEditValidatorFromMsg(statedb, localChain, big.NewInt(0), ev); err == nil {
		t.Error("expected", "slot key to remove not found", "got", nil)
	}
}

// Test EV30: specify small gas limit to cause "txn out of gas"
func TestEV30(t *testing.T) {
	// TODO:
}

func createDelegation() *staking.Delegate {
	return &staking.Delegate{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddress,
		Amount:           big.NewInt(1e18),
	}
}

func createUnDelegation() *staking.Undelegate {
	return &staking.Undelegate{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddress,
		Amount:           big.NewInt(1e18),
	}
}

// Test D1: delegation with user balance
func TestD1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test D2: delegator address != sender address
func TestD2(t *testing.T) {
	// TODO:
}

// Test D3: validator not exist at the validator address
func TestD3(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err == nil {
		t.Error("expected", "staking validator does not exist", "got", nil)
	}
}

// Test D4: negative amount
func TestD4(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	d.Amount = big.NewInt(-1)
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err == nil {
		t.Error("expected", "amount can not be negative", "got", nil)
	}
}

// Test D5: delegation add up to be greater than validator's max total delegation
func TestD5(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	d.Amount = big.NewInt(4e18 + 1)
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(4e18+1))
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err == nil {
		t.Error("expected", "total 9000000000000000001 max-total 9000000000000000000: total delegation can not be bigger than max_total_delegation", "got", nil)
	}
}

// Test D6: insufficient balance for delegation
func TestD6(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err == nil {
		t.Error("expected", "insufficient balance to stake", "got", nil)
	}
}

// Test D7: delegation using pending undelegated token (if there are tokens in undelegation, then delegation to the same validator will first use that)
func TestD7(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	d.Amount = big.NewInt(1e18)
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err != nil {
		t.Error("expected", "insufficient balance to stake", "got", err)
	}
}

// Test D8: delegation using pending undelegated token + user balance
func TestD8(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	d.Amount = big.NewInt(1e18 + 1)
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1))
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err != nil {
		t.Error("expected", "insufficient balance to stake", "got", err)
	}
}

// Test D9: delegation using pending undelegated token + user balance (insufficient)
func TestD9(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	commitWrapper(statedb, wrapper)
	d.Amount = big.NewInt(1e18 + 2)
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1))
	if _, _, err := VerifyAndDelegateFromMsg(statedb, d); err == nil {
		t.Error("expected", "total-delegated 1000000000000000000 own-current-balance 1 amount-to-delegate 1000000000000000002: insufficient balance to stake", "got", nil)
	}
}

// Test D10: specify small gas limit to cause "txn out of gas"
func TestD10(t *testing.T) {
	// TODO:
}

// Test U1: successful undelegation
func TestU1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if err := commitWrapper(statedb, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test U2: successful undelegation from non-selected validator
func TestU2(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommitInActive(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(1e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if err := commitWrapper(statedb, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test U3: delegator address != sender address
func TestU3(t *testing.T) {
	// TODO:
}

// Test U4: validator not exist at the validator address
func TestU4(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	ud := createUnDelegation()
	_, err := VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err == nil {
		t.Error("expected", "staking validator does not exist", "got", nil)
	}
}

// Test U5: negative amount
func TestU5(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommitInActive(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	ud.Amount = big.NewInt(-1e18)
	if _, err := VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud); err == nil {
		t.Error("expected", "amount can not be negative", "got", nil)
	}
}

// Test U6: undelegate a second time in differnet epochs
func TestU6(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	d.Amount = big.NewInt(2e18)
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(3e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if err := commitWrapper(statedb, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	wrapper, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(2), ud)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if err := commitWrapper(statedb, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test U7: insufficient delegation balance for undelegation
func TestU7(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	d := createDelegation()
	statedb.AddBalance(d.DelegatorAddress, big.NewInt(3e18))
	wrapper, balanceToBeDeducted, err := VerifyAndDelegateFromMsg(statedb, d)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SubBalance(d.DelegatorAddress, balanceToBeDeducted)
	if err := statedb.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	ud.Amount = big.NewInt(2e18)
	_, err = VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud)
	if err == nil {
		t.Error("expected", "Insufficient balance to undelegate", "got", nil)
	}
}

// Test U8: no delegation to undelegate
func TestU8(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	ud := createUnDelegation()
	if _, err := VerifyAndUndelegateFromMsg(statedb, big.NewInt(1), ud); err == nil {
		t.Error("expected", "no delegation to undelegate", "got", nil)
	}
}

// Test U9: specify small gas limit to cause "txn out of gas"
func TestU9(t *testing.T) {
	// TODO:
}

func createCollectRewards() *staking.CollectRewards {
	return &staking.CollectRewards{
		DelegatorAddress: delegatorAddr,
	}
}

// Test CR1: rewards collected from reward balance into account balance.
func TestCR1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	cr := createCollectRewards()
	cr.DelegatorAddress = validatorAddress
	wrapper, err := statedb.ValidatorWrapper(cr.DelegatorAddress)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	reward := big.NewInt(1e18)
	wrapper.Delegations[0].Reward = reward
	if err := commitWrapper(statedb, wrapper); err != nil {
		t.Error("expected", nil, "got", err)
	}
	delegation := types.DelegationIndex{
		ValidatorAddress: validatorAddress,
		Index:            0,
	}
	delegations := []types.DelegationIndex{delegation}
	if wrapper, totalRewards, err := VerifyAndCollectRewardsFromDelegation(statedb, delegations); err != nil {
		t.Error("expected", nil, "got", err)
	} else if totalRewards.Cmp(reward) != 0 {
		t.Error("expected collected reward", reward, "got", totalRewards)
	} else {
		if err := commitWrapper(statedb, wrapper[0]); err != nil {
			t.Error("expected", nil, "got", err)
		} else {
			statedb.AddBalance(cr.DelegatorAddress, totalRewards)
			if statedb.GetBalance(validatorAddress).Cmp(reward) != 0 {
				t.Error("expected collected reward of ", reward, "credited to balance", "but it didn't")
			}
		}
	}
}

// Test CR2: delegator address != sender address
func TestCR2(t *testing.T) {
	// TODO:
}

// Test CR3: no rewards to collect
func TestCR3(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	if err := createValidatorAndCommit(statedb); err != nil {
		t.Error("expected", nil, "got", err)
	}
	cr := createCollectRewards()
	cr.DelegatorAddress = validatorAddress
	delegation := types.DelegationIndex{
		ValidatorAddress: cr.DelegatorAddress,
		Index:            0,
	}
	delegations := []types.DelegationIndex{delegation}
	if _, _, err := VerifyAndCollectRewardsFromDelegation(statedb, delegations); err == nil {
		t.Error("expected", "no rewards to collect", "got", nil)
	}
}

// Test CR4: specify small gas limit to cause "txn out of gas"
func TestCR4(t *testing.T) {
	// TODO:
}

// Test CR5: rewards collected after being fully undelegated
func TestCR5(t *testing.T) {
	// TODO: need simulation to test
}
