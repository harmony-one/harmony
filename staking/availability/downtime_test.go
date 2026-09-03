package availability

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/stretchr/testify/require"
)

// downtimeReader serves snapshots with a real epoch, which the fork gates need.
type downtimeReader struct {
	snapshots map[common.Address]*staking.ValidatorWrapper
	epoch     *big.Int
	config    *params.ChainConfig
}

func (r downtimeReader) ReadValidatorSnapshot(
	addr common.Address,
) (*staking.ValidatorSnapshot, error) {
	wrapper, ok := r.snapshots[addr]
	if !ok {
		return nil, errors.New("not a valid validator address")
	}
	return &staking.ValidatorSnapshot{Validator: wrapper, Epoch: r.epoch}, nil
}

func (r downtimeReader) Config() *params.ChainConfig { return r.config }

func oneToken(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(denominations.One))
}

// makeDowntimeValidator builds a wrapper that passes SanityCheck, with selfStake of its
// own and one external delegation, having signed `signed` of `toSign` blocks.
func makeDowntimeValidator(
	addr common.Address, selfStake, external *big.Int, signed, toSign int64,
) *staking.ValidatorWrapper {
	var key bls.SerializedPublicKey
	key[0] = addr[0]
	w := &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:            addr,
			SlotPubKeys:        []bls.SerializedPublicKey{key},
			MinSelfDelegation:  oneToken(10000),
			MaxTotalDelegation: oneToken(100000000),
			Status:             effective.Active,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					Rate:          numeric.MustNewDecFromStr("0.1"),
					MaxRate:       numeric.MustNewDecFromStr("0.9"),
					MaxChangeRate: numeric.MustNewDecFromStr("0.05"),
				},
			},
		},
		Delegations: staking.Delegations{
			staking.NewDelegation(addr, new(big.Int).Set(selfStake)),
			staking.NewDelegation(common.HexToAddress("0xdead"), new(big.Int).Set(external)),
		},
	}
	w.Counters.NumBlocksSigned = big.NewInt(signed)
	w.Counters.NumBlocksToSign = big.NewInt(toSign)
	return w
}

// zeroSnapshot is the epoch-start counter baseline, so current counters are the epoch.
func zeroSnapshot(w *staking.ValidatorWrapper) *staking.ValidatorWrapper {
	snap := *w
	snap.Counters.NumBlocksSigned = big.NewInt(0)
	snap.Counters.NumBlocksToSign = big.NewInt(0)
	return &snap
}

func downtimeFixture(
	t *testing.T, selfStake, external *big.Int, signed, toSign int64, slashEpoch *big.Int,
) (common.Address, *staking.ValidatorWrapper, downtimeReader, testStateDB) {
	t.Helper()
	addr := common.HexToAddress("0xb0b")
	wrapper := makeDowntimeValidator(addr, selfStake, external, signed, toSign)
	require.NoError(t, wrapper.SanityCheck(), "fixture must be a valid validator")
	reader := downtimeReader{
		snapshots: map[common.Address]*staking.ValidatorWrapper{addr: zeroSnapshot(wrapper)},
		epoch:     big.NewInt(10),
		config:    &params.ChainConfig{StakingEpoch: big.NewInt(0), DowntimeSlashEpoch: slashEpoch},
	}
	return addr, wrapper, reader, testStateDB{addr: wrapper}
}

func TestDowntimeSlashInactiveBeforeActivationEpoch(t *testing.T) {
	// EpochTBD stands in for a network that has not scheduled the fork.
	addr, wrapper, reader, state := downtimeFixture(
		t, oneToken(50000), oneToken(10000), 0, 1000, params.EpochTBD,
	)
	slashed, err := ComputeAndMutateDowntimeSlash(reader, state, addr, big.NewInt(10))
	require.NoError(t, err)
	require.Nil(t, slashed, "no slash before the activation epoch")
	require.Zero(t, oneToken(50000).Cmp(wrapper.Delegations[0].Amount))
}

