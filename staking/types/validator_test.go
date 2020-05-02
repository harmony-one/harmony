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
)

var (
	nineK   = new(big.Int).Mul(big.NewInt(9000), big.NewInt(1e18))
	tenK    = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	twelveK = new(big.Int).Mul(big.NewInt(12000), big.NewInt(1e18))

	negativeRate = numeric.NewDec(-1)
	invalidRate  = numeric.NewDec(2)
)

var (
	invalidDescription = Description{
		Name:            "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongname",
		Identity:        "jacky@harmony.one",
		Website:         "harmony.one/jacky",
		SecurityContact: "jacky@harmony.one",
		Details:         "Details of jacky",
	}
)

// Using public keys to create slot for validator
func setSlotPubKeys() []shard.BLSPublicKey {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(blsPubKey)
	pub := shard.BLSPublicKey{}
	pub.FromLibBLSPublicKey(p)
	return []shard.BLSPublicKey{pub}
}

// Using private keys to create sign slot for message.CreateValidator
func setSlotKeySigs() []shard.BLSSignature {
	messageBytes := []byte(BLSVerificationStr)
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

func TestNewEmptyStats(t *testing.T) {
	stats := NewEmptyStats()
	if len(stats.APRs) != 0 {
		t.Errorf("empty stats not empty ARPs")
	}
	if !stats.TotalEffectiveStake.Equal(numeric.ZeroDec()) {
		t.Errorf("empty stats not zero total effective stake")
	}
	if len(stats.MetricsPerShard) != 0 {
		t.Errorf("empty stats not empty metris per shard")
	}
	if stats.BootedStatus != effective.Booted {
		t.Errorf("empty stats not booted statsu")
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
	tests := []struct {
		editValidator func(*Validator)
		expErr        error
	}{
		{func(v *Validator) {}, nil},
		{func(v *Validator) { v.Description = invalidDescription }, errors.New("invalid description")},
		{func(v *Validator) { v.SlotPubKeys = v.SlotPubKeys[:0] }, errNeedAtLeastOneSlotKey},
		{func(v *Validator) { v.MinSelfDelegation = nil }, errNilMinSelfDelegation},
		{func(v *Validator) { v.MaxTotalDelegation = nil }, errNilMaxTotalDelegation},
		{func(v *Validator) { v.MinSelfDelegation = nineK }, errMinSelfDelegationTooSmall},
		{func(v *Validator) { v.MaxTotalDelegation = nineK }, errInvalidMaxTotalDelegation},
		{func(v *Validator) { v.Rate = negativeRate }, errInvalidCommissionRate},
		{func(v *Validator) { v.Rate = invalidRate }, errInvalidCommissionRate},
		{func(v *Validator) { v.MaxRate = negativeRate }, errInvalidCommissionRate},
		{func(v *Validator) { v.MaxRate = invalidRate }, errInvalidCommissionRate},
		{func(v *Validator) { v.MaxChangeRate = negativeRate }, errInvalidCommissionRate},
		{func(v *Validator) { v.MaxChangeRate = invalidRate }, errInvalidCommissionRate},
		{
			func(v *Validator) { v.Rate, v.MaxRate = numeric.OneDec(), numeric.NewDecWithPrec(5, 1) },
			errCommissionRateTooLarge,
		},
		{
			func(v *Validator) { v.MaxChangeRate, v.MaxRate = numeric.OneDec(), numeric.NewDecWithPrec(5, 1) },
			errCommissionRateTooLarge,
		},
		{
			func(v *Validator) { v.SlotPubKeys = []shard.BLSPublicKey{slotPubKeys[0], slotPubKeys[0]} },
			errDuplicateSlotKeys,
		},
	}
	for i, test := range tests {
		if i != 5 {
			continue
		}
		v := createNewValidator()
		test.editValidator(&v)
		err := v.SanityCheck(DoNotEnforceMaxBLS)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: unexpected error [%v] / [%v]", i, err, test.expErr)
		}
	}
}

func TestDescription_EnsureLength(t *testing.T) {
	tests := []struct {
		desc   Description
		expErr error
	}{
		{
			desc: Description{
				Name:            "Jacky Wang",
				Identity:        "jacky@harmony.one",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of jacky",
			},
			expErr: nil,
		},
		{
			desc:   Description{},
			expErr: nil,
		},
		{
			desc: Description{
				Name:            "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongname",
				Identity:        "jacky@harmony.one",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of jacky",
			},
			expErr: errors.New("name too long"),
		},
		{
			desc: Description{
				Name:            "Jacky Wang",
				Identity:        "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongidentity",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of jacky",
			},
			expErr: errors.New("identity too long"),
		},
		{
			desc: Description{
				Name:            "Jacky Wang",
				Identity:        "jacky@harmony.one",
				Website:         "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongwebsite",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of jacky",
			},
			expErr: errors.New("website too long"),
		},
		{
			desc: Description{
				Name:            "Jacky Wang",
				Identity:        "jacky@harmony.one",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongdetail",
			},
			expErr: errors.New("details too long"),
		},
	}
	for _, test := range tests {
		d, err := test.desc.EnsureLength()
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("unexpected error: [%v] / [%v]", err, test.expErr)
		}
		if err != nil {
			continue
		}
		if err := assertDescriptionEqual(test.desc, d); err != nil {
			t.Errorf("assert equal: %v", err)
		}
	}
}

func TestUpdateDescription(t *testing.T) {
	tests := []struct {
		raw    Description
		update Description
		expect Description
		expErr error
	}{
		{
			raw: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "wenDetails",
			},
			update: Description{
				Name:            "Jacky",
				Identity:        "jw",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of Jacky",
			},
			expect: Description{
				Name:            "Jacky",
				Identity:        "jw",
				Website:         "harmony.one/jacky",
				SecurityContact: "jacky@harmony.one",
				Details:         "Details of Jacky",
			},
		},
		{
			raw: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "wenDetails",
			},
			update: Description{},
			expect: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "wenDetails",
			},
		},
		{
			raw: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "wenDetails",
			},
			update: Description{
				Details: "new details",
			},
			expect: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "new details",
			},
		},
		{
			raw: Description{
				Name:            "Wayne",
				Identity:        "wen",
				Website:         "harmony.one.wen",
				SecurityContact: "wenSecurity",
				Details:         "wenDetails",
			},
			update: Description{
				Website: "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongwebsite",
			},
			expErr: errors.New("website too long"),
		},
	}
	for i, test := range tests {
		d, err := UpdateDescription(test.raw, test.update)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: unexpected error: %v / %v", i, err, test.expErr)
		}
		if err != nil {
			continue
		}
		if err := assertDescriptionEqual(d, test.expect); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

// compare two descriptions' items
func assertDescriptionEqual(d1, d2 Description) error {
	if strings.Compare(d1.Name, d2.Name) != 0 ||
		strings.Compare(d1.Identity, d2.Identity) != 0 ||
		strings.Compare(d1.Website, d2.Website) != 0 ||
		strings.Compare(d1.SecurityContact, d2.SecurityContact) != 0 ||
		strings.Compare(d1.Details, d2.Details) != 0 {

		return errors.New("description not equal")
	}
	return nil
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
	_, err := CreateValidatorFromNewMsg(&v, blockNum, new(big.Int))
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
	UpdateValidatorFromEditMsg(&validator, &ev, new(big.Int))

	if validator.MinSelfDelegation.Cmp(tenK) != 0 {
		t.Errorf("UpdateValidatorFromEditMsg failed")
	}
}

func TestString(t *testing.T) {
	// print out the string
	fmt.Println(validator.String())
}
