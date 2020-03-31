package core

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	validatorAddress = common.Address(common2.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
	postStakingEpoch = big.NewInt(200)
	tenK             = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	twelveK          = new(big.Int).Mul(big.NewInt(12000), big.NewInt(1e18))
)

func generateBLSKeySigPair() (shard.BLSPublicKey, shard.BLSSignature) {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey)
	pub := shard.BLSPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BLSVerificationStr)
	privateKey := &bls.SecretKey{}
	privateKey.DeserializeHexStr(testBLSPrvKey)
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
	maxRate, _ := numeric.NewDecFromStr("0.5")
	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
	commission := staking.CommissionRates{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
	}
	minSelfDel := tenK
	maxTotalDel := twelveK
	pubKey, pubSig := generateBLSKeySigPair()
	slotPubKeys := []shard.BLSPublicKey{pubKey}
	slotKeySigs := []shard.BLSSignature{pubSig}
	amount := tenK
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
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV3: validator already exists
func TestCV3(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
	statedb.SetValidatorFlag(msg.ValidatorAddress)

	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); !strings.Contains(err.Error(), errValidatorExist.Error()) {
		t.Error("expected", errValidatorExist, "got", err)
	}
}

// Test CV5: identity longer than 140 characters
func TestCV5(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// identity length: 200 characters
	msg.Identity = "adsfwryuiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfhuewhfiuewhfefhshfrhfhifhwbfvberhbvihfwuoefhusioehfeuwiafhaiobcfwfhceirui"
	identitylengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Identity), "maxIdentityLen", staking.MaxIdentityLength)
	var err error
	if _, err = VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
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
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// Website length: 200 characters
	msg.Website = "https://www.iwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfwwwwwfiuewhfefhshfrheterhbvihfwuoefhusioehfeuwiafhaiobcfwfhceirui.com"
	websiteLengthCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Website), "maxWebsiteLen", staking.MaxWebsiteLength)
	_, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	)
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
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// Security Contact length: 200 characters
	msg.SecurityContact = "HelloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfwwwwwfiuewhfefhshfrheterhbvihfwuoefhusioehfeuwiafhaiobcfwfhceiruiHellodfdfdf"
	securityContactLengthError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.SecurityContact), "maxSecurityContactLen", staking.MaxSecurityContactLength)
	_, err := VerifyAndCreateValidatorFromMsg(statedb, postStakingEpoch, big.NewInt(0), msg)
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
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// Details length: 300 characters
	msg.Details = "HelloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfwwwwwfiuewhfefhshfrheterhbvihfwuoefhusioehfeuwiafhaiobcfwfhceiruiHellodfdfdfjiusngognoherugbounviesrbgufhuoshcofwevusguahferhgvuervurehniwjvseivusehvsghjvorsugjvsiovjpsevsvvvvv"
	detailsLenCtxError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Details), "maxDetailsLen", staking.MaxDetailsLength)
	_, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	)
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
	msg.Name = "Helloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV10: identity == 140 characters
