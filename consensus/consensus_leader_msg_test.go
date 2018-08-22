package consensus

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/p2p"
	consensus_proto "github.com/simple-rules/harmony-benchmark/proto/consensus"
)

func TestConstructAnnounceMessage(test *testing.T) {
	leader := p2p.Peer{Ip: "1", Port: "2"}
	validator := p2p.Peer{Ip: "3", Port: "5"}
	consensus := NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	header := consensus.blockHeader
	msg := consensus.constructAnnounceMessage()

	if len(msg) != 1+1+1+4+32+2+64+len(header) {
		test.Errorf("Annouce message is not constructed in the correct size: %d", len(msg))
	}
}

func TestConstructChallengeMessage(test *testing.T) {
	leaderPriKey := crypto.Ed25519Curve.Scalar()
	priKeyInBytes := crypto.HashSha256("12")
	leaderPriKey.UnmarshalBinary(priKeyInBytes[:])
	leaderPubKey := pki.GetPublicKeyFromScalar(leaderPriKey)
	leader := p2p.Peer{Ip: "1", Port: "2", PubKey: leaderPubKey}

	validatorPriKey := crypto.Ed25519Curve.Scalar()
	priKeyInBytes = crypto.HashSha256("12")
	validatorPriKey.UnmarshalBinary(priKeyInBytes[:])
	validatorPubKey := pki.GetPublicKeyFromScalar(leaderPriKey)
	validator := p2p.Peer{Ip: "3", Port: "5", PubKey: validatorPubKey}

	consensus := NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)
	consensus.blockHash = [32]byte{}
	(*consensus.commitments)[0] = leaderPubKey
	(*consensus.commitments)[1] = validatorPubKey
	consensus.bitmap.SetKey(leaderPubKey, true)
	consensus.bitmap.SetKey(validatorPubKey, true)

	msg, _ := consensus.constructChallengeMessage(consensus_proto.CHALLENGE)

	if len(msg) != 1+1+1+4+32+2+33+33+32+64 {
		test.Errorf("Annouce message is not constructed in the correct size: %d", len(msg))
	}
}
