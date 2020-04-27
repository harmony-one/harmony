package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
)

func TestNew(test *testing.T) {
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := New(
		nil, shard.BeaconChainShardID, multibls.GetPrivateKey(bls.RandPrivateKey()), decider,
	)
	if err != nil {
		test.Fatalf("Cannot craeate consensus: %v", err)
	}

}
