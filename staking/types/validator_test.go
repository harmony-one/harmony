package types

import (
	"fmt"
	"math/big"
	"testing"

	common "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/pkg/errors"
)

var (
	blsPubSigPairs []blsPubSigPair
	hmyBLSPub      shard.BLSPublicKey

	hmyBLSPubStr     = "c2962419d9999a87daa134f6d177f9ccabfe168a470587b13dd02ce91d1690a92170e5949d3dbdfc1b13fd7327dbef8c"
	validatorAddr, _ = common2.Bech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy")
)

var (
	nineK   = new(big.Int).Mul(big.NewInt(9000), big.NewInt(1e18))
	tenK    = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	twelveK = new(big.Int).Mul(big.NewInt(12000), big.NewInt(1e18))
	twentyK = new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))

	negativeRate = numeric.NewDec(-1)
	invalidRate  = numeric.NewDec(2)
)

var (
	validDescription = Description{
		Name:            "Jacky Wang",
		Identity:        "jacky@harmony.one",
		Website:         "harmony.one/jacky",
		SecurityContact: "jacky@harmony.one",
		Details:         "Details of jacky",
	}

	invalidDescription = Description{
		Name:            "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongname",
		Identity:        "jacky@harmony.one",
		Website:         "harmony.one/jacky",
		SecurityContact: "jacky@harmony.one",
		Details:         "Details of jacky",
	}

	validCommissionRates = CommissionRates{
		Rate:          numeric.ZeroDec(),
		MaxRate:       numeric.ZeroDec(),
		MaxChangeRate: numeric.ZeroDec(),
	}
)

