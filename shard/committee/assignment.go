package committee

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

// StateID means reading off whole network when using calls that accept
// a shardID parameter
const StateID = -1

// ValidatorListProvider ..
type ValidatorListProvider interface {
	Compute(
		epoch *big.Int, config params.ChainConfig, reader DataProvider,
	) (shard.State, error)
	ReadFromDB(epoch *big.Int, reader DataProvider) (shard.State, error)
}

// PublicKeysProvider per epoch
type PublicKeysProvider interface {
	// If call shardID with StateID then only superCommittee is non-nil,
	// otherwise get back the shardSpecific slice as well.
	ComputePublicKeys(
		epoch *big.Int, reader DataProvider, shardID int,
	) (superCommittee, shardSpecific []*bls.PublicKey)

	ReadPublicKeysFromDB(
		hash common.Hash, reader DataProvider,
	) ([]*bls.PublicKey, error)
}

// Reader is committee.Reader and it is the API that committee membership assignment needs
type Reader interface {
	PublicKeysProvider
	ValidatorListProvider
}

// StakingCandidatesReader ..
type StakingCandidatesReader interface {
	ValidatorInformation(addr common.Address) (*staking.Validator, error)
	ValidatorStakingWithDelegation(addr common.Address) *big.Int
	ValidatorCandidates() []common.Address
}

// ChainReader is a subset of Engine.ChainReader, just enough to do assignment
type ChainReader interface {
	// ReadShardState retrieves sharding state given the epoch number.
	// This api reads the shard state cached or saved on the chaindb.
	// Thus, only should be used to read the shard state of the current chain.
	ReadShardState(epoch *big.Int) (shard.State, error)
	// GetHeader retrieves a block header from the database by hash and number.
	GetHeaderByHash(common.Hash) *block.Header
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig
}

// DataProvider ..
type DataProvider interface {
	StakingCandidatesReader
	ChainReader
}

type partialStakingEnabled struct{}

var (
	// WithStakingEnabled ..
	WithStakingEnabled Reader = partialStakingEnabled{}
)

func preStakingEnabledCommittee(s shardingconfig.Instance) shard.State {
	shardNum := int(s.NumShards())
	shardHarmonyNodes := s.NumHarmonyOperatedNodesPerShard()
	shardSize := s.NumNodesPerShard()
	hmyAccounts := s.HmyAccounts()
	fnAccounts := s.FnAccounts()
	shardState := shard.State{}
	for i := 0; i < shardNum; i++ {
		com := shard.Committee{ShardID: uint32(i)}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardNum // The initial account to use for genesis nodes
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hmyAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				common2.ParseAddr(hmyAccounts[index].Address),
				pubKey,
				nil,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				common2.ParseAddr(fnAccounts[index].Address),
				pubKey,
				nil,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		shardState = append(shardState, com)
	}
	return shardState
}

func with400Stakers(
	s shardingconfig.Instance, stakerReader DataProvider,
) (shard.State, error) {
	// TODO Nervous about this because overtime the list will become quite large
	candidates := stakerReader.ValidatorCandidates()
	stakers := make([]*staking.Validator, len(candidates))
	// TODO benchmark difference if went with data structure that sorts on insert
	for i := range candidates {
		// TODO Should be using .ValidatorStakingWithDelegation, not implemented yet
		validator, err := stakerReader.ValidatorInformation(candidates[i])
		if err != nil {
			return nil, err
		}
		essentials[validator.Address] = effective.SlotOrder{
			validator.Stake,
			validator.SlotPubKeys,
		}
	}

	for i := range stakers {
		staker := stakers[i]
		stakers[i].Stake = new(big.Int).Div(
			staker.Stake, big.NewInt(int64(len(staker.SlotPubKeys))),
		)
	}

	unsortedStakes := make([]int, len(stakers))
	eposStakes := make([]*big.Int, len(stakers))

	for i, j := range stakers {
		unsortedStakes[i] = int(j.Stake.Int64())
		eposStakes[i] = j.Stake
	}

	s3 := effective.Apply(eposStakes)

	sort.SliceStable(
		stakers,
		func(i, j int) bool { return stakers[i].Stake.Cmp(stakers[j].Stake) >= 0 },
	)

	// for i, j := range stakers {
	// 	sortedStakes[i] = int(j.Stake.Int64())
	// }

	type t struct {
		Stakes []int
	}

	t2 := t{make([]int, len(eposStakes))}
	for i := range s3 {
		t2.Stakes[i] = int(s3[i].TruncateInt64())
	}

	s1, _ := json.Marshal(t{unsortedStakes})
	s2, _ := json.Marshal(t2)

	fmt.Println("Unsorted")
	fmt.Println(string(s1))
	fmt.Println("as EPOS")
	fmt.Println(string(s2))

	// fmt.Println("Sorted stakers %+v\n", stakers)

	shardCount := int(s.NumShards())
	superComm := make(shard.State, shardCount)
	fillCount := make([]int, shardCount)

	for i := 0; i < shardCount; i++ {
		superComm[i] = shard.Committee{
			uint32(i), make(shard.NodeIDList, s.NumNodesPerShard()),
		}
	}

	shardBig := big.NewInt(int64(s.NumShards()))

	for i := 0; i < len(s.FnAccounts()); i++ {
		bucket := int(new(big.Int).Mod(stakers[i].Address.Big(), shardBig).Int64())
		org := stakers[i].Stake
		epos := big.NewInt(s3[i].TruncateInt64())
		fmt.Println("stakes", org, epos)
		superComm[bucket].NodeList[fillCount[bucket]] = shard.NodeID{
			stakers[i].Address,
			stakers[i].SlotPubKeys[0],
			epos,
		}
		fillCount[bucket]++
	}

	hAccounts := s.HmyAccounts()
	offset := 0

	for i := range fillCount {
		missing := s.NumNodesPerShard() - fillCount[i]
		for j := 0; j < missing; j++ {
			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hAccounts[offset].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			superComm[i].NodeList[fillCount[i]+j] = shard.NodeID{
				common2.ParseAddr(hAccounts[offset].Address),
				pubKey,
				nil,
			}
			offset++
		}
	}

	fmt.Println("Final", superComm.JSON())
	fmt.Println("stakers", fillCount)
	return superComm, nil
}