func TestDowntimeSlashBySigningRatio(t *testing.T) {
	selfStake := oneToken(50000)
	// 0.1% of 50,000 ONE.
	expected := oneToken(50)

	for _, tc := range []struct {
		name           string
		signed, toSign int64
		wantSlash      bool
	}{
		{"never signed", 0, 1000, true},
		{"signed a handful", 50, 1000, true},
		{"exactly at the threshold", 333, 999, true},
		{"just above the threshold", 334, 999, false},
		{"present but unreliable", 500, 1000, false},
		{"below availability but well present", 600, 1000, false},
		{"fully available", 1000, 1000, false},
		{"nothing was assigned", 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, wrapper, reader, state := downtimeFixture(
				t, selfStake, oneToken(10000), tc.signed, tc.toSign, big.NewInt(0),
			)
			slashed, err := ComputeAndMutateDowntimeSlash(reader, state, addr, big.NewInt(10))
			require.NoError(t, err)

			if !tc.wantSlash {
				require.Nil(t, slashed)
				require.Zero(t, selfStake.Cmp(wrapper.Delegations[0].Amount))
				return
			}
			require.NotNil(t, slashed)
			require.Zero(t, expected.Cmp(slashed), "slashed %s want %s", slashed, expected)
			require.Zero(t,
				new(big.Int).Sub(selfStake, expected).Cmp(wrapper.Delegations[0].Amount),
			)
		})
	}
}

func TestDowntimeSlashLeavesDelegatorsAlone(t *testing.T) {
	external := oneToken(10000)
	addr, wrapper, reader, state := downtimeFixture(
		t, oneToken(50000), external, 0, 1000, big.NewInt(0),
	)
	_, err := ComputeAndMutateDowntimeSlash(reader, state, addr, big.NewInt(10))
	require.NoError(t, err)
	require.Zero(t, external.Cmp(wrapper.Delegations[1].Amount),
		"an external delegation must not pay for the validator's absence")
}

// A validator sitting on exactly the minimum self-delegation is the case that decides
// whether this rule can run at all: SanityCheck rejects an Active validator below the
// minimum, so the status has to move first.
func TestDowntimeSlashAtMinimumSelfDelegation(t *testing.T) {
	minimum := oneToken(10000)
	addr, wrapper, reader, state := downtimeFixture(
		t, minimum, oneToken(10000), 0, 1000, big.NewInt(0),
	)
	slashed, err := ComputeAndMutateDowntimeSlash(reader, state, addr, big.NewInt(10))
	require.NoError(t, err)
	require.NotNil(t, slashed)
	require.Zero(t, oneToken(10).Cmp(slashed))
	require.Equal(t, effective.Inactive, wrapper.Status)
	require.Equal(t, -1, wrapper.Delegations[0].Amount.Cmp(minimum))
}

func TestDowntimeSlashSkipsBannedValidator(t *testing.T) {
	selfStake := oneToken(50000)
	addr, wrapper, reader, state := downtimeFixture(
		t, selfStake, oneToken(10000), 0, 1000, big.NewInt(0),
	)
	wrapper.Status = effective.Banned
	slashed, err := ComputeAndMutateDowntimeSlash(reader, state, addr, big.NewInt(10))
	require.NoError(t, err)
	require.Nil(t, slashed, "a banned validator has already lost everything")
	require.Zero(t, selfStake.Cmp(wrapper.Delegations[0].Amount))
}

// The downtime threshold has to sit strictly below the availability threshold, otherwise
// every validator that loses its seat also pays stake, which is not the intent.
func TestDowntimeThresholdIsBelowAvailabilityThreshold(t *testing.T) {
	require.True(t, DowntimeSlashThreshold().LT(measure))
	require.True(t, DowntimeSlashRate().GT(numeric.ZeroDec()))
	require.True(t, DowntimeSlashRate().LT(numeric.MustNewDecFromStr("0.01")),
		"first release of slashing is meant to be gentle")
}

// recordingState notes which validators were marked for a write, so a test can tell a
// skipped validator from one that was slashed by zero.
type recordingState struct {
	testStateDB
	dirty map[common.Address]int
}

