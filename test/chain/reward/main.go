package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/crypto/bls"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/common"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	validatorAddress = common2.Address(common.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))

	testBLSPubKey    = "a8c1e0a9f4c6db2166c873addd8a925f6541f70e6baca6f61a470040320573ea29d906aa2fd329d6c75a6ebaaf2d3cac"
	testBLSPrvKey    = "2a899297ecd72b60aa70ce22d83b2fcb8e564ddb54f8e97695ee69a8b826e9b0"
	postStakingEpoch = big.NewInt(200)
)

func generateBLSKeySigPair() (bls.SerializedPublicKey, bls.SerializedSignature) {
	pubBytes, _ := hex.DecodeString(testBLSPubKey)
	pub, _ := bls.PublicKeyFromBytes(pubBytes)
	messageBytes := []byte(staking.BLSVerificationStr)
	privBytes, _ := hex.DecodeString(testBLSPrvKey)
	privateKey, _ := bls.SecretKeyFromBytes(privBytes)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.Sign(msgHash[:])
	return pub.Serialized(), signature.Serialized()
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
	database := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(database)
	_ = genesis
	engine := chain.NewEngine()
	bc, _ := core.NewBlockChain(database, nil, gspec.Config, engine, vm.Config{}, nil)
	statedb, _ := state.New(common2.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()))
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

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_PREPARE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	blsPriKey := bls.RandSecretKey()
	pubKeyWrapper := blsPriKey.PublicKey()

	request := message.GetConsensus()
	request.ViewId = 5
	request.BlockNum = 5
	request.ShardId = 1
	// 32 byte block hash
	request.BlockHash = []byte("stasdlkfjsadlkjfkdsljflksadjf")
	// sender address
	request.SenderPubkey = pubKeyWrapper.ToBytes()

	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	// 64 byte of signature on previous data
	hash1 := hash.Keccak256(marshaledMessage)

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		blsPriKey.Sign(hash1[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to sign: %f seconds\n", endTime.Sub(startTime).Seconds())

	sig := blsPriKey.Sign(hash1[:])
	message.Signature = sig.ToBytes()
	marshaledMessage2, _ := protobuf.Marshal(message)

	message = &msg_pb.Message{}
	if err := protobuf.Unmarshal(marshaledMessage2, message); err != nil {
		return
	}

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		if err := protobuf.Unmarshal(marshaledMessage2, message); err != nil {
			return
		}
	}
	endTime = time.Now()
	fmt.Printf("Time required to unmarshall: %f seconds\n", endTime.Sub(startTime).Seconds())

	signature := message.Signature
	message.Signature = nil

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		protobuf.Marshal(message)
	}
	endTime = time.Now()
	fmt.Printf("Time required to marshal: %f seconds\n", endTime.Sub(startTime).Seconds())
	messageBytes, err := protobuf.Marshal(message)

	msgSig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return
	}

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		msgSig.FromBytes(signature)
	}
	endTime = time.Now()
	fmt.Printf("Time required to deserialize sig: %f seconds\n", endTime.Sub(startTime).Seconds())

	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.Verify(pubKeyWrapper, msgHash[:]) {
		return
	}

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		hash.Keccak256(messageBytes)
	}
	endTime = time.Now()
	fmt.Printf("Time required to hash message: %f seconds\n", endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		msgSig.Verify(pubKeyWrapper, msgHash[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to verify sig: %f seconds\n", endTime.Sub(startTime).Seconds())

	message.Signature = signature

	// A example result of a single run:
	//
	//Time required to calc percentage 100001 delegations: 0.058205 seconds
	//Time required to reward a validator with 100001 delegations: 0.015543 seconds
	//Time required to sign: 0.479827 seconds
	//Time required to unmarshall: 0.000662 seconds
	//Time required to marshal: 0.000453 seconds
	//Time required to deserialize sig: 0.517965 seconds
	//Time required to hash message: 0.001191 seconds
	//Time required to verify sig: 1.444604 seconds
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