func init() {
	// set bls pub and sig pairs for testing
	blsPubSigPairs = makeBLSPubSigPairs(5)
	// set bls pub keys for hmy
	copy(hmyBLSPub[:], common.Hex2Bytes(hmyBLSPubStr))
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

// Test UnmarshalValidator
func TestMarshalUnmarshalValidator(t *testing.T) {
	raw := makeValidValidator()

	b, err := MarshalValidator(raw)
	if err != nil {
		t.Fatal(err)
	}
	val, err := UnmarshalValidator(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertValidatorEqual(raw, val); err != nil {
		t.Error(err)
	}
}

// check the validator wrapper's sanity
func TestValidator_SanityCheck(t *testing.T) {
	tests := []struct {
		editValidator func(*Validator)
		expErr        error
	}{
		{
			func(v *Validator) {},
			nil,
		},
		{
			func(v *Validator) { v.Description = invalidDescription },
			errors.New("invalid description"),
		},
		{
			func(v *Validator) { v.SlotPubKeys = v.SlotPubKeys[:0] },
			errNeedAtLeastOneSlotKey,
		},
		{
			func(v *Validator) { v.MinSelfDelegation = nil },
			errNilMinSelfDelegation,
		},
		{
			func(v *Validator) { v.MaxTotalDelegation = nil },
			errNilMaxTotalDelegation,
		},
		{
			func(v *Validator) { v.MinSelfDelegation = nineK },
			errMinSelfDelegationTooSmall,
		},
		{
			func(v *Validator) { v.MaxTotalDelegation = nineK },
			errInvalidMaxTotalDelegation,
		},
		{
			func(v *Validator) { v.Rate = negativeRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.Rate = invalidRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.MaxRate = negativeRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.MaxRate = invalidRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.MaxChangeRate = negativeRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.MaxChangeRate = invalidRate },
			errInvalidCommissionRate,
		},
		{
			func(v *Validator) { v.Rate, v.MaxRate = numeric.OneDec(), numeric.NewDecWithPrec(5, 1) },
			errCommissionRateTooLarge,
		},
		{
			func(v *Validator) { v.MaxChangeRate, v.MaxRate = numeric.OneDec(), numeric.NewDecWithPrec(5, 1) },
			errCommissionRateTooLarge,
		},
		{
			func(v *Validator) { v.SlotPubKeys = []shard.BLSPublicKey{blsPubSigPairs[0].pub, blsPubSigPairs[0].pub} },
			errDuplicateSlotKeys,
		},
	}
	for i, test := range tests {
		v := makeValidValidator()
		test.editValidator(&v)
		err := v.SanityCheck(DoNotEnforceMaxBLS)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: unexpected error [%v] / [%v]", i, err, test.expErr)
		}
	}
}

func TestTotalDelegation(t *testing.T) {
	// add a delegation to validator
	// delegation.Amount = 10000
	wrapper := makeValidValidatorWrapper()
	totalNum := wrapper.TotalDelegation()

	if totalNum.Cmp(twelveK) != 0 {
		t.Errorf("TotalDelegation number is not right")
	}
}

func TestValidatorWrapper_SanityCheck(t *testing.T) {
	tests := []struct {
		editValidatorWrapper func(*ValidatorWrapper)
		expErr               error
	}{
		{
			func(*ValidatorWrapper) {},
			nil,
		},
		{
			func(vw *ValidatorWrapper) { vw.Validator.Description = invalidDescription },
			errors.New("invalid validator"),
		},
		{
			func(vw *ValidatorWrapper) { vw.Delegations = nil },
			errors.New("empty delegations"),
		},
		{
			func(vw *ValidatorWrapper) { vw.Delegations[0].Amount = nineK },
			errors.New("small self delegation"),
		},
		{
			func(vw *ValidatorWrapper) { vw.Delegations[1].Amount = twentyK },
			errors.New("large total delegation"),
		},
		{
			// banned node does not check minDelegation
			func(vw *ValidatorWrapper) {
				vw.Status = effective.Banned
				vw.Delegations[0].Amount = nineK
			},
			nil,
		},
		{
			// Banned node also checks total delegation
			func(vw *ValidatorWrapper) {
				vw.Status = effective.Banned
				vw.Delegations[1].Amount = twentyK
			},
			errors.New("banned node with large total delegation"),
		},
	}
	for i, test := range tests {
		vw := makeValidValidatorWrapper()
		test.editValidatorWrapper(&vw)
		err := vw.SanityCheck(DoNotEnforceMaxBLS)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: [%v] / [%v]", i, err, test.expErr)
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
				SecurityContact: "thisisaverylonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglonglongcontact",
				Details:         "Details of jacky",
			},
			expErr: errors.New("contact too long"),
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

func TestVerifyBLSKeys(t *testing.T) {
	pairs := makeBLSPubSigPairs(5)
	tests := []struct {
		pubIndexes []int
		sigIndexes []int
		expErr     error
	}{
		{[]int{0, 1, 2, 3, 4}, []int{0, 1, 2, 3, 4}, nil},
		{[]int{}, []int{}, nil},
		{[]int{0}, []int{}, errBLSKeysNotMatchSigs},
		{[]int{}, []int{1}, errBLSKeysNotMatchSigs},
		{[]int{0, 1, 2, 3}, []int{0, 0, 2, 3}, errors.New("bls not match")},
		{[]int{3, 2, 1, 0}, []int{0, 1, 2, 3}, errors.New("bls order not match")},
	}
	for i, test := range tests {
		pubs := getPubsFromPairs(pairs, test.pubIndexes)
		sigs := getSigsFromPairs(pairs, test.sigIndexes)

		err := VerifyBLSKeys(pubs, sigs)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: [%v] / [%v]", i, err, test.expErr)
		}
	}
}

func TestContainsHarmonyBLSKeys(t *testing.T) {
	pairs := makeBLSPubSigPairs(10)
	tests := []struct {
		pubIndexes    []int
		deployIndexes []int
		expErr        error
	}{
		{[]int{0}, []int{}, nil},
		{[]int{}, []int{0}, nil},
		{[]int{0, 1, 2, 3, 4, 5, 6, 7, 8}, []int{9}, nil},
		{[]int{0}, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, nil},
		{[]int{0, 1, 2, 3, 4}, []int{5, 6, 7, 8, 9}, nil},
		{[]int{0}, []int{0}, errors.New("duplicate bls pub")},
		{[]int{0, 1, 2, 3, 4, 5}, []int{5, 6, 7, 8, 9}, errors.New("duplicate bls pub")},
	}
	for i, test := range tests {
		pubs := getPubsFromPairs(pairs, test.pubIndexes)
		dPubs := getPubsFromPairs(pairs, test.deployIndexes)
		das := makeDeployAccountsFromBLSPubs(dPubs)

		err := containsHarmonyBLSKeys(pubs, das, big.NewInt(0))
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: [%v] / [%v]", i, err, test.expErr)
		}
	}
}

func makeDeployAccountsFromBLSPubs(pubs []shard.BLSPublicKey) []genesis.DeployAccount {
	das := make([]genesis.DeployAccount, 0, len(pubs))
	for i, pub := range pubs {
		das = append(das, genesis.DeployAccount{
			Address:      common.BigToAddress(big.NewInt(int64(i))).Hex(),
			BLSPublicKey: pub.Hex(),
		})
	}
	return das
}

func TestUpdateValidatorFromEditMsg(t *testing.T) {
	ev := EditValidator{
		ValidatorAddress:   validatorAddr,
		Description:        validDescription,
		MinSelfDelegation:  tenK,
		MaxTotalDelegation: twelveK,
	}
	validator := makeValidValidator()
	UpdateValidatorFromEditMsg(&validator, &ev, new(big.Int))

	if validator.MinSelfDelegation.Cmp(tenK) != 0 {
		t.Errorf("UpdateValidatorFromEditMsg failed")
	}
}

