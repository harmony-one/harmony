package consensus

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999", ConsensusPubKey: bls.RandSecretKey().PublicKey()}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandSecretKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, []bls.SecretKey{blsPriKey}, decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	if _, err = consensus.construct(msg_pb.MessageType_ANNOUNCE, nil, []bls.SecretKey{blsPriKey}); err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
}

func TestConstructPreparedMessage(test *testing.T) {
	leaderPriKey := bls.RandSecretKey()
	leaderPubKey := leaderPriKey.PublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999", ConsensusPubKey: leaderPubKey}

	validatorPriKey := bls.RandSecretKey()
	validatorPubKey := leaderPriKey.PublicKey()
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	blsPriKey := bls.RandSecretKey()
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, []bls.SecretKey{blsPriKey}, decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.ResetState()
	consensus.UpdateBitmaps()
	consensus.blockHash = [32]byte{}

	message := []byte("test string")
	consensus.Decider.AddNewVote(
		quorum.Prepare,
		[]bls.PublicKey{leaderPubKey},
		leaderPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.blockNum,
		consensus.GetCurBlockViewID(),
	)
	if _, err := consensus.Decider.AddNewVote(
		quorum.Prepare,
		[]bls.PublicKey{validatorPubKey},
		validatorPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.blockNum,
		consensus.GetCurBlockViewID(),
	); err != nil {
		test.Log(err)
	}

	// According to RJ these failures are benign.
	if err := consensus.prepareBitmap.SetKey(leaderPubKey.Serialized(), true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}
	if err := consensus.prepareBitmap.SetKey(validatorPubKey.Serialized(), true); err != nil {
		test.Log(errors.New("prepareBitmap.SetKey"))
	}

	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil, []bls.SecretKey{blsPriKey})
	if err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if network.MessageType != msg_pb.MessageType_PREPARED {
		test.Error("it did not create prepared message")
	}
}

func TestConstructPrepareMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999", ConsensusPubKey: bls.RandSecretKey().PublicKey()}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}

	blsPriKey1 := bls.RandSecretKey()
	pubKeyWrapper1 := blsPriKey1.PublicKey()

	blsPriKey2 := bls.RandSecretKey()
	pubKeyWrapper2 := blsPriKey2.PublicKey()

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, shard.BeaconChainShardID,
	)

	consensus, err := New(
		host, shard.BeaconChainShardID, leader, []bls.SecretKey{blsPriKey1}, decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.UpdatePublicKeys([]bls.PublicKey{pubKeyWrapper1, pubKeyWrapper2})

	consensus.SetCurBlockViewID(2)
	consensus.blockHash = [32]byte{}
	copy(consensus.blockHash[:], []byte("random"))
	consensus.blockNum = 1000

	sig := blsPriKey1.Sign(consensus.blockHash[:])
	network, err := consensus.construct(msg_pb.MessageType_PREPARE, nil, []bls.SecretKey{blsPriKey1})

	if err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
	if network.MessageType != msg_pb.MessageType_PREPARE {
		test.Errorf("MessageType is not populated correctly")
	}
	if network.FBFTMsg.BlockHash != consensus.blockHash {
		test.Errorf("BlockHash is not populated correctly")
	}
	if network.FBFTMsg.ViewID != consensus.GetCurBlockViewID() {
		test.Errorf("ViewID is not populated correctly")
	}
	if len(network.FBFTMsg.SenderPubkeys) != 1 && network.FBFTMsg.SenderPubkeys[0].Serialized() != pubKeyWrapper1.Serialized() {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, sig.ToBytes()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}

	keys := []bls.SecretKey{blsPriKey1, blsPriKey2}

	signatures := []bls.Signature{}
	for _, priKey := range keys {
		if s := priKey.Sign(consensus.blockHash[:]); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	network, err = consensus.construct(msg_pb.MessageType_PREPARE, nil, keys)

	if err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
	if network.MessageType != msg_pb.MessageType_PREPARE {
		test.Errorf("MessageType is not populated correctly")
	}
	if network.FBFTMsg.BlockHash != consensus.blockHash {
		test.Errorf("BlockHash is not populated correctly")
	}
	if network.FBFTMsg.ViewID != consensus.GetCurBlockViewID() {
		test.Errorf("ViewID is not populated correctly")
	}
	if len(network.FBFTMsg.SenderPubkeys) != 2 && (network.FBFTMsg.SenderPubkeys[0].Serialized() != pubKeyWrapper1.Serialized() || network.FBFTMsg.SenderPubkeys[1].Serialized() != pubKeyWrapper2.Serialized()) {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, aggSig.ToBytes()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.SenderPubkeyBitmap, []byte{0x03}) != 0 {
		test.Errorf("SenderPubkeyBitmap is not populated correctly")
	}
}

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999", ConsensusPubKey: bls.RandSecretKey().PublicKey()}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}

	blsPriKey1 := bls.RandSecretKey()
	pubKeyWrapper1 := blsPriKey1.PublicKey()

	blsPriKey2 := bls.RandSecretKey()
	pubKeyWrapper2 := blsPriKey2.PublicKey()

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, shard.BeaconChainShardID,
	)

	consensus, err := New(
		host, shard.BeaconChainShardID, leader, []bls.SecretKey{blsPriKey1}, decider,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.UpdatePublicKeys([]bls.PublicKey{pubKeyWrapper1, pubKeyWrapper2})

	consensus.SetCurBlockViewID(2)
	consensus.blockHash = [32]byte{}
	copy(consensus.blockHash[:], []byte("random"))
	consensus.blockNum = 1000

	sigPayload := []byte("payload")

	sig := blsPriKey1.Sign(sigPayload)
	network, err := consensus.construct(msg_pb.MessageType_COMMIT, sigPayload, []bls.SecretKey{blsPriKey1})

	if err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
	if network.MessageType != msg_pb.MessageType_COMMIT {
		test.Errorf("MessageType is not populated correctly")
	}
	if network.FBFTMsg.BlockHash != consensus.blockHash {
		test.Errorf("BlockHash is not populated correctly")
	}
	if network.FBFTMsg.ViewID != consensus.GetCurBlockViewID() {
		test.Errorf("ViewID is not populated correctly")
	}
	if len(network.FBFTMsg.SenderPubkeys) != 1 && network.FBFTMsg.SenderPubkeys[0].Serialized() != pubKeyWrapper1.Serialized() {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, sig.ToBytes()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}

	keys := []bls.SecretKey{blsPriKey1, blsPriKey2}

	signatures := []bls.Signature{}
	for _, priKey := range keys {
		if s := priKey.Sign(sigPayload); s != nil {
			signatures = append(signatures, s)
		}
	}
	aggSig := bls.AggreagateSignatures(signatures)

	network, err = consensus.construct(msg_pb.MessageType_COMMIT, sigPayload, keys)

	if err != nil {
		test.Fatalf("could not construct announce: %v", err)
	}
	if network.MessageType != msg_pb.MessageType_COMMIT {
		test.Errorf("MessageType is not populated correctly")
	}
	if network.FBFTMsg.BlockHash != consensus.blockHash {
		test.Errorf("BlockHash is not populated correctly")
	}
	if network.FBFTMsg.ViewID != consensus.GetCurBlockViewID() {
		test.Errorf("ViewID is not populated correctly")
	}
	if len(network.FBFTMsg.SenderPubkeys) != 2 && (network.FBFTMsg.SenderPubkeys[0].Serialized() != pubKeyWrapper1.Serialized() || network.FBFTMsg.SenderPubkeys[1].Serialized() != pubKeyWrapper2.Serialized()) {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, aggSig.ToBytes()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.SenderPubkeyBitmap, []byte{0x03}) != 0 {
		test.Errorf("SenderPubkeyBitmap is not populated correctly")
	}
}

func TestPopulateMessageFields(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902", ConsensusPubKey: bls.RandSecretKey().PublicKey()}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	blsPriKey := bls.RandSecretKey()
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := New(
		host, shard.BeaconChainShardID, leader, []bls.SecretKey{blsPriKey}, decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.SetCurBlockViewID(2)
	blockHash := [32]byte{}
	consensus.blockHash = blockHash

	msg := &msg_pb.Message{
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := consensus.populateMessageFieldsAndSender(msg.GetConsensus(), consensus.blockHash[:],
		blsPriKey.PublicKey().Serialized())

	if consensusMsg.ViewId != 2 {
		t.Errorf("Consensus ID is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.BlockHash[:], blockHash[:]) {
		t.Errorf("Block hash is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.SenderPubkey, blsPriKey.PublicKey().ToBytes()) {
		t.Errorf("Sender ID is not populated correctly")
	}
	if len(consensusMsg.SenderPubkeyBitmap) > 0 {
		t.Errorf("SenderPubkeyBitmap should not be populated for func populateMessageFieldsAndSender")
	}

	msg = &msg_pb.Message{
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	bitmap := []byte("random bitmap")
	consensusMsg = consensus.populateMessageFieldsAndSendersBitmap(msg.GetConsensus(), consensus.blockHash[:],
		bitmap)

	if consensusMsg.ViewId != 2 {
		t.Errorf("Consensus ID is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.BlockHash[:], blockHash[:]) {
		t.Errorf("Block hash is not populated correctly")
	}
	if bytes.Compare(consensusMsg.SenderPubkeyBitmap, bitmap) != 0 {
		t.Errorf("SenderPubkeyBitmap is not populated correctly")
	}
	if len(consensusMsg.SenderPubkey) > 0 {
		t.Errorf("SenderPubkey should not be populated for func populateMessageFieldsAndSendersBitmap")
	}
}
