package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/stretchr/testify/require"
)

func offenderRecord(offender common.Address, viewID uint64) slash.Record {
	return slash.Record{
		Evidence: slash.Evidence{
			Moment: slash.Moment{
				Epoch:   big.NewInt(1),
				ShardID: 0,
				Height:  100,
				ViewID:  viewID,
			},
			Offender: offender,
		},
		Reporter: common.HexToAddress("0xreporter"),
	}
}

func TestFirstRecordPerOffenderKeepsOneRecordEach(t *testing.T) {
	alice := common.HexToAddress("0xa11ce")
	bob := common.HexToAddress("0xb0b")
	slashed := map[common.Address]struct{}{}

	kept := firstRecordPerOffender(slash.Records{
		offenderRecord(alice, 1),
		offenderRecord(bob, 1),
		offenderRecord(alice, 2),
	}, slashed)

	require.Len(t, kept, 2)
	require.Equal(t, alice, kept[0].Evidence.Offender)
	require.Equal(t, bob, kept[1].Evidence.Offender)
	require.Equal(t, uint64(1), kept[0].Evidence.ViewID, "the first record for an offender survives")
	require.Len(t, slashed, 2)
}

// The set carries across groups, so a validator leaving conflicting ballots at more than
// one view of a height still answers once.
func TestFirstRecordPerOffenderCarriesAcrossGroups(t *testing.T) {
	alice := common.HexToAddress("0xa11ce")
	slashed := map[common.Address]struct{}{}

	firstGroup := firstRecordPerOffender(slash.Records{offenderRecord(alice, 1)}, slashed)
	secondGroup := firstRecordPerOffender(slash.Records{offenderRecord(alice, 2)}, slashed)

	require.Len(t, firstGroup, 1)
	require.Empty(t, secondGroup)
}

func TestFirstRecordPerOffenderOnEmptyInput(t *testing.T) {
	slashed := map[common.Address]struct{}{}
	require.Empty(t, firstRecordPerOffender(nil, slashed))
	require.Empty(t, firstRecordPerOffender(slash.Records{}, slashed))
	require.Empty(t, slashed)
}

// Distinct offenders in one group are all kept, so grouping never costs a validator's
// slash because another validator shares its moment.
func TestFirstRecordPerOffenderKeepsDistinctOffenders(t *testing.T) {
	slashed := map[common.Address]struct{}{}
	records := slash.Records{
		offenderRecord(common.HexToAddress("0x1"), 1),
		offenderRecord(common.HexToAddress("0x2"), 1),
		offenderRecord(common.HexToAddress("0x3"), 1),
	}
	require.Len(t, firstRecordPerOffender(records, slashed), 3)
}