func TestCV10(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// identity length: 140 characters
	msg.Identity = "Helloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV11: website == 140 characters
func TestCV11(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// website length: 140 characters
	msg.Website = "Helloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV12: security == 140 characters
func TestCV12(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// security contact length: 140 characters
	msg.SecurityContact = "Helloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV13: details == 280 characters
func TestCV13(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// details length: 280 characters
	msg.Details = "HelloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjoweHlloiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuedbfcuyiewfhwieufwiweifhcwefhwefhwidsffevjnononwondqmeofniowfndjowe"
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV14: commission rate <= max rate & max change rate <= max rate
func TestCV14(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate == max rate &&  max change rate == max rate
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("0.5")
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("0.5")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV15: commission rate > max rate
func TestCV15(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate: 0.6 > max rate: 0.5
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("0.6")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "commission rate and change rate can not be larger than max commission rate", "got", nil)
	}
}

// Test CV16: max change rate > max rate
func TestCV16(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max change rate: 0.6 > max rate: 0.5
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("0.6")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "commission rate and change rate can not be larger than max commission rate", "got", nil)
	}
}

// Test CV17: max rate == 1
func TestCV17(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max rate == 1
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("1")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV18: max rate == 1 && max change rate == 1 && commission rate == 0
func TestCV18(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max rate == 1 && max change rate == 1 && commission rate == 0
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("1")
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("1")
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("1")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV19: commission rate == 0
func TestCV19(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate == 0
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("0")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV20: max change rate == 0
func TestCV20(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate == 0
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("0")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV21: max change rate == 0 & max rate == 0 & commission rate == 0
func TestCV21(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max change rate == 0 & max rate == 0 & commission rate == 0
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("0")
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("0")
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("0")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV22: max change rate == 1 & max rate == 1
func TestCV22(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max change rate == 1 & max rate == 1
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("1")
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("1")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV23: commission rate < 0
func TestCV23(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate < 0
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("-0.1")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:-0.100000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV24: max rate < 0
func TestCV24(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max rate < 0
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("-0.001")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:-0.001000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV25: max change rate < 0
func TestCV25(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max rate < 0
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("-0.001")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:-0.001000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV26: commission rate > 1
func TestCV26(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// commission rate > 1
	msg.CommissionRates.Rate, _ = numeric.NewDecFromStr("1.01")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV27: max rate > 1
func TestCV27(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max rate > 1
	msg.CommissionRates.MaxRate, _ = numeric.NewDecFromStr("1.01")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV28: max change rate > 1
func TestCV28(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// max change rate > 1
	msg.CommissionRates.MaxChangeRate, _ = numeric.NewDecFromStr("1.01")
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "rate:1.01000000000000000: commission rate, change rate and max rate should be within 0-100 percent", "got", nil)
	}
}

// Test CV29: amount > MinSelfDelegation
func TestCV29(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, twelveK)
	// amount > MinSelfDelegation
	msg.Amount = twelveK
	msg.MinSelfDelegation = tenK
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV30: amount == MinSelfDelegation
func TestCV30(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// amount > MinSelfDelegation
	msg.Amount = tenK
	msg.MinSelfDelegation = tenK
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV31: amount < MinSelfDelegation
func TestCV31(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// amount > MinSelfDelegation
	msg.Amount = twelveK
	msg.MinSelfDelegation = tenK
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "min_self_delegation 5000000000000000000, after delegation amount 4000000000000000000: self delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test CV32: MaxTotalDelegation < MinSelfDelegation
func TestCV32(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// MaxTotalDelegation < MinSelfDelegation
	msg.MaxTotalDelegation = tenK
	msg.MinSelfDelegation = twelveK
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "max_total_delegation can not be less than min_self_delegation", "got", nil)
	}
}

// Test CV33: MinSelfDelegation < 1 ONE
func TestCV33(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// MinSelfDelegation < 10,000 ONE
	msg.MinSelfDelegation = big.NewInt(1e18)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "delegation-given 1000000000000000000: min_self_delegation has to be greater than 10,000 ONE", "got", nil)
	}
}

// Test CV34: MinSelfDelegation not specified
func TestCV34(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// MinSelfDelegation not specified
	msg.MinSelfDelegation = nil
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "MinSelfDelegation can not be nil", "got", nil)
	}
}

// Test CV35: MinSelfDelegation < 0
func TestCV35(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// MinSelfDelegation < 0
	msg.MinSelfDelegation = big.NewInt(-1)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "delegation-given -1: min_self_delegation has to be greater than 1 ONE", "got", nil)
	}
}

// Test CV36: amount > MaxTotalDelegation
func TestCV36(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// amount > MaxTotalDelegation
	msg.Amount = big.NewInt(4e18)
	msg.MaxTotalDelegation = big.NewInt(3e18)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "total delegation can not be bigger than max_total_delegation", "got", nil)
	}
}

// Test CV39: MaxTotalDelegation < 0
func TestCV39(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, tenK)
	// MaxTotalDelegation < 0
	msg.MaxTotalDelegation = big.NewInt(-1)
	if _, err := VerifyAndCreateValidatorFromMsg(
		statedb, postStakingEpoch, big.NewInt(0), msg,
	); err == nil {
		t.Error("expected", "max_total_delegation can not be less than min_self_delegation", "got", nil)
	}
}
