package types

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	common "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/pkg/errors"
)

var (
	testAddr1, _  = common2.Bech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy")
	validatorAddr = common.Address(testAddr1)
	desc          = Description{
		Name:            "john",
		Identity:        "john",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecurity",
		Details:         "wenDetails",
	}
	blsPubKey   = "ba41f49d70d40434110e32b269dc9b52879ca5fb2aee01c49311c45e008a4b6494c3bd9e6ef7954e39d25d023243b898"
	blsPriKey   = "0a8c69c12020a762e7087f52bacbe835b8a91728f7310a191e026001f753a00e"
	slotPubKeys = setSlotPubKeys()
	slotKeySigs = setSlotKeySigs()

	validator = createNewValidator()
	wrapper   = createNewValidatorWrapper(validator)

	delegationAmt1 = big.NewInt(1e18)
	delegation1    = NewDelegation(delegatorAddr, delegationAmt1)

	longNameDesc = Description{
		Name:            "WayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayneWayne1",
		Identity:        "wen",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecurity",
		Details:         "wenDetails",
	}
	longIdentityDesc = Description{
		Name:            "Wayne",
		Identity:        "wenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwenwe1",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecurity",
		Details:         "wenDetails",
	}
	longWebsiteDesc = Description{
		Name:            "Wayne",
		Identity:        "wen",
		Website:         "harmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmony.one.wenharmo1",
		SecurityContact: "wenSecurity",
		Details:         "wenDetails",
	}
	longSecurityContactDesc = Description{
		Name:            "Wayne",
		Identity:        "wen",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuritywenSecuri",
		Details:         "wenDetails",
	}
	longDetailsDesc = Description{
		Name:            "Wayne",
		Identity:        "wen",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecurity",
		Details:         "wenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswwenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetailswenDetails",
	}
	tenK    = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	twelveK = new(big.Int).Mul(big.NewInt(12000), big.NewInt(1e18))
)

// Using public keys to create slot for validator
func setSlotPubKeys() []shard.BlsPublicKey {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(blsPubKey)
	pub := shard.BlsPublicKey{}
	pub.FromLibBLSPublicKey(p)
	return []shard.BlsPublicKey{pub}
}

