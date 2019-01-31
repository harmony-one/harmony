package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	"github.com/harmony-one/harmony/p2p"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{IP: "127.0.0.1", Port: "19999"}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "55555"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msg := consensus.constructAnnounceMessage()

	if len(msg) != 93 {
		test.Errorf("Annouce message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructPreparedMessage(test *testing.T) {

	leaderPriKey, leaderPubKey := utils.GenKey("127.0.0.1", "6000")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "6000", PubKey: leaderPubKey}

	validatorPriKey, validatorPubKey := utils.GenKey("127.0.0.1", "5555")
	validator := p2p.Peer{IP: "127.0.0.1", Port: "5555", PubKey: validatorPubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		test.Fatalf("newhost failure: %v", err)
	}
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}

	message := "test string"
	(*consensus.prepareSigs)[0] = leaderPriKey.Sign(message)
	(*consensus.prepareSigs)[1] = validatorPriKey.Sign(message)
	consensus.prepareBitmap.SetKey(leaderPubKey, true)
	consensus.prepareBitmap.SetKey(validatorPubKey, true)

	msg, _ := consensus.constructPreparedMessage()

	if len(msg) != 144 {
		test.Errorf("Challenge message is not constructed in the correct size: %d", len(msg))
	}
}
