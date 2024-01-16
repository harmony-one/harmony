package quorum

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var _ Decider = threadSafeDeciderImpl{}

type threadSafeDeciderImpl struct {
	mu      *sync.RWMutex
	decider Decider
}

func NewThreadSafeDecider(decider Decider, mu *sync.RWMutex) Decider {
	return threadSafeDeciderImpl{
		mu:      mu,
		decider: decider,
	}
}

func (a threadSafeDeciderImpl) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.String()
}

func (a threadSafeDeciderImpl) Participants() multibls.PublicKeys {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.Participants()
}

func (a threadSafeDeciderImpl) IndexOf(key bls.SerializedPublicKey) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.IndexOf(key)
}

func (a threadSafeDeciderImpl) ParticipantsCount() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.ParticipantsCount()
}

func (a threadSafeDeciderImpl) NthNextValidator(slotList shard.SlotList, pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.NthNextValidator(slotList, pubKey, next)
}

func (a threadSafeDeciderImpl) NthNextHmy(instance shardingconfig.Instance, pubkey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.NthNextHmy(instance, pubkey, next)
}

func (a threadSafeDeciderImpl) NthNextHmyExt(instance shardingconfig.Instance, wrapper *bls.PublicKeyWrapper, i int) (bool, *bls.PublicKeyWrapper) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.NthNextHmyExt(instance, wrapper, i)
}

func (a threadSafeDeciderImpl) FirstParticipant(instance shardingconfig.Instance) *bls.PublicKeyWrapper {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.FirstParticipant(instance)
}

func (a threadSafeDeciderImpl) UpdateParticipants(pubKeys, allowlist []bls.PublicKeyWrapper) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.decider.UpdateParticipants(pubKeys, allowlist)
}

func (a threadSafeDeciderImpl) submitVote(p Phase, pubkeys []bls.SerializedPublicKey, sig *bls_core.Sign, headerHash common.Hash, height, viewID uint64) (*votepower.Ballot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.submitVote(p, pubkeys, sig, headerHash, height, viewID)
}

func (a threadSafeDeciderImpl) SignersCount(phase Phase) int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.SignersCount(phase)
}

func (a threadSafeDeciderImpl) reset(phases []Phase) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.decider.reset(phases)
}

func (a threadSafeDeciderImpl) ReadBallot(p Phase, pubkey bls.SerializedPublicKey) *votepower.Ballot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.ReadBallot(p, pubkey)
}

func (a threadSafeDeciderImpl) TwoThirdsSignersCount() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.TwoThirdsSignersCount()
}

func (a threadSafeDeciderImpl) AggregateVotes(p Phase) *bls_core.Sign {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.AggregateVotes(p)
}

func (a threadSafeDeciderImpl) SetVoters(subCommittee *shard.Committee, epoch *big.Int) (*TallyResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.SetVoters(subCommittee, epoch)
}

func (a threadSafeDeciderImpl) Policy() Policy {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.Policy()
}

func (a threadSafeDeciderImpl) AddNewVote(p Phase, pubkeys []*bls.PublicKeyWrapper, sig *bls_core.Sign, headerHash common.Hash, height, viewID uint64) (*votepower.Ballot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.AddNewVote(p, pubkeys, sig, headerHash, height, viewID)
}

func (a threadSafeDeciderImpl) IsQuorumAchievedByMask(mask *bls.Mask) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.IsQuorumAchievedByMask(mask)
}

func (a threadSafeDeciderImpl) QuorumThreshold() numeric.Dec {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.QuorumThreshold()
}

func (a threadSafeDeciderImpl) IsAllSigsCollected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.IsAllSigsCollected()
}

func (a threadSafeDeciderImpl) ResetPrepareAndCommitVotes() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.decider.ResetPrepareAndCommitVotes()
}

func (a threadSafeDeciderImpl) ResetViewChangeVotes() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.decider.ResetViewChangeVotes()
}

func (a threadSafeDeciderImpl) CurrentTotalPower(p Phase) (*numeric.Dec, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.CurrentTotalPower(p)
}

func (a threadSafeDeciderImpl) IsQuorumAchieved(p Phase) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.decider.IsQuorumAchieved(p)
}
