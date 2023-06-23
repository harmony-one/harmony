package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/harmony-one/harmony/core/rawdb"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/crypto/bls"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
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
	database := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(database)
	_ = genesis
	engine := chain.NewEngine()
	bc, _ := core.NewBlockChain(database, nil, nil, nil, gspec.Config, engine, vm.Config{})
	statedb, _ := state.New(common2.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
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
			Amount: big.NewInt(int64(rand.Intn(100))),
			Reward: big.NewInt(0),
		})
	}

	statedb.UpdateValidatorWrapper(msg.ValidatorAddress, validator)

	startTime := time.Now()
	validator, _ = statedb.ValidatorWrapper(msg.ValidatorAddress, true, false)
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

	blsPriKey := bls.RandPrivateKey()
	pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
	pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)

	request := message.GetConsensus()
	request.ViewId = 5
	request.BlockNum = 5
	request.ShardId = 1
	// 32 byte block hash
	request.BlockHash = []byte("stasdlkfjsadlkjfkdsljflksadjf")
	// sender address
	request.SenderPubkey = pubKeyWrapper.Bytes[:]

	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	// 64 byte of signature on previous data
	hash1 := hash.Keccak256(marshaledMessage)

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		blsPriKey.SignHash(hash1[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to sign: %f seconds\n", endTime.Sub(startTime).Seconds())

	sig := blsPriKey.SignHash(hash1[:])
	message.Signature = sig.Serialize()
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
	msgSig := bls_core.Sign{}
	err = msgSig.Deserialize(signature)

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		msgSig.Deserialize(signature)
	}
	endTime = time.Now()
	fmt.Printf("Time required to deserialize sig: %f seconds\n", endTime.Sub(startTime).Seconds())

	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(pubKeyWrapper.Object, msgHash[:]) {
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
		msgSig.VerifyHash(pubKeyWrapper.Object, msgHash[:])
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
