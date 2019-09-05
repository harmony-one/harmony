package drand

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"

	block2 "github.com/harmony-one/harmony/block"
	bls2 "github.com/harmony-one/harmony/crypto/bls"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())

	if !dRand.IsLeader {
		test.Error("dRand should belong to a leader")
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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())
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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())

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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())

	pubKey1 := bls2.RandPrivateKey().GetPublicKey()
	pubKey2 := bls2.RandPrivateKey().GetPublicKey()

	publicKeys := []*bls.PublicKey{pubKey1, pubKey2}

	if dRand.UpdatePublicKeys(publicKeys) != 2 {
		test.Error("Count of public keys doesn't match")
	}

	for index, publicKey := range dRand.PublicKeys {
		if strings.Compare(publicKey.SerializeToHexStr(), publicKeys[index].SerializeToHexStr()) != 0 {
			test.Error("Public keys not updated succssfully")
		}
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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_DRAND,
		Type:        msg_pb.MessageType_DRAND_INIT,
		Request: &msg_pb.Message_Drand{
			Drand: &msg_pb.DrandRequest{},
		},
	}
	drandMsg := message.GetDrand()
	drandMsg.SenderPubkey = dRand.pubKey.Serialize()
	drandMsg.BlockHash = dRand.blockHash[:]

	dRand.signDRandMessage(message)

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
	dRand := New(host, 0, []p2p.Peer{leader, validator}, leader, nil, bls2.RandPrivateKey())
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	txs := []*types.Transaction{tx1}

	block := types.NewBlock(block2.NewHeaderWith().Number(big.NewInt(314)).Header(), txs, nil, nil, nil)
	blockHash := block.Hash()

	dRand.vrf(blockHash)
}
