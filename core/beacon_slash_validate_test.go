package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/stretchr/testify/require"
)

// reporterVariantSlashRecords returns two slash records that share identical
// signed evidence but use different reporter addresses (bug07 reporter-variant clone).
func reporterVariantSlashRecords(t *testing.T) (slash.Records, common.Hash) {
	t.Helper()

	evidence := slash.Evidence{
		Moment: slash.Moment{
			Epoch:   big.NewInt(4),
			ShardID: shard.BeaconChainShardID,
			Height:  37,
			ViewID:  38,
		},
		ConflictingVotes: slash.ConflictingVotes{
			FirstVote: slash.Vote{
				BlockHeaderHash: common.HexToHash("0x01"),
			},
			SecondVote: slash.Vote{
				BlockHeaderHash: common.HexToHash("0x02"),
			},
		},
		Offender: common.HexToAddress("0x0000000000000000000000000000000000000b22"),
	}
	evidenceHash := hash.FromRLPNew256(evidence)

	return slash.Records{
		{Evidence: evidence, Reporter: common.HexToAddress("0x0000000000000000000000000000000000000c33")},
		{Evidence: evidence, Reporter: common.HexToAddress("0x0000000000000000000000000000000000000d44")},
	}, evidenceHash
}

func TestCheckBeaconSlashEvidenceUniqueness_RejectsReporterVariants(t *testing.T) {
	records, _ := reporterVariantSlashRecords(t)
	cfg := params.TestChainConfig

	err := checkBeaconSlashEvidenceUniqueness(cfg, big.NewInt(0), records)
	require.ErrorIs(t, err, errInvalidBeaconSlashPayload)
}

func TestCheckBeaconSlashEvidenceUniqueness_SkippedBeforeFork(t *testing.T) {
	records, _ := reporterVariantSlashRecords(t)
	cfg := params.MainnetChainConfig

	err := checkBeaconSlashEvidenceUniqueness(cfg, big.NewInt(2963), records)
	require.NoError(t, err)
}

func TestCheckBeaconSlashEvidenceUniqueness_AllowsSingleRecord(t *testing.T) {
	records, _ := reporterVariantSlashRecords(t)
	cfg := params.TestChainConfig

	err := checkBeaconSlashEvidenceUniqueness(cfg, big.NewInt(0), slash.Records{records[0]})
	require.NoError(t, err)
}

func TestCheckBeaconSlashEvidenceUniqueness_AllowsDistinctEvidence(t *testing.T) {
	base, _ := reporterVariantSlashRecords(t)
	other := base[0]
	other.Evidence.ConflictingVotes.SecondVote.BlockHeaderHash = common.HexToHash("0x03")

	records := slash.Records{base[0], other}
	cfg := params.TestChainConfig

	err := checkBeaconSlashEvidenceUniqueness(cfg, big.NewInt(0), records)
	require.NoError(t, err)
}

func TestCheckBeaconSlashEvidenceUniqueness_RejectsSwappedVoteEvidence(t *testing.T) {
	base, evidenceHash := reporterVariantSlashRecords(t)
	swapped := base[0].Evidence
	tmp := swapped.ConflictingVotes.FirstVote
	swapped.ConflictingVotes.FirstVote = swapped.ConflictingVotes.SecondVote
	swapped.ConflictingVotes.SecondVote = tmp

	records := slash.Records{
		base[0],
		{Evidence: swapped, Reporter: common.HexToAddress("0x0000000000000000000000000000000000000e55")},
	}
	cfg := params.TestChainConfig

	err := checkBeaconSlashEvidenceUniqueness(cfg, big.NewInt(0), records)
	require.ErrorIs(t, err, errInvalidBeaconSlashPayload)
	require.Equal(t, evidenceHash, hash.FromRLPNew256(records[0].Evidence))
}

func TestCheckBeaconHeaderSlashEvidence_SkippedBeforeFork(t *testing.T) {
	records, _ := reporterVariantSlashRecords(t)
	cfg := params.MainnetChainConfig

	err := checkBeaconHeaderSlashEvidence(cfg, nil, nil, big.NewInt(2963), records)
	require.NoError(t, err)
}

func TestCheckBeaconHeaderSlashEvidence_SkippedWhenEmpty(t *testing.T) {
	cfg := params.TestChainConfig

	err := checkBeaconHeaderSlashEvidence(cfg, nil, nil, big.NewInt(0), nil)
	require.NoError(t, err)
}
