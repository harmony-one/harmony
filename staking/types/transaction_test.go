package types

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	numeric "github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// for testing purpose
var (
	testAccount    = "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"
	testBLSPubKey  = "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"
	testBLSPubKey2 = "40379eed79ed82bebfb4310894fd33b6a3f8413a78dc4d43b98d0adc9ef69f3285df05eaab9f2ce5f7227f8cb920e809"
)

func CreateTestNewTransaction() (*StakingTransaction, error) {
	dAddr, _ := common2.Bech32ToAddress(testAccount)

	stakePayloadMaker := func() (Directive, interface{}) {
		p := &bls.PublicKey{}
		p.DeserializeHexStr(testBLSPubKey)
		pub := shard.BlsPublicKey{}
		pub.FromLibBLSPublicKey(p)

		ra, _ := numeric.NewDecFromStr("0.7")
		maxRate, _ := numeric.NewDecFromStr("1")
		maxChangeRate, _ := numeric.NewDecFromStr("0.5")
		return DirectiveCreateValidator, CreateValidator{
			Description: &Description{
				Name:            "SuperHero",
				Identity:        "YouWouldNotKnow",
				Website:         "Secret Website",
				SecurityContact: "LicenseToKill",
				Details:         "blah blah blah",
			},
			CommissionRates: CommissionRates{
				Rate:          ra,
				MaxRate:       maxRate,
				MaxChangeRate: maxChangeRate,
			},
			MinSelfDelegation:  big.NewInt(10),
			MaxTotalDelegation: big.NewInt(3000),
			ValidatorAddress:   common.Address(dAddr),
			SlotPubKeys:        []shard.BlsPublicKey{pub},
			Amount:             big.NewInt(100),
		}
	}

	gasPrice := big.NewInt(1)
	return NewStakingTransaction(0, 600000, gasPrice, stakePayloadMaker)
}

func TestTransactionCopy(t *testing.T) {
	tx1, err := CreateTestNewTransaction()
	if err != nil {
		t.Errorf("cannot create new staking transaction, %v\n", err)
	}
	tx2 := tx1.Copy()

	cv1 := tx1.data.StakeMsg.(CreateValidator)

	// modify cv1 fields
	cv1.Amount = big.NewInt(20)
	cv1.Description.Name = "NewName"
	newRate, _ := numeric.NewDecFromStr("0.5")
	cv1.CommissionRates.Rate = newRate

	p := &bls.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey2)
	pub := shard.BlsPublicKey{}
	pub.FromLibBLSPublicKey(p)
	cv1.SlotPubKeys = append(cv1.SlotPubKeys, pub)

	tx1.data.StakeMsg = cv1

	cv2 := tx2.data.StakeMsg.(CreateValidator)

	if cv1.Amount.Cmp(cv2.Amount) == 0 {
		t.Errorf("Amount should not be equal")
	}

	if len(cv1.SlotPubKeys) == len(cv2.SlotPubKeys) {
		t.Errorf("SlotPubKeys should not be equal length")
	}

	if len(cv1.Description.Name) == len(cv2.Description.Name) {
		t.Errorf("Description name should not be the same")
	}

	if cv1.CommissionRates.Rate.Equal(cv2.CommissionRates.Rate) {
		t.Errorf("CommissionRate should not be equal")
	}

	fmt.Println("cv1", cv1)
	fmt.Println("cv2", cv2)
	fmt.Println("cv1", cv1.Description)
	fmt.Println("cv2", cv2.Description)
}