// ComputePublicKeys produces publicKeys of entire supercommittee per epoch, optionally providing a
// shard specific subcommittee
func (def partialStakingEnabled) ComputePublicKeys(
	epoch *big.Int, d DataProvider, shardID int,
) ([]*bls.PublicKey, []*bls.PublicKey) {
	config := d.Config()
	instance := shard.Schedule.InstanceForEpoch(epoch)
	superComm := shard.State{}

	if config.IsStaking(epoch) {
		superComm, _ = with400Stakers(instance, d)
	} else {
		superComm = preStakingEnabledCommittee(instance)
	}

	spot := 0
	shouldBe := int(instance.NumShards()) * instance.NumNodesPerShard()

	total := 0
	for i := range superComm {
		total += len(superComm[i].NodeList)
	}

	if shouldBe != total {
		fmt.Println("Count mismatch", shouldBe, total)
	}

	allIdentities := make([]*bls.PublicKey, shouldBe)
	for i := range superComm {
		for j := range superComm[i].NodeList {
			identity := &bls.PublicKey{}
			superComm[i].NodeList[j].BlsPublicKey.ToLibBLSPublicKey(identity)
			allIdentities[spot] = identity
			spot++
		}
	}

	if shardID == StateID {
		return allIdentities, nil
	}

	subCommittee := superComm.FindCommitteeByID(uint32(shardID))
	subCommitteeIdentities := make([]*bls.PublicKey, len(subCommittee.NodeList))
	spot = 0
	for i := range subCommittee.NodeList {
		identity := &bls.PublicKey{}
		subCommittee.NodeList[i].BlsPublicKey.ToLibBLSPublicKey(identity)
		subCommitteeIdentities[spot] = identity
		spot++
	}

	return allIdentities, subCommitteeIdentities
}

func (def partialStakingEnabled) ReadPublicKeysFromDB(
	h common.Hash, reader DataProvider,
) ([]*bls.PublicKey, error) {
	header := reader.GetHeaderByHash(h)
	shardID := header.ShardID()
	superCommittee, err := reader.ReadShardState(header.Epoch())
	if err != nil {
		return nil, err
	}
	subCommittee := superCommittee.FindCommitteeByID(shardID)
	if subCommittee == nil {
		return nil, ctxerror.New("cannot find shard in the shard state",
			"blockNumber", header.Number(),
			"shardID", header.ShardID(),
		)
	}
	committerKeys := []*bls.PublicKey{}

	for i := range subCommittee.NodeList {
		committerKey := new(bls.PublicKey)
		err := subCommittee.NodeList[i].BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", subCommittee.NodeList[i].BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	return committerKeys, nil

	return nil, nil
}

func (def partialStakingEnabled) ReadFromDB(
	epoch *big.Int, reader DataProvider,
) (newSuperComm shard.State, err error) {
	return reader.ReadShardState(epoch)
}

// ReadFromComputation is single entry point for reading the State of the network
func (def partialStakingEnabled) Compute(
	epoch *big.Int, config params.ChainConfig, stakerReader DataProvider,
) (newSuperComm shard.State, err error) {
	instance := shard.Schedule.InstanceForEpoch(epoch)
	if !config.IsStaking(epoch) {
		return preStakingEnabledCommittee(instance), nil
	}
	fmt.Println("Staking epoch happened", config.String())
	return with400Stakers(instance, stakerReader)
}
