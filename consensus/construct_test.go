package consensus

import (
	"bytes"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
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
	blsPriKey := bls.RandPrivateKey()
	reg := registry.New()
	consensus, err := New(host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey), reg, decider, 3, false)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.blockHash = [32]byte{}
	pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
	pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)
	priKeyWrapper := bls.PrivateKeyWrapper{Pri: blsPriKey, Pub: &pubKeyWrapper}
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
	blsPriKey := bls.RandPrivateKey()
	reg := registry.New()
	consensus, err := New(host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey), reg, decider, 3, false)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}
	consensus.resetState()
	consensus.updateBitmaps()
	consensus.blockHash = [32]byte{}

	message := "test string"
	leaderKey := bls.SerializedPublicKey{}
	leaderKey.FromLibBLSPublicKey(leaderPubKey)
	leaderKeyWrapper := bls.PublicKeyWrapper{Object: leaderPubKey, Bytes: leaderKey}
	validatorKey := bls.SerializedPublicKey{}
	validatorKey.FromLibBLSPublicKey(validatorPubKey)
	validatorKeyWrapper := bls.PublicKeyWrapper{Object: validatorPubKey, Bytes: validatorKey}
	consensus.Decider.AddNewVote(
		quorum.Prepare,
		[]*bls.PublicKeyWrapper{&leaderKeyWrapper},
		leaderPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.BlockNum(),
		consensus.GetCurBlockViewID(),
	)
	if _, err := consensus.Decider.AddNewVote(
		quorum.Prepare,
		[]*bls.PublicKeyWrapper{&validatorKeyWrapper},
		validatorPriKey.Sign(message),
		common.BytesToHash(consensus.blockHash[:]),
		consensus.BlockNum(),
		consensus.GetCurBlockViewID(),
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
	priKeyWrapper := bls.PrivateKeyWrapper{Pri: blsPriKey, Pub: &pubKeyWrapper}
	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil, []*bls.PrivateKeyWrapper{&priKeyWrapper})
	if err != nil {
		test.Errorf("Error when creating prepared message")
	}
	if network.MessageType != msg_pb.MessageType_PREPARED {
		test.Error("it did not create prepared message")
	}
}

func TestConstructPrepareMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}

	blsPriKey1 := bls.RandPrivateKey()
	pubKeyWrapper1 := bls.PublicKeyWrapper{Object: blsPriKey1.GetPublicKey()}
	pubKeyWrapper1.Bytes.FromLibBLSPublicKey(pubKeyWrapper1.Object)
	priKeyWrapper1 := bls.PrivateKeyWrapper{Pri: blsPriKey1, Pub: &pubKeyWrapper1}

	blsPriKey2 := bls.RandPrivateKey()
	pubKeyWrapper2 := bls.PublicKeyWrapper{Object: blsPriKey2.GetPublicKey()}
	pubKeyWrapper2.Bytes.FromLibBLSPublicKey(pubKeyWrapper2.Object)
	priKeyWrapper2 := bls.PrivateKeyWrapper{Pri: blsPriKey2, Pub: &pubKeyWrapper2}

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, shard.BeaconChainShardID,
	)

	consensus, err := New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey1), registry.New(), decider, 3, false,
	)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.UpdatePublicKeys([]bls.PublicKeyWrapper{pubKeyWrapper1, pubKeyWrapper2}, []bls.PublicKeyWrapper{})

	consensus.SetCurBlockViewID(2)
	consensus.blockHash = [32]byte{}
	copy(consensus.blockHash[:], []byte("random"))
	atomic.StoreUint64(&consensus.blockNum, 1000)

	sig := priKeyWrapper1.Pri.SignHash(consensus.blockHash[:])
	network, err := consensus.construct(msg_pb.MessageType_PREPARE, nil, []*bls.PrivateKeyWrapper{&priKeyWrapper1})

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
	if len(network.FBFTMsg.SenderPubkeys) != 1 && network.FBFTMsg.SenderPubkeys[0].Bytes != pubKeyWrapper1.Bytes {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, sig.Serialize()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}

	keys := []*bls.PrivateKeyWrapper{&priKeyWrapper1, &priKeyWrapper2}
	aggSig := bls_core.Sign{}
	for _, priKey := range keys {
		if s := priKey.Pri.SignHash(consensus.blockHash[:]); s != nil {
			aggSig.Add(s)
		}
	}
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
	if len(network.FBFTMsg.SenderPubkeys) != 2 && (network.FBFTMsg.SenderPubkeys[0].Bytes != pubKeyWrapper1.Bytes || network.FBFTMsg.SenderPubkeys[1].Bytes != pubKeyWrapper2.Bytes) {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, aggSig.Serialize()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.SenderPubkeyBitmap, []byte{0x03}) != 0 {
		test.Errorf("SenderPubkeyBitmap is not populated correctly")
	}
}

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}

	blsPriKey1 := bls.RandPrivateKey()
	pubKeyWrapper1 := bls.PublicKeyWrapper{Object: blsPriKey1.GetPublicKey()}
	pubKeyWrapper1.Bytes.FromLibBLSPublicKey(pubKeyWrapper1.Object)
	priKeyWrapper1 := bls.PrivateKeyWrapper{Pri: blsPriKey1, Pub: &pubKeyWrapper1}

	blsPriKey2 := bls.RandPrivateKey()
	pubKeyWrapper2 := bls.PublicKeyWrapper{Object: blsPriKey2.GetPublicKey()}
	pubKeyWrapper2.Bytes.FromLibBLSPublicKey(pubKeyWrapper2.Object)
	priKeyWrapper2 := bls.PrivateKeyWrapper{Pri: blsPriKey2, Pub: &pubKeyWrapper2}

	decider := quorum.NewDecider(
		quorum.SuperMajorityStake, shard.BeaconChainShardID,
	)

	consensus, err := New(host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey1), registry.New(), decider, 3, false)
	if err != nil {
		test.Fatalf("Cannot create consensus: %v", err)
	}
	consensus.UpdatePublicKeys([]bls.PublicKeyWrapper{pubKeyWrapper1, pubKeyWrapper2}, []bls.PublicKeyWrapper{})

	consensus.SetCurBlockViewID(2)
	consensus.blockHash = [32]byte{}
	copy(consensus.blockHash[:], []byte("random"))
	atomic.StoreUint64(&consensus.blockNum, 1000)

	sigPayload := []byte("payload")

	sig := priKeyWrapper1.Pri.SignHash(sigPayload)
	network, err := consensus.construct(msg_pb.MessageType_COMMIT, sigPayload, []*bls.PrivateKeyWrapper{&priKeyWrapper1})

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
	if len(network.FBFTMsg.SenderPubkeys) != 1 && network.FBFTMsg.SenderPubkeys[0].Bytes != pubKeyWrapper1.Bytes {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, sig.Serialize()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}

	keys := []*bls.PrivateKeyWrapper{&priKeyWrapper1, &priKeyWrapper2}
	aggSig := bls_core.Sign{}
	for _, priKey := range keys {
		if s := priKey.Pri.SignHash(sigPayload); s != nil {
			aggSig.Add(s)
		}
	}
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
	if len(network.FBFTMsg.SenderPubkeys) != 2 && (network.FBFTMsg.SenderPubkeys[0].Bytes != pubKeyWrapper1.Bytes || network.FBFTMsg.SenderPubkeys[1].Bytes != pubKeyWrapper2.Bytes) {
		test.Errorf("SenderPubkeys is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.Payload, aggSig.Serialize()) != 0 {
		test.Errorf("Payload is not populated correctly")
	}
	if bytes.Compare(network.FBFTMsg.SenderPubkeyBitmap, []byte{0x03}) != 0 {
		test.Errorf("SenderPubkeyBitmap is not populated correctly")
	}
}

func TestPopulateMessageFields(t *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(p2p.HostConfig{
		Self:   &leader,
		BLSKey: priKey,
	})
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	blsPriKey := bls.RandPrivateKey()
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := New(
		host, shard.BeaconChainShardID, multibls.GetPrivateKeys(blsPriKey), registry.New(), decider, 3, false,
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

	keyBytes := bls.SerializedPublicKey{}
	keyBytes.FromLibBLSPublicKey(blsPriKey.GetPublicKey())
	consensusMsg := consensus.populateMessageFieldsAndSender(msg.GetConsensus(), consensus.blockHash[:],
		keyBytes)

	if consensusMsg.ViewId != 2 {
		t.Errorf("Consensus ID is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.BlockHash[:], blockHash[:]) {
		t.Errorf("Block hash is not populated correctly")
	}
	if !bytes.Equal(consensusMsg.SenderPubkey, blsPriKey.GetPublicKey().Serialize()) {
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
