package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKeys(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
	pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)
	priKeyWrapper := bls.PrivateKeyWrapper{blsPriKey, &pubKeyWrapper}
	if _, err = consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, []*bls.PrivateKeyWrapper{&priKeyWrapper}); err != nil {
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
	host, err := p2p.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandPrivateKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKeys(blsPriKey), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.ResetState()
	consensus.UpdateBitmaps()
	consensus.blockHash = [32]byte{}

	message := "test string"
	leaderKey := bls.SerializedPublicKey{}
	leaderKey.FromLibBLSPublicKey(leaderPubKey)
	validatorKey := bls.SerializedPublicKey{}
	validatorKey.FromLibBLSPublicKey(validatorPubKey)
	consensus.Decider.SubmitVote(
		quorum.Prepare,
		[]bls.SerializedPublicKey{leaderKey},
		leaderPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.blockNum,
		consensus.GetCurViewID(),
	)
	if _, err := consensus.Decider.SubmitVote(
		quorum.Prepare,
		[]bls.SerializedPublicKey{validatorKey},
		validatorPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.blockNum,
		consensus.GetCurViewID(),
	); err != nil {
		test.Log(err)
	}

	// According to RJ these failures are benign.
	if err := consensus.prepareBitmap.SetKey(leaderKey, true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}
	if err := consensus.prepareBitmap.SetKey(validatorKey, true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}

	pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
	pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)
	priKeyWrapper := bls.PrivateKeyWrapper{blsPriKey, &pubKeyWrapper}
	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil, []*bls.PrivateKeyWrapper{&priKeyWrapper})
	if err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if network.Phase != msg_pb.MessageType_PREPARED {
		test.Error("it did not created prepared message")
	}
}