// Using private keys to create sign slot for message.CreateValidator
func setSlotKeySigs() []shard.BLSSignature {
	messageBytes := []byte(BlsVerificationStr)
	privateKey := &bls.SecretKey{}
	privateKey.DeserializeHexStr(blsPriKey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig shard.BLSSignature
	copy(sig[:], signature.Serialize())
	return []shard.BLSSignature{sig}
}

// create a new validator
func createNewValidator() Validator {
	cr := CommissionRates{
		Rate:          numeric.OneDec(),
		MaxRate:       numeric.OneDec(),
		MaxChangeRate: numeric.ZeroDec(),
	}
	c := Commission{cr, big.NewInt(300)}
	d := Description{
		Name:     "Wayne",
		Identity: "wen",
		Website:  "harmony.one.wen",
		Details:  "best",
	}
	v := Validator{
		Address:              validatorAddr,
		SlotPubKeys:          slotPubKeys,
		LastEpochInCommittee: big.NewInt(20),
		MinSelfDelegation:    tenK,
		MaxTotalDelegation:   twelveK,
		Status:               effective.Active,
		Commission:           c,
		Description:          d,
		CreationHeight:       big.NewInt(12306),
	}
	return v
}

// create a new validator wrapper
func createNewValidatorWrapper(v Validator) ValidatorWrapper {
	return ValidatorWrapper{
		Validator: v,
	}
}

// Test MarshalValidator
func TestMarshalValidator(t *testing.T) {
	_, err := MarshalValidator(validator)
	if err != nil {
		t.Errorf("MarshalValidator failed")
	}
}

// Test UnmarshalValidator
func TestMarshalUnmarshalValidator(t *testing.T) {
	tmp, _ := MarshalValidator(validator)
	_, err := UnmarshalValidator(tmp)
	if err != nil {
		t.Errorf("UnmarshalValidator failed!")
	}
}

func TestTotalDelegation(t *testing.T) {
	// add a delegation to validator
	// delegation.Amount = 10000
	wrapper.Delegations = append(wrapper.Delegations, delegation1)
	totalNum := wrapper.TotalDelegation()

	// check if the numebr is 10000
	if totalNum.Cmp(big.NewInt(1e18)) != 0 {
		t.Errorf("TotalDelegation number is not right")
	}
}

// check the validator wrapper's sanity
func TestValidatorSanityCheck(t *testing.T) {
	err := validator.SanityCheck(DoNotEnforceMaxBLS)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}

	v := Validator{
		Address: validatorAddr,
	}
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err != errNeedAtLeastOneSlotKey {
		t.Error("expected", errNeedAtLeastOneSlotKey, "got", err)
	}

	v.SlotPubKeys = setSlotPubKeys()
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err != errNilMinSelfDelegation {
		t.Error("expected", errNilMinSelfDelegation, "got", err)
	}
	v.MinSelfDelegation = tenK
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err != errNilMaxTotalDelegation {
		t.Error("expected", errNilMaxTotalDelegation, "got", err)
	}
	v.MinSelfDelegation = big.NewInt(1e18)
	v.MaxTotalDelegation = twelveK
	e := errors.Wrapf(
		errMinSelfDelegationTooSmall,
		"delegation-given %s", v.MinSelfDelegation.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}

	v.MinSelfDelegation = twelveK
	v.MaxTotalDelegation = tenK
	e = errors.Wrapf(
		errInvalidMaxTotalDelegation,
		"max-total-delegation %s min-self-delegation %s",
		v.MaxTotalDelegation.String(),
		v.MinSelfDelegation.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.MinSelfDelegation = tenK
	v.MaxTotalDelegation = twelveK
	minusOneDec, _ := numeric.NewDecFromStr("-1")
	plusTwoDec, _ := numeric.NewDecFromStr("2")
	cr := CommissionRates{Rate: minusOneDec, MaxRate: numeric.OneDec(), MaxChangeRate: numeric.ZeroDec()}
	c := Commission{cr, big.NewInt(300)}
	v.Commission = c
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.Rate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.Rate = plusTwoDec
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.Rate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.Rate = numeric.MustNewDecFromStr("0.5")
	v.Commission.MaxRate = minusOneDec
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.MaxRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxRate = plusTwoDec
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.MaxRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxRate = numeric.MustNewDecFromStr("0.9")
	v.Commission.MaxChangeRate = minusOneDec
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.MaxChangeRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxChangeRate = plusTwoDec
	e = errors.Wrapf(
		errInvalidCommissionRate, "rate:%s", v.MaxChangeRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxChangeRate = numeric.MustNewDecFromStr("0.05")
	v.Commission.MaxRate = numeric.MustNewDecFromStr("0.41")
	e = errors.Wrapf(
		errCommissionRateTooLarge, "rate:%s", v.MaxRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxRate = numeric.MustNewDecFromStr("0.51")
	v.Commission.MaxChangeRate = numeric.MustNewDecFromStr("0.95")
	e = errors.Wrapf(
		errCommissionRateTooLarge, "rate:%s", v.MaxChangeRate.String(),
	)
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err.Error() != e.Error() {
		t.Error("expected", e, "got", err)
	}
	v.Commission.MaxChangeRate = numeric.MustNewDecFromStr("0.05")
	v.SlotPubKeys = append(v.SlotPubKeys, v.SlotPubKeys[0])
	if err := v.SanityCheck(DoNotEnforceMaxBLS); err != errDuplicateSlotKeys {
		t.Error("expected", errDuplicateSlotKeys, "got", err)
	}
}

func TestValidatorWrapperSanityCheck(t *testing.T) {
	// no delegation must fail
	wrapper := createNewValidatorWrapper(createNewValidator())
	if err := wrapper.SanityCheck(DoNotEnforceMaxBLS); err == nil {
		t.Error("expected", errInvalidSelfDelegation, "got", err)
	}

	// valid self delegation must not fail
	wrapper.Validator.MaxTotalDelegation = tenK
	valDel := NewDelegation(validatorAddr, tenK)
	wrapper.Delegations = []Delegation{valDel}
	if err := wrapper.SanityCheck(DoNotEnforceMaxBLS); err != nil {
		t.Errorf("validator wrapper SanityCheck failed: %s", err)
	}

	// invalid self delegation must fail
	valDel = NewDelegation(validatorAddr, big.NewInt(1e18))
	wrapper.Delegations = []Delegation{valDel}
	if err := wrapper.SanityCheck(DoNotEnforceMaxBLS); err == nil {
		t.Error("expected", errInvalidSelfDelegation, "got", err)
	}

	// invalid self delegation of less than 10K ONE must fail
	valDel = NewDelegation(validatorAddr, big.NewInt(1e18))
	if err := wrapper.SanityCheck(DoNotEnforceMaxBLS); err == nil {
		t.Error("expected", errInvalidSelfDelegation, "got", err)
	}
}

func testEnsureLength(t *testing.T) {
	// test name length > MaxNameLength
	if _, err := longNameDesc.EnsureLength(); err == nil {
		t.Error("expected", ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(longNameDesc.Name), "maxNameLen", MaxNameLength), "got", err)
	}
	// test identity length > MaxIdentityLength
	if _, err := longIdentityDesc.EnsureLength(); err == nil {
		t.Error("expected", ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(longIdentityDesc.Identity), "maxIdentityLen", MaxIdentityLength), "got", err)
	}
	// test website length > MaxWebsiteLength
	if _, err := longWebsiteDesc.EnsureLength(); err == nil {
		t.Error("expected", ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(longWebsiteDesc.Website), "maxWebsiteLen", MaxWebsiteLength), "got", err)
	}
	// test security contact length > MaxSecurityContactLength
	if _, err := longSecurityContactDesc.EnsureLength(); err == nil {
		t.Error("expected", ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(longSecurityContactDesc.SecurityContact), "maxSecurityContactLen", MaxSecurityContactLength), "got", err)
	}
	// test details length > MaxDetailsLength
	if _, err := longDetailsDesc.EnsureLength(); err == nil {
		t.Error("expected", ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(longDetailsDesc.Details), "maxDetailsLen", MaxDetailsLength), "got", err)
	}
}

