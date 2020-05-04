package types

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/shard"
)

var (
	cpTestCreateValidator CreateValidator
	cpTestEditValidator   EditValidator
	cpTestDelegate        Delegate
	cpTestUndelegate      Undelegate
	cpTestCollectReward   CollectRewards
)

func init() {
	cpTestDataSetup()
}

func TestCreateValidator_Copy(t *testing.T) {
	tests := []struct {
		cv CreateValidator
	}{
		{cpTestCreateValidator},
		{CreateValidator{}}, // empty values
	}
	for i, test := range tests {
		cp := test.cv.Copy().(CreateValidator)

		if err := assertCreateValidatorDeepCopy(test.cv, cp); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func cpTestDataSetup() {
	cpTestCreateValidator = CreateValidator{
		ValidatorAddress: validatorAddr,
		Description: Description{
			Name:            "Wayne",
			Identity:        "wen",
			Website:         "harmony.one.wen",
			SecurityContact: "wenSecurity",
			Details:         "wenDetails",
		},
		CommissionRates: CommissionRates{
			Rate:          zeroDec,
			MaxRate:       oneDec,
			MaxChangeRate: zeroDec,
		},
		MinSelfDelegation:  tenK,
		MaxTotalDelegation: twelveK,
		SlotPubKeys:        slotPubKeys,
		SlotKeySigs:        slotKeySigs,
		Amount:             twelveK,
	}
}

func assertCreateValidatorDeepCopy(cv1, cv2 CreateValidator) error {
	if cv1.ValidatorAddress != cv2.ValidatorAddress {
		return fmt.Errorf("validator address not equal")
	}
	if !reflect.DeepEqual(cv1.Description, cv2.Description) {
		return fmt.Errorf("description value not equal")
	}
	if &cv1.Description == &cv2.Description {
		return fmt.Errorf("description not copy")
	}
	if err := assertCommissionRatesDeepCopy(cv1.CommissionRates, cv2.CommissionRates); err != nil {
		return fmt.Errorf("commissionRate %v", err)
	}
	if err := assertBigIntDeepCopy(cv1.MinSelfDelegation, cv2.MinSelfDelegation); err != nil {
		return fmt.Errorf("MinSelfDelegation %v", err)
	}
	if err := assertBigIntDeepCopy(cv1.MaxTotalDelegation, cv2.MaxTotalDelegation); err != nil {
		return fmt.Errorf("MaxTotalDelegation %v", err)
	}
	if err := assertPubsDeepCopy(cv1.SlotPubKeys, cv2.SlotPubKeys); err != nil {
		return fmt.Errorf("SlotPubKeys %v", err)
	}
	if err := assertSigsDeepCopy(cv1.SlotKeySigs, cv2.SlotKeySigs); err != nil {
		return fmt.Errorf("SlotKeySigs %v", err)
	}
	if err := assertBigIntDeepCopy(cv1.Amount, cv2.Amount); err != nil {
		return fmt.Errorf("amount %v", err)
	}
	return nil
}

func assertBigIntDeepCopy(i1, i2 *big.Int) error {
	if (i1 == nil) != (i2 == nil) {
		return errors.New("is nil not equal")
	}
	if i1 == nil {
		return nil
	}
	if i1.Cmp(i2) != 0 {
		return errors.New("value not equal")
	}
	if i1 == i2 {
		return errors.New("not copy")
	}
	return nil
}

func assertPubsDeepCopy(s1, s2 []shard.BLSPublicKey) error {
	if len(s1) != len(s2) {
		return fmt.Errorf("size not equal")
	}
	for i := range s1 {
		pub1, pub2 := s1[i], s2[i]
		if pub1 != pub2 {
			return fmt.Errorf("[%v] value not equal", i)
		}
		if &pub1 == &pub2 {
			return fmt.Errorf("[%v] same address", i)
		}
	}
	return nil
}

func assertSigsDeepCopy(s1, s2 []shard.BLSSignature) error {
	if len(s1) != len(s2) {
		return fmt.Errorf("size not equal")
	}
	for i := range s1 {
		pub1, pub2 := s1[i], s2[i]
		if pub1 != pub2 {
			return fmt.Errorf("[%v] value not equal", i)
		}
		if &pub1 == &pub2 {
			return fmt.Errorf("[%v] same address", i)
		}
	}
	return nil
}

// var (
// 	minSelfDelegation = big.NewInt(1000)
// 	stakeAmount       = big.NewInt(2000)
// 	delegateAmount    = big.NewInt(500)
// 	validatorAddress  = common.Address(common.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
// 	validatorAddress2 = common.Address(common.MustBech32ToAddress("one1d2rngmem4x2c6zxsjjz29dlah0jzkr0k2n88wc"))
// 	delegatorAddress  = common.Address(common.MustBech32ToAddress("one16qsd5ant9v94jrs89mruzx62h7ekcfxmduh2rx"))
// 	blsPubKey         = bls.RandPrivateKey().GetPublicKey()
// )

// func TestMsgCreateValidatorRLP(t *testing.T) {
// 	commissionRate := NewDecWithPrec(1, 2) // 10%
// 	maxRate := NewDecWithPrec(2, 2)        // 20%
// 	maxChangeRate := NewDecWithPrec(1, 3)  // 1%

// 	blsPublickey := shard.BLSPublicKey{}
// 	blsPublickey.FromLibBLSPublicKey(blsPubKey)

// 	msgCreateValidator := NewMsgCreateValidator(Description{
// 		Name:            "validator 1",
// 		Identity:        "1",
// 		Website:         "harmony.one",
// 		SecurityContact: "11.111.1111",
// 		Details:         "the best validator ever",
// 	}, CommissionRates{
// 		Rate:          commissionRate,
// 		MaxRate:       maxRate,
// 		MaxChangeRate: maxChangeRate,
// 	}, minSelfDelegation, validatorAddress, blsPublickey, stakeAmount)

// 	rlpBytes, err := rlp.EncodeToBytes(msgCreateValidator)
// 	if err != nil {
// 		t.Error("failed to rlp encode 'create validator' message")
// 	}

// 	decodedMsg := &MsgCreateValidator{}
// 	err = rlp.DecodeBytes(rlpBytes, decodedMsg)

// 	if err != nil {
// 		t.Error("failed to rlp decode 'create validator' message")
// 	}

// 	if !decodedMsg.Commission.Rate.Equal(msgCreateValidator.Commission.Rate) {
// 		t.Error("Commission rate does not match")
// 	}

// 	if !decodedMsg.Commission.MaxRate.Equal(msgCreateValidator.Commission.MaxRate) {
// 		t.Error("MaxRate does not match")
// 	}

// 	if !decodedMsg.Commission.MaxChangeRate.Equal(msgCreateValidator.Commission.MaxChangeRate) {
// 		t.Error("MaxChangeRate does not match")
// 	}

// 	if !reflect.DeepEqual(decodedMsg.Description, msgCreateValidator.Description) {
// 		t.Error("Description does not match")
// 	}

// 	if decodedMsg.MinSelfDelegation.Cmp(msgCreateValidator.MinSelfDelegation) != 0 {
// 		t.Error("MinSelfDelegation does not match")
// 	}

// 	if decodedMsg.StakingAddress.String() != msgCreateValidator.StakingAddress.String() {
// 		t.Error("StakingAddress does not match")
// 	}

// 	if shard.CompareBLSPublicKey(decodedMsg.ValidatingPubKey, msgCreateValidator.ValidatingPubKey) != 0 {
// 		t.Error("ValidatingPubKey does not match")
// 	}

// 	if decodedMsg.Amount.Cmp(msgCreateValidator.Amount) != 0 {
// 		t.Error("Amount does not match")
// 	}
// }

// func TestMsgEditValidatorRLP(t *testing.T) {
// 	commissionRate := NewDecWithPrec(1, 2) // 10%

// 	blsPublickey := shard.BLSPublicKey{}
// 	blsPublickey.FromLibBLSPublicKey(blsPubKey)

// 	msgEditValidator := NewMsgEditValidator(Description{
// 		Name:            "validator 1",
// 		Identity:        "1",
// 		Website:         "harmony.one",
// 		SecurityContact: "11.111.1111",
// 		Details:         "the best validator ever",
// 	}, validatorAddress, commissionRate, minSelfDelegation)

// 	rlpBytes, err := rlp.EncodeToBytes(msgEditValidator)
// 	if err != nil {
// 		t.Error("failed to rlp encode 'create validator' message")
// 	}

// 	decodedMsg := &MsgEditValidator{}
// 	err = rlp.DecodeBytes(rlpBytes, decodedMsg)

// 	if err != nil {
// 		t.Error("failed to rlp decode 'create validator' message")
// 	}

// 	if !reflect.DeepEqual(decodedMsg.Description, msgEditValidator.Description) {
// 		t.Error("Description does not match")
// 	}

// 	if decodedMsg.StakingAddress.String() != msgEditValidator.StakingAddress.String() {
// 		t.Error("StakingAddress does not match")
// 	}

// 	if !decodedMsg.CommissionRate.Equal(msgEditValidator.CommissionRate) {
// 		t.Error("Commission rate does not match")
// 	}

// 	if decodedMsg.MinSelfDelegation.Cmp(msgEditValidator.MinSelfDelegation) != 0 {
// 		t.Error("MinSelfDelegation does not match")
// 	}
// }

// func TestMsgDelegateRLP(t *testing.T) {
// 	msgDelegate := NewMsgDelegate(delegatorAddress, validatorAddress, delegateAmount)

// 	rlpBytes, err := rlp.EncodeToBytes(msgDelegate)
// 	if err != nil {
// 		t.Error("failed to rlp encode 'create validator' message")
// 	}

// 	decodedMsg := &MsgDelegate{}
// 	err = rlp.DecodeBytes(rlpBytes, decodedMsg)

// 	if err != nil {
// 		t.Error("failed to rlp decode 'create validator' message")
// 	}

// 	if decodedMsg.DelegatorAddress.String() != msgDelegate.DelegatorAddress.String() {
// 		t.Error("DelegatorAddress does not match")
// 	}

// 	if decodedMsg.ValidatorAddress.String() != msgDelegate.ValidatorAddress.String() {
// 		t.Error("ValidatorAddress does not match")
// 	}

// 	if decodedMsg.Amount.Cmp(msgDelegate.Amount) != 0 {
// 		t.Error("Amount does not match")
// 	}
// }

// func TestMsgRedelegateRLP(t *testing.T) {
// 	msgRedelegate := NewMsgRedelegate(delegatorAddress, validatorAddress, validatorAddress2, delegateAmount)

// 	rlpBytes, err := rlp.EncodeToBytes(msgRedelegate)
// 	if err != nil {
// 		t.Error("failed to rlp encode 'create validator' message")
// 	}

// 	decodedMsg := &MsgRedelegate{}
// 	err = rlp.DecodeBytes(rlpBytes, decodedMsg)

// 	if err != nil {
// 		t.Error("failed to rlp decode 'create validator' message")
// 	}

// 	if decodedMsg.DelegatorAddress.String() != msgRedelegate.DelegatorAddress.String() {
// 		t.Error("DelegatorAddress does not match")
// 	}

// 	if decodedMsg.ValidatorSrcAddress.String() != msgRedelegate.ValidatorSrcAddress.String() {
// 		t.Error("ValidatorSrcAddress does not match")
// 	}

// 	if decodedMsg.ValidatorDstAddress.String() != msgRedelegate.ValidatorDstAddress.String() {
// 		t.Error("ValidatorDstAddress does not match")
// 	}

// 	if decodedMsg.Amount.Cmp(msgRedelegate.Amount) != 0 {
// 		t.Error("Amount does not match")
// 	}
// }

// func TestMsgUndelegateRLP(t *testing.T) {
// 	msgUndelegate := NewMsgUndelegate(delegatorAddress, validatorAddress, delegateAmount)

// 	rlpBytes, err := rlp.EncodeToBytes(msgUndelegate)
// 	if err != nil {
// 		t.Error("failed to rlp encode 'create validator' message")
// 	}

// 	decodedMsg := &MsgUndelegate{}
// 	err = rlp.DecodeBytes(rlpBytes, decodedMsg)

// 	if err != nil {
// 		t.Error("failed to rlp decode 'create validator' message")
// 	}

// 	if decodedMsg.DelegatorAddress.String() != msgUndelegate.DelegatorAddress.String() {
// 		t.Error("DelegatorAddress does not match")
// 	}

// 	if decodedMsg.ValidatorAddress.String() != msgUndelegate.ValidatorAddress.String() {
// 		t.Error("ValidatorAddress does not match")
// 	}

// 	if decodedMsg.Amount.Cmp(msgUndelegate.Amount) != 0 {
// 		t.Error("Amount does not match")
// 	}
// }
