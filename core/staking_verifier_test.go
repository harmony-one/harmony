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
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	validatorAddress = common.Address(common2.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
)

func generateBlsKeySigPair() (shard.BlsPublicKey, shard.BLSSignature) {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey)
	pub := shard.BlsPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BlsVerificationStr)
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
	minSelfDel := big.NewInt(1e18)
	maxTotalDel := big.NewInt(9e18)
	pubKey, pubSig := generateBlsKeySigPair()
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
func Test_CV1(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err != nil {
		t.Error("expected", nil, "got", err)
	}
}

// Test CV3: validator already exists
func Test_CV3(t *testing.T) {
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

// Test CV4: identity longer than 140 characters
func Test_CV4(t *testing.T) {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	// identity length: 200 characters 
	msg.Identity = "adsfwryuiwfhwifbwfbcerghveugbviuscbhwiefbcusidbcifwefhgciwefherhbfiwuehfciwiuebfcuyiewfhwieufwiweifhcwefhwefhwiewwerfhuwefiuewfhuewhfiuewhfefhshfrhfhifhwbfvberhbvihfwuoefhusioehfeuwiafhaiobcfwfhceirui"
	statedb.AddBalance(msg.ValidatorAddress, big.NewInt(5e18))
	lengthError := ctxerror.New("[EnsureLength] Exceed Maximum Length", "have", len(msg.Identity), "maxIdentityLen", staking.MaxIdentityLength)
	if _, err := VerifyAndCreateValidatorFromMsg(statedb, big.NewInt(0), big.NewInt(0), msg); err.Error() != lengthError.Error() {
		t.Error("expected", lengthError, "got", err)
	}
}
