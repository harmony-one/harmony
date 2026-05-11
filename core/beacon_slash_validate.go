package core

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/staking/slash"
)

var errInvalidBeaconSlashPayload = errors.New("invalid beacon slash payload in header")

// checkBeaconSlashEvidenceUniqueness enforces canonical uniqueness for decoded
// beacon slash records when the chain schedule enables the rule. Caller runs this
// from StateProcessor.Process before the processor result cache and before
// transaction execution so all execution paths apply the same checks.
func checkBeaconSlashEvidenceUniqueness(cfg *params.ChainConfig, epoch *big.Int, records slash.Records) error {
	if cfg == nil || !cfg.IsRejectDuplicateSlashEvidence(epoch) || len(records) < 2 {
		return nil
	}
	seen := make(map[common.Hash]struct{}, len(records)*2)
	for i := range records {
		ev := &records[i].Evidence
		h := hash.FromRLPNew256(ev)
		if _, ok := seen[h]; ok {
			return errInvalidBeaconSlashPayload
		}
		swapEv := *ev
		tmp := swapEv.ConflictingVotes.FirstVote
		swapEv.ConflictingVotes.FirstVote = swapEv.ConflictingVotes.SecondVote
		swapEv.ConflictingVotes.SecondVote = tmp
		sh := hash.FromRLPNew256(&swapEv)
		if _, ok := seen[sh]; ok {
			return errInvalidBeaconSlashPayload
		}
		seen[h] = struct{}{}
		seen[sh] = struct{}{}
	}
	return nil
}
