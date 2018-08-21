package consensus

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/p2p"
	consensus_proto "github.com/simple-rules/harmony-benchmark/proto/consensus"
)

func TestConstructCommitMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port: "2"}
	validator := p2p.Peer{Ip: "3", Port: "5"}
	consensus := NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	_, msg := consensus.constructCommitMessage(consensus_proto.COMMIT)

	if len(msg) != 1+1+1+4+32+2+32+64 {
		test.Errorf("Commit message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructResponseMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port: "2"}
	validator := p2p.Peer{Ip: "3", Port: "5"}
	consensus := NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	msg := consensus.constructResponseMessage(consensus_proto.RESPONSE, crypto.Ed25519Curve.Scalar())

	if len(msg) != 1+1+1+4+32+2+32+64 {
		test.Errorf("Response message is not constructed in the correct size: %d", len(msg))
	}
}
