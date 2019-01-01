package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/p2p"
)

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	host := p2pimpl.NewHost(leader)
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	_, msg := consensus.constructCommitMessage(consensus_proto.MessageType_COMMIT)

	if len(msg) != 139 {
		test.Errorf("Commit message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructResponseMessage(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	host := p2pimpl.NewHost(leader)
	consensus := New(host, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msg := consensus.constructResponseMessage(consensus_proto.MessageType_RESPONSE, crypto.Ed25519Curve.Scalar())

	if len(msg) != 139 {
		test.Errorf("Response message is not constructed in the correct size: %d", len(msg))
	}
}
