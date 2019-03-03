package drand

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestNew(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	if !dRand.IsLeader {
		test.Error("dRand should belong to a leader")
	}
}

func TestGetValidatorPeers(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	if !dRand.IsLeader {
		test.Error("dRand should belong to a leader")
	}

	countValidatorPeers := len(dRand.GetValidatorPeers())

	if countValidatorPeers != 2 {
		test.Error("Count of validator peers doesn't match")
	}
}

func TestAddPeers(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	if !dRand.IsLeader {
		test.Error("dRand should belong to a leader")
	}

	newPeer := p2p.Peer{IP: "127.0.0.1", Port: "9907"}
	countValidatorPeers := dRand.AddPeers([]*p2p.Peer{&newPeer})

	if countValidatorPeers != 1 {
		test.Error("Unable to add new peer")
	}
}

func TestGetValidatorByPeerId(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	if !dRand.IsLeader {
		test.Error("dRand should belong to a leader")
	}

	validatorID := utils.GetUniqueIDFromPeer(validator)

	if dRand.getValidatorPeerByID(validatorID) == nil {
		test.Error("Unable to get validator by Peerid")
	}
	newPeer := p2p.Peer{IP: "127.0.0.1", Port: "9907"}
	newPeerID := utils.GetUniqueIDFromPeer(newPeer)

	if dRand.getValidatorPeerByID(newPeerID) != nil {
		test.Error("Found validator for absent validatorId")
	}
}

func TestResetState(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)
	dRand.ResetState()
}

func TestSetLeaderPubKey(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	_, newPublicKey, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	newPublicKeyBytes, _ := newPublicKey.Bytes()
	if dRand.SetLeaderPubKey(newPublicKeyBytes) == nil {
		test.Error("Failed to Set Public Key of Leader")
	}
}

func TestUpdatePublicKeys(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)

	_, pubKey1 := utils.GenKey("127.0.0.1", "5555")
	_, pubKey2 := utils.GenKey("127.0.0.1", "6666")

	if dRand.UpdatePublicKeys([]*bls.PublicKey{pubKey1, pubKey2}) != 2 {
		test.Error("Count of public keys doesn't match")
	}
}

func TestVerifyMessageSig(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)
	message := drand_proto.Message{}
	message.Type = drand_proto.MessageType_INIT
	message.SenderId = dRand.nodeID

	message.BlockHash = dRand.blockHash[:]
	dRand.signDRandMessage(&message)

	if verifyMessageSig(dRand.pubKey, message) != nil {
		test.Error("Failed to verify the signature")
	}
}

func TestVrf(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9902"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9905"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	dRand := New(host, "0", []p2p.Peer{leader, validator}, leader, nil, true)
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	txs := []*types.Transaction{tx1}

	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil)
	blockHash := block.Hash()

	dRand.vrf(blockHash)
}
