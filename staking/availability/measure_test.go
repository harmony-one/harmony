package availability

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

type fakerAuctioneer struct{}

const (
	to0           = "one1zyxauxquys60dk824p532jjdq753pnsenrgmef"
	to2           = "one14438psd5vrjes7qm97jrj3t0s5l4qff5j5cn4h"
	testBLSPubKey = "30b2c38b1316da91e068ac3bd8751c0901ef6c02a1d58bc712104918302c6ed03d5894671d0c816dad2b4d303320f202"
	testBLSPrvKey = "c6d7603520311f7a4e6aac0b26701fc433b75b38df504cd416ef2b900cd66205"
)

var (
	validatorS0Addr, validatorS2Addr = common.Address{}, common.Address{}
	addrs                            = []common.Address{}
	validatorS0, validatorS2         = &staking.ValidatorWrapper{}, &staking.ValidatorWrapper{}
)

func generateBlsKeySigPair(pubkey, prvkey string) (shard.BlsPublicKey, shard.BLSSignature) {
	p := &bls.PublicKey{}
	p.DeserializeHexStr(pubkey)
	pub := shard.BlsPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BlsVerificationStr)
	privateKey := &bls.SecretKey{}
	privateKey.DeserializeHexStr(prvkey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig shard.BLSSignature
	copy(sig[:], signature.Serialize())
	return pub, sig
}

func createValidator(addr, pubkey, prvkey string) *staking.Validator {
	pubKey, _ := generateBlsKeySigPair(pubkey, prvkey)
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
	return &staking.Validator{
		Address:            validatorS0Addr,
		SlotPubKeys:        []shard.BlsPublicKey{pubKey},
		MinSelfDelegation:  big.NewInt(1e18),
		MaxTotalDelegation: big.NewInt(9e18),
		Description:        desc,
		Commission:         staking.Commission{CommissionRates: commission, UpdateHeight: big.NewInt(0)},
	}
}

func createValidatorWrapper(v *staking.Validator) *staking.ValidatorWrapper {
	delegation := staking.Delegation{
		DelegatorAddress: v.Address,
		Amount:           big.NewInt(5e18),
	}
	delegations := []staking.Delegation{delegation}
	wrapper := &staking.ValidatorWrapper{
		Validator:   *v,
		Delegations: delegations,
	}
	wrapper.Counters.NumBlocksToSign = big.NewInt(0)
	wrapper.Counters.NumBlocksSigned = big.NewInt(0)
	return wrapper
}

func init() {
	validatorS0Addr, _ = common2.Bech32ToAddress(to0)
	validatorS2Addr, _ = common2.Bech32ToAddress(to2)
	addrs = []common.Address{validatorS0Addr, validatorS2Addr}
	validatorS0 = createValidatorWrapper(
		createValidator(to0, testBLSPubKey, testBLSPrvKey))

}

func (fakerAuctioneer) ReadValidatorSnapshot(
	addr common.Address,
) (*staking.ValidatorWrapper, error) {
	switch addr {
	case validatorS0Addr:
		return validatorS0, nil
	case validatorS2Addr:
		return validatorS2, nil
	default:
		panic("bad input in test case")
	}
}

func defaultStateWithAccountsApplied() *state.DB {
	st := ethdb.NewMemDatabase()
	stateHandle, _ := state.New(common.Hash{}, state.NewDatabase(st))
	for _, addr := range addrs {
		stateHandle.CreateAccount(addr)
	}
	stateHandle.SetBalance(validatorS0Addr, big.NewInt(0).SetUint64(1994680320000000000))
	stateHandle.SetBalance(validatorS2Addr, big.NewInt(0).SetUint64(1999975592000000000))
	return stateHandle
}

func resetWrapperCounters(wrapper *staking.ValidatorWrapper) {
	wrapper.Counters.NumBlocksToSign = big.NewInt(0)
	wrapper.Counters.NumBlocksSigned = big.NewInt(0)
}

func setWrapperCounters(wrapper *staking.ValidatorWrapper, toSign, signed *big.Int) {
	wrapper.Counters.NumBlocksToSign = toSign
	wrapper.Counters.NumBlocksSigned = signed
}

func TestComputeCurrentSigning(t *testing.T) {
	// nothing to sign case
	snapshot := createValidatorWrapper(
		createValidator(to0, testBLSPubKey, testBLSPrvKey))
	if _, _, quotient, _ := ComputeCurrentSigning(snapshot, snapshot); !quotient.Equal(numeric.ZeroDec()) {
		t.Error("[quotient] expected", numeric.ZeroDec(), "got", quotient)
	}
	wrapper := createValidatorWrapper(
		createValidator(to0, testBLSPubKey, testBLSPrvKey))
	// signed less than previous epoch
	snapshot.Counters.NumBlocksToSign = big.NewInt(10)
	snapshot.Counters.NumBlocksSigned = big.NewInt(6)

	if _, _, _, err := ComputeCurrentSigning(snapshot, wrapper); err == nil {
		t.Error("expected",
			"diff for signed period wrong: stat 0, snapshot 6: impossible period of signing",
			"got",
			nil,
		)
	}
	// toSign less than previous epoch
	snapshot.Counters.NumBlocksSigned = big.NewInt(0)

	if _, _, _, err := ComputeCurrentSigning(snapshot, wrapper); err == nil {
		t.Error("expected",
			"diff for toSign period wrong: stat 0, snapshot 10: impossible period of signing",
			"got",
			nil,
		)
	}
	// valid quotient
	resetWrapperCounters(snapshot)
	signed := big.NewInt(6)
	toSign := big.NewInt(10)
	expectedQuotient := numeric.NewDec(signed.Int64()).Quo(numeric.NewDec(toSign.Int64()))
	setWrapperCounters(wrapper, toSign, signed)
	_, _, quotient, err := ComputeCurrentSigning(snapshot, wrapper)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	if !quotient.Equal(expectedQuotient) {
		t.Error("[quotient] expected", expectedQuotient, "got", quotient)
	}
}

func testCompute(state *state.DB, wrapper *staking.ValidatorWrapper,
	toSign, signed *big.Int,
	status effective.Eligibility) error {
	setWrapperCounters(wrapper, toSign, signed)
	if err := compute(
		fakerAuctioneer{}, state, wrapper,
	); err != nil {
		return err
	}
	if wrapper.EPOSStatus != status {
		return errors.Wrapf(errors.New("EPOSStatus mismatch"), "expected %s, got %s", wrapper.EPOSStatus, status)
	}
	return nil
}

func TestCompute(t *testing.T) {
	state := defaultStateWithAccountsApplied()
	const junkValue = 0

	testWrapperS0 := &staking.ValidatorWrapper{
		Validator:   validatorS0.Validator,
		Delegations: validatorS0.Delegations,
	}
	err := testCompute(state, testWrapperS0, big.NewInt(10), big.NewInt(6), effective.Inactive)
	if err != nil {
		t.Error(err)
	}
	err = testCompute(state, testWrapperS0, big.NewInt(10), big.NewInt(7), effective.Active)
	if err != nil {
		t.Error(err)
	}
}

func TestBlockSigners(t *testing.T) {
	t.Log("Unimplemented")
}

func TestBallotResult(t *testing.T) {
	t.Log("Unimplemented")
}

func TestBumpCount(t *testing.T) {
	t.Log("Unimplemented")
}

func TestIncrementValidatorSigningCounts(t *testing.T) {
	t.Log("Unimplemented")
}
