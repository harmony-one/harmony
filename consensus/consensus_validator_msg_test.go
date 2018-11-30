package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/p2p"
	consensus_proto "github.com/harmony-one/harmony/proto/consensus"
)

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := New(leader, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	_, msg := consensus.constructCommitMessage(consensus_proto.Commit)

	if len(msg) != 1+1+1+4+32+2+32+64 {
		test.Errorf("Commit message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructResponseMessage(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := New(leader, "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msg := consensus.constructResponseMessage(consensus_proto.Response, crypto.Ed25519Curve.Scalar())

	if len(msg) != 1+1+1+4+32+2+32+64 {
		test.Errorf("Response message is not constructed in the correct size: %d", len(msg))
	}
}
