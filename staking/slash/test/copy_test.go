package slashtest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/staking/slash"
)

func TestCopyRecord(t *testing.T) {
	tests := []slash.Record{
		makeNonZeroRecord(),
		makeZeroRecord(),
		slash.Record{},
	}
	for i, test := range tests {
		cp := CopyRecord(test)
		if err := assertRecordDeepCopy(cp, test); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func assertRecordDeepCopy(r1, r2 slash.Record) error {
	if !reflect.DeepEqual(r1, r2) {
		return fmt.Errorf("not deep equal")
	}
	if r1.Evidence.Epoch != nil && r1.Evidence.Epoch == r2.Evidence.Epoch {
		return fmt.Errorf("epoch not deep copy")
	}
	if err := assertVoteDeepCopy(r1.Evidence.FirstVote, r2.Evidence.FirstVote); err != nil {
		return fmt.Errorf("FirstVote: %v", err)
	}
	if err := assertVoteDeepCopy(r1.Evidence.SecondVote, r2.Evidence.SecondVote); err != nil {
		return fmt.Errorf("SecondVote: %v", err)
	}
	return nil
}

func assertVoteDeepCopy(v1, v2 slash.Vote) error {
	if !reflect.DeepEqual(v1, v2) {
		return fmt.Errorf("not deep equal")
	}
	if len(v1.Signature) != 0 && &v1.Signature[0] == &v2.Signature[0] {
		return fmt.Errorf("signature same pointer")
	}
	return nil
}

func makeNonZeroRecord() slash.Record {
	return slash.Record{
		Evidence: slash.Evidence{
			Moment: nonZeroMoment,
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote:  nonZeroVote1,
				SecondVote: nonZeroVote2,
			},
			Offender: common.BigToAddress(common.Big2),
		},
	}
}

func makeZeroRecord() slash.Record {
	return slash.Record{
		Evidence: slash.Evidence{
			Moment: ZeroMoment,
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote:  zeroVote,
				SecondVote: zeroVote,
			},
		},
	}
}

var (
	nonZeroMoment = slash.Moment{
		Epoch:   common.Big1,
		ShardID: 1,
		Height:  2,
		ViewID:  3,
	}

	ZeroMoment = slash.Moment{
		Epoch: common.Big0,
	}

	nonZeroVote1 = slash.Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{{1}},
		BlockHeaderHash: common.Hash{2},
		Signature:       []byte{1, 2, 3},
	}

	nonZeroVote2 = slash.Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{{3}},
		BlockHeaderHash: common.Hash{4},
		Signature:       []byte{4, 5, 6},
	}

	zeroVote = slash.Vote{
		Signature: make([]byte, 0),
	}
)
