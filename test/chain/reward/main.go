package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	validatorAddress = common2.Address(common.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))

	testBLSPubKey    = "30b2c38b1316da91e068ac3bd8751c0901ef6c02a1d58bc712104918302c6ed03d5894671d0c816dad2b4d303320f202"
	testBLSPrvKey    = "c6d7603520311f7a4e6aac0b26701fc433b75b38df504cd416ef2b900cd66205"
	postStakingEpoch = big.NewInt(200)
)

func init() {
	bls_core.Init(bls_core.BLS12_381)
}

func generateBLSKeySigPair() (bls.SerializedPublicKey, bls.SerializedSignature) {
	p := &bls_core.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey)
	pub := bls.SerializedPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BLSVerificationStr)
	privateKey := &bls_core.SecretKey{}
	privateKey.DeserializeHexStr(testBLSPrvKey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig bls.SerializedSignature
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
	minSelfDel := new(big.Int).Mul(big.NewInt(5e18), big.NewInt(2000))
	maxTotalDel := new(big.Int).Mul(big.NewInt(5e18), big.NewInt(100000))
	pubKey, pubSig := generateBLSKeySigPair()
	slotPubKeys := []bls.SerializedPublicKey{pubKey}
	slotKeySigs := []bls.SerializedSignature{pubSig}
	amount := new(big.Int).Mul(big.NewInt(5e18), big.NewInt(2000))
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

func main() {
	key, _ := crypto.GenerateKey()
	gspec := core.Genesis{
		Config:  params.TestChainConfig,
		Factory: blockfactory.ForTest,
		Alloc: core.GenesisAlloc{
			crypto.PubkeyToAddress(key.PublicKey): {
				Balance: big.NewInt(8000000000000000000),
			},
		},
		GasLimit: 1e18,
		ShardID:  0,
	}
	database := ethdb.NewMemDatabase()
	genesis := gspec.MustCommit(database)
	_ = genesis
	bc, _ := core.NewBlockChain(database, nil, gspec.Config, chain.Engine, vm.Config{}, nil)
	statedb, _ := state.New(common2.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	msg := createValidator()
	statedb.AddBalance(msg.ValidatorAddress, new(big.Int).Mul(big.NewInt(5e18), big.NewInt(2000)))
	validator, err := core.VerifyAndCreateValidatorFromMsg(
		statedb, bc, postStakingEpoch, big.NewInt(0), msg,
	)
	if err != nil {
		fmt.Print(err)
	}
	for i := 0; i < 100000; i++ {
		validator.Delegations = append(validator.Delegations, staking.Delegation{
			common2.Address{},
			big.NewInt(int64(rand.Intn(100))),
			big.NewInt(0),
			nil,
		})
	}

	statedb.UpdateValidatorWrapper(msg.ValidatorAddress, validator)

	startTime := time.Now()
	validator, _ = statedb.ValidatorWrapper(msg.ValidatorAddress)
	endTime := time.Now()
	fmt.Printf("Time required to read validator: %f seconds\n", endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	shares, _ := lookupDelegatorShares(validator)
	endTime = time.Now()
	fmt.Printf("Time required to calc percentage %d delegations: %f seconds\n", len(validator.Delegations), endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	statedb.AddReward(validator, big.NewInt(1000), shares)
	endTime = time.Now()
	fmt.Printf("Time required to reward a validator with %d delegations: %f seconds\n", len(validator.Delegations), endTime.Sub(startTime).Seconds())
}

func lookupDelegatorShares(
	snapshot *staking.ValidatorWrapper,
) (result map[common2.Address]numeric.Dec, err error) {
	result = map[common2.Address]numeric.Dec{}
	totalDelegationDec := numeric.NewDecFromBigInt(snapshot.TotalDelegation())
	for i := range snapshot.Delegations {
		delegation := snapshot.Delegations[i]
		// NOTE percentage = <this_delegator_amount>/<total_delegation>
		if totalDelegationDec.IsZero() {
			utils.Logger().Info().
				RawJSON("validator-snapshot", []byte(snapshot.String())).
				Msg("zero total delegation during AddReward delegation payout")
			return nil, nil
		}
		percentage := numeric.NewDecFromBigInt(delegation.Amount).Quo(totalDelegationDec)
		result[delegation.DelegatorAddress] = percentage
	}
	return result, nil
}