func TestEnsureLength(t *testing.T) {
	_, err := validator.Description.EnsureLength()
	if err != nil {
		t.Error("expected", "nil", "got", err)
	}
	testEnsureLength(t)
}

func TestUpdateDescription(t *testing.T) {
	// create two description
	d1 := Description{
		Name:            "Wayne",
		Identity:        "wen",
		Website:         "harmony.one.wen",
		SecurityContact: "wenSecurity",
		Details:         "wenDetails",
	}
	d2 := Description{
		Name:            "John",
		Identity:        "jw",
		Website:         "harmony.one.john",
		SecurityContact: "johnSecurity",
		Details:         "johnDetails",
	}
	d1, _ = UpdateDescription(d1, d2)

	// check whether update description function work?
	if compareTwoDescription(d1, d2) {
		t.Errorf("UpdateDescription failed")
	}
}

// compare two descriptions' items
func compareTwoDescription(d1, d2 Description) bool {
	return (strings.Compare(d1.Name, d2.Name) != 0 &&
		strings.Compare(d1.Identity, d2.Identity) != 0 &&
		strings.Compare(d1.Website, d2.Website) != 0 &&
		strings.Compare(d1.SecurityContact, d2.SecurityContact) != 0 &&
		strings.Compare(d1.Details, d2.Details) != 0)
}

func TestVerifyBLSKeys(t *testing.T) {
	// test verify bls for valid single key/sig pair
	val := CreateValidator{
		ValidatorAddress: validatorAddr,
		Description:      desc,
		SlotPubKeys:      slotPubKeys,
		SlotKeySigs:      slotKeySigs,
		Amount:           big.NewInt(1e18),
	}
	if err := VerifyBLSKeys(val.SlotPubKeys, val.SlotKeySigs); err != nil {
		t.Errorf("VerifyBLSKeys failed")
	}

	// test verify bls for not matching single key/sig pair

	// test verify bls for not length matching multiple key/sig pairs

	// test verify bls for not order matching multiple key/sig pairs

	// test verify bls for empty key/sig pairs
}

func TestCreateValidatorFromNewMsg(t *testing.T) {
	v := CreateValidator{
		ValidatorAddress: validatorAddr,
		Description:      desc,
		Amount:           big.NewInt(1e18),
	}
	blockNum := big.NewInt(1000)
	_, err := CreateValidatorFromNewMsg(&v, blockNum)
	if err != nil {
		t.Errorf("CreateValidatorFromNewMsg failed")
	}
}

func TestUpdateValidatorFromEditMsg(t *testing.T) {
	ev := EditValidator{
		ValidatorAddress:   validatorAddr,
		Description:        desc,
		MinSelfDelegation:  tenK,
		MaxTotalDelegation: twelveK,
	}
	UpdateValidatorFromEditMsg(&validator, &ev)

	if validator.MinSelfDelegation.Cmp(tenK) != 0 {
		t.Errorf("UpdateValidatorFromEditMsg failed")
	}
}

func TestString(t *testing.T) {
	// print out the string
	fmt.Println(validator.String())
}
