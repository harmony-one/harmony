package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"github.com/harmony-one/harmony/shard"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKey(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	if _, err = consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, blsPriKey.GetPublicKey(), blsPriKey); err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
}

func TestConstructPreparedMessage(test *testing.T) {
	leaderPriKey := bls.RandPrivateKey()
	leaderPubKey := leaderPriKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999", ConsensusPubKey: leaderPubKey}

	validatorPriKey := bls.RandPrivateKey()
	validatorPubKey := leaderPriKey.GetPublicKey()
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKey(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.ResetState()
	consensus.blockHash = [32]byte{}

	message := "test string"
	consensus.Decider.SubmitVote(
		quorum.Prepare,
		leaderPubKey,
		leaderPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
	)
	consensus.Decider.SubmitVote(
		quorum.Prepare,
		validatorPubKey,
		validatorPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
	)

	// According to RJ these failures are benign.
	if err := consensus.prepareBitmap.SetKey(leaderPubKey, true); err != nil {
		test.Log(ctxerror.New("prepareBitmap.SetKey").WithCause(err))
	}
	if err := consensus.prepareBitmap.SetKey(validatorPubKey, true); err != nil {
		test.Log(ctxerror.New("prepareBitmap.SetKey").WithCause(err))
	}

	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil, blsPriKey.GetPublicKey(), blsPriKey)
	if err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if network.Phase != msg_pb.MessageType_PREPARED {
		test.Error("it did not created prepared message")
	}
}
