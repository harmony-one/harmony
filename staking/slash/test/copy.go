package slashtest

import (
	"math/big"

	"github.com/harmony-one/harmony/staking/slash"
)

// CopyRecord makes a deep copy the slash Record
func CopyRecord(r slash.Record) slash.Record {
	return slash.Record{
		Evidence: CopyEvidence(r.Evidence),
		Reporter: r.Reporter,
	}
}

// CopyEvidence makes a deep copy the slash evidence
func CopyEvidence(e slash.Evidence) slash.Evidence {
	return slash.Evidence{
		Moment:           CopyMoment(e.Moment),
		ConflictingVotes: CopyConflictingVotes(e.ConflictingVotes),
		Offender:         e.Offender,
	}
}

// CopyMoment makes a deep copy of the Moment structure
func CopyMoment(m slash.Moment) slash.Moment {
	cp := slash.Moment{
		ShardID: m.ShardID,
		Height:  m.Height,
		ViewID:  m.ViewID,
	}
	if m.Epoch != nil {
		cp.Epoch = new(big.Int).Set(m.Epoch)
	}
	return cp
}

// CopyConflictingVotes makes a deep copy of slash.ConflictingVotes
func CopyConflictingVotes(cv slash.ConflictingVotes) slash.ConflictingVotes {
	return slash.ConflictingVotes{
		FirstVote:  CopyVote(cv.FirstVote),
		SecondVote: CopyVote(cv.SecondVote),
	}
}

// CopyVote makes a deep copy of slash.Vote
func CopyVote(v slash.Vote) slash.Vote {
	cp := slash.Vote{
		SignerPubKeys:   v.SignerPubKeys,
		BlockHeaderHash: v.BlockHeaderHash,
	}
	if v.Signature != nil {
		cp.Signature = make([]byte, len(v.Signature))
		copy(cp.Signature, v.Signature)
	}
	return cp
}