func TestCreateValidatorFromNewMsg(t *testing.T) {
	tests := []struct {
		editCreateValidator func(*CreateValidator)
		expErr              error
	}{
		{func(cv *CreateValidator) {}, nil},
		{func(cv *CreateValidator) { cv.Description = invalidDescription }, errors.New("invalid description")},
		{func(cv *CreateValidator) { cv.SlotPubKeys[0] = hmyBLSPub }, errors.New("hmy node")},
		{func(cv *CreateValidator) { cv.SlotKeySigs[0] = blsPubSigPairs[2].sig }, errors.New("wrong bls signature")},
	}
	for i, test := range tests {
		cv := makeCreateValidator()
		test.editCreateValidator(&cv)

		v, err := CreateValidatorFromNewMsg(&cv, common.Big1, common.Big1)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: error [%v] / [%v]", i, err, test.expErr)
		}
		if err != nil {
			continue
		}
		if err := assertValidatorAlignCreateValidator(*v, cv); err != nil {
			t.Error(err)
		}
	}
}

type blsPubSigPair struct {
	pub shard.BLSPublicKey
	sig shard.BLSSignature
}

func makeBLSPubSigPairs(size int) []blsPubSigPair {
	pairs := make([]blsPubSigPair, 0, size)
	for i := 0; i != size; i++ {
		pairs = append(pairs, makeBLSPubSigPair())
	}
	return pairs
}

func makeBLSPubSigPair() blsPubSigPair {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub shard.BLSPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig shard.BLSSignature
	copy(shardSig[:], sig.Serialize())

	return blsPubSigPair{shardPub, shardSig}
}

func getPubsFromPairs(pairs []blsPubSigPair, indexes []int) []shard.BLSPublicKey {
	pubs := make([]shard.BLSPublicKey, 0, len(indexes))
	for _, index := range indexes {
		pubs = append(pubs, pairs[index].pub)
	}
	return pubs
}

func getSigsFromPairs(pairs []blsPubSigPair, indexes []int) []shard.BLSSignature {
	sigs := make([]shard.BLSSignature, 0, len(indexes))
	for _, index := range indexes {
		sigs = append(sigs, pairs[index].sig)
	}
	return sigs
}

func TestString(t *testing.T) {
	// print out the string
	//fmt.Println(validator.String())
}

