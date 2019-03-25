package node

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func prepareNode(t *testing.T) *Node {
	pubKey := bls.RandPrivateKey().GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, 0, []p2p.Peer{leader, validator}, leader, nil)
	return New(host, consensus, nil, false)

}

func TestAddLotteryContract(t *testing.T) {
	node := prepareNode(t)
	node.AddLotteryContract()
	if len(node.DemoContractAddress) == 0 {
		t.Error("Can not create demo contract")
	}
}
