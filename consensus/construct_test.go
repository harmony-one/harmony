package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

func TestConstructAnnounceMessage(test *testing.T) {

	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		nil, shard.BeaconChainShardID, multibls.GetPrivateKey(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	if _, err = consensus.construct(
		msg_pb.MessageType_ANNOUNCE, nil, blsPriKey.GetPublicKey(), blsPriKey,
	); err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
}

func TestConstructPreparedMessage(test *testing.T) {
	leaderPriKey := bls.RandPrivateKey()
	leaderPubKey := leaderPriKey.GetPublicKey()
	validatorPriKey := bls.RandPrivateKey()
	validatorPubKey := leaderPriKey.GetPublicKey()
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		nil, shard.BeaconChainShardID, multibls.GetPrivateKey(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.ResetState()
	consensus.blockHash = [32]byte{}
	num := consensus.BlockNum()
	viewID := consensus.ViewID()
	message := "test string"
	consensus.Decider.SubmitVote(
		quorum.Prepare,
		leaderPubKey,
		leaderPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		num,
		viewID,
	)
	if _, err := consensus.Decider.SubmitVote(
		quorum.Prepare,
		validatorPubKey,
		validatorPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		num,
		viewID,
	); err != nil {
		test.Log(err)
	}

	// According to RJ these failures are benign.
	if err := consensus.prepareBitmap.SetKey(leaderPubKey, true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}
	if err := consensus.prepareBitmap.SetKey(validatorPubKey, true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}

	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil, blsPriKey.GetPublicKey(), blsPriKey)
	if err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if network.Phase != msg_pb.MessageType_PREPARED {
		test.Error("it did not created prepared message")
	}
}