// makeValidValidator makes a valid Validator data structure
func makeValidValidator() Validator {
	cr := validCommissionRates
	c := Commission{cr, big.NewInt(300)}
	d := Description{
		Name:     "Wayne",
		Identity: "wen",
		Website:  "harmony.one.wen",
		Details:  "best",
	}
	v := Validator{
		Address:              validatorAddr,
		SlotPubKeys:          []shard.BLSPublicKey{blsPubSigPairs[0].pub},
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

// makeValidValidatorWrapper makes a valid validator wrapper
func makeValidValidatorWrapper() ValidatorWrapper {
	v := makeValidValidator()
	ds := Delegations{
		NewDelegation(v.Address, new(big.Int).Set(v.MinSelfDelegation)),
		NewDelegation(common.BigToAddress(common.Big1), new(big.Int).Sub(v.MaxTotalDelegation, v.MinSelfDelegation)),
	}
	return ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

// makeCreateValidator makes a structure of CreateValidator
func makeCreateValidator() CreateValidator {
	addr := validatorAddr
	desc := validDescription
	cr := validCommissionRates
	pubs := getPubsFromPairs(blsPubSigPairs, []int{0, 1})
	sigs := getSigsFromPairs(blsPubSigPairs, []int{0, 1})
	return CreateValidator{
		ValidatorAddress:   addr,
		Description:        desc,
		CommissionRates:    cr,
		MinSelfDelegation:  tenK,
		MaxTotalDelegation: twelveK,
		SlotPubKeys:        pubs,
		SlotKeySigs:        sigs,
		Amount:             twelveK,
	}
}

func assertValidatorEqual(v1, v2 Validator) error {
	if v1.Address != v2.Address {
		return fmt.Errorf("address not equal: %v / %v", v1.Address, v2.Address)
	}
	if len(v1.SlotPubKeys) != len(v2.SlotPubKeys) {
		return fmt.Errorf("len(SlotPubKeys) not equal: %v / %v", len(v1.SlotPubKeys), len(v2.SlotPubKeys))
	}
	for i := range v1.SlotPubKeys {
		pk1, pk2 := v1.SlotPubKeys[i], v2.SlotPubKeys[i]
		if pk1 != pk2 {
			return fmt.Errorf("SlotPubKeys[%v] not equal: %s / %s", i, pk1.Hex(), pk2.Hex())
		}
	}
	if v1.LastEpochInCommittee.Cmp(v2.LastEpochInCommittee) != 0 {
		return fmt.Errorf("LastEpochInCommittee not equal: %v / %v", v1.LastEpochInCommittee, v2.LastEpochInCommittee)
	}
	if v1.MinSelfDelegation.Cmp(v2.MinSelfDelegation) != 0 {
		return fmt.Errorf("MinSelfDelegation not equal: %v / %v", v1.MinSelfDelegation, v2.MinSelfDelegation)
	}
	if v1.MaxTotalDelegation.Cmp(v2.MaxTotalDelegation) != 0 {
		return fmt.Errorf("MaxTotalDelegation not equal: %v / %v", v1.MaxTotalDelegation, v2.MaxTotalDelegation)
	}
	if v1.Status != v2.Status {
		return fmt.Errorf("status not equal: %v / %v", v1.Status, v2.Status)
	}
	if err := assertCommissionRatesEqual(v1.CommissionRates, v2.CommissionRates); err != nil {
		return fmt.Errorf("validator.Commission: %v", err)
	}
	if err := assertDescriptionEqual(v1.Description, v2.Description); err != nil {
		return fmt.Errorf("validator.Description: %v", err)
	}
	if v1.CreationHeight.Cmp(v2.CreationHeight) != 0 {
		return fmt.Errorf("CreationHeight not equal: %v / %v", v1.CreationHeight, v2.CreationHeight)
	}
	return nil
}

func assertValidatorAlignCreateValidator(v Validator, cv CreateValidator) error {
	if v.Address != cv.ValidatorAddress {
		return fmt.Errorf("addressed not equal")
	}
	if len(v.SlotPubKeys) != len(cv.SlotPubKeys) {
		return fmt.Errorf("len(SlotPubKeys) not equal")
	}
	for i := range v.SlotPubKeys {
		if v.SlotPubKeys[i] != cv.SlotPubKeys[i] {
			return fmt.Errorf("SlotPubKeys[%v] not equal", i)
		}
	}
	if v.LastEpochInCommittee.Cmp(new(big.Int)) != 0 {
		return fmt.Errorf("LastEpochInCommittee not zero")
	}
	if v.MinSelfDelegation.Cmp(cv.MinSelfDelegation) != 0 {
		return fmt.Errorf("MinSelfDelegation not equal")
	}
	if v.MaxTotalDelegation.Cmp(cv.MaxTotalDelegation) != 0 {
		return fmt.Errorf("MaxTotalDelegation not equal")
	}
	if v.Status != effective.Active {
		return fmt.Errorf("status not active")
	}
	if err := assertCommissionRatesEqual(v.CommissionRates, cv.CommissionRates); err != nil {
		return fmt.Errorf("commissionRate not expected: %v", err)
	}
	if v.UpdateHeight.Cmp(v.CreationHeight) != 0 {
		return fmt.Errorf("validator's update height not equal to creation height")
	}
	if err := assertDescriptionEqual(v.Description, cv.Description); err != nil {
		return fmt.Errorf("description not expected: %v", err)
	}
	return nil
}

func assertCommissionRatesEqual(c1, c2 CommissionRates) error {
	if !c1.Rate.Equal(c2.Rate) {
		return fmt.Errorf("rate not equal: %v / %v", c1.Rate, c2.Rate)
	}
	if !c1.MaxRate.Equal(c2.MaxRate) {
		return fmt.Errorf("max rate not equal: %v / %v", c1.MaxRate, c2.MaxRate)
	}
	if !c1.MaxChangeRate.Equal(c2.MaxChangeRate) {
		return fmt.Errorf("max change rate not equal: %v / %v", c1.MaxChangeRate, c2.MaxChangeRate)
	}
	return nil
}

// compare two descriptions' items
func assertDescriptionEqual(d1, d2 Description) error {
	if d1.Name != d2.Name {
		return fmt.Errorf("name not equal: [%v] / [%v]", d1.Name, d2.Name)
	}
	if d1.Identity != d2.Identity {
		return fmt.Errorf("identity not equal: [%v] / [%v]", d1.Identity, d2.Identity)
	}
	if d1.Website != d2.Website {
		return fmt.Errorf("website not equal: [%v] / [%v]", d1.Website, d2.Website)
	}
	if d1.SecurityContact != d2.SecurityContact {
		return fmt.Errorf("security contact not equal: [%v] / [%v]", d1.SecurityContact, d2.SecurityContact)
	}
	if d1.Details != d2.Details {
		return fmt.Errorf("details not equal: [%v] / [%v]", d1.Details, d2.Details)
	}
	return nil
}