func (r recordingState) MarkValidatorWrapperDirty(addr common.Address) { r.dirty[addr]++ }

func recording(state testStateDB) recordingState {
	return recordingState{testStateDB: state, dirty: map[common.Address]int{}}
}

func TestDowntimeSlashSkipsValidatorWithoutSelfDelegation(t *testing.T) {
	addr, wrapper, reader, state := downtimeFixture(
		t, oneToken(50000), oneToken(10000), 0, 1000, big.NewInt(0),
	)
	wrapper.Delegations = nil
	rec := recording(state)

	slashed, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(10))
	require.NoError(t, err, "a validator holding no self delegation is skipped, not fatal")
	require.Nil(t, slashed)
	require.Empty(t, rec.dirty)
}

// A stake small enough that the rate truncates to nothing leaves the validator alone
// rather than recording a write of zero.
func TestDowntimeSlashSkipsStakeTooSmallToDivide(t *testing.T) {
	tiny := big.NewInt(500)
	addr, wrapper, reader, state := downtimeFixture(
		t, oneToken(50000), oneToken(10000), 0, 1000, big.NewInt(0),
	)
	wrapper.Delegations[0].Amount = tiny
	rec := recording(state)

	slashed, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(10))
	require.NoError(t, err)
	require.Nil(t, slashed)
	require.Zero(t, tiny.Cmp(wrapper.Delegations[0].Amount))
	require.Empty(t, rec.dirty)
}

// A wrapper that would not satisfy the validator rules once reduced is left exactly as
// it was found, so the write that follows is one the state will accept.
func TestDowntimeSlashLeavesUnsatisfiableValidatorUnchanged(t *testing.T) {
	selfStake := oneToken(50000)
	addr, wrapper, reader, state := downtimeFixture(
		t, selfStake, oneToken(10000), 0, 1000, big.NewInt(0),
	)
	// A rate above the maximum rate is rejected by the validator rules.
	wrapper.Rate = numeric.MustNewDecFromStr("0.99")
	rec := recording(state)

	slashed, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(10))
	require.NoError(t, err, "an unsatisfiable wrapper is skipped, not fatal")
	require.Nil(t, slashed)
	require.Zero(t, selfStake.Cmp(wrapper.Delegations[0].Amount))
	require.Equal(t, effective.Active, wrapper.Status, "status is restored with the stake")
	require.Empty(t, rec.dirty)
}

func TestDowntimeSlashIgnoresNonPositiveBlocksToSign(t *testing.T) {
	selfStake := oneToken(50000)
	addr, wrapper, reader, state := downtimeFixture(
		t, selfStake, oneToken(10000), 0, 0, big.NewInt(0),
	)
	wrapper.Counters.NumBlocksToSign = big.NewInt(-5)
	rec := recording(state)

	slashed, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(10))
	require.NoError(t, err)
	require.Nil(t, slashed)
	require.Zero(t, selfStake.Cmp(wrapper.Delegations[0].Amount))
	require.Empty(t, rec.dirty)
}

// Each absent epoch takes its share of what is left, so a validator that keeps
// returning while still absent pays steadily more in total and never reaches zero.
func TestDowntimeSlashCompoundsOverEpochs(t *testing.T) {
	start := oneToken(50000)
	addr, wrapper, reader, state := downtimeFixture(
		t, start, oneToken(10000), 0, 1000, big.NewInt(0),
	)
	rec := recording(state)

	first, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(10))
	require.NoError(t, err)
	second, err := ComputeAndMutateDowntimeSlash(reader, rec, addr, big.NewInt(11))
	require.NoError(t, err)

	require.Equal(t, 1, first.Cmp(second), "the second epoch takes its share of less")
	remaining := new(big.Int).Sub(start, new(big.Int).Add(first, second))
	require.Zero(t, remaining.Cmp(wrapper.Delegations[0].Amount))
	require.Equal(t, 1, wrapper.Delegations[0].Amount.Sign())
	require.Equal(t, 2, rec.dirty[addr])
}
