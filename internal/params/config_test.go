package params

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOneEpochBeforeHIP30(t *testing.T) {
	c := ChainConfig{
		HIP30Epoch: big.NewInt(3),
	}

	require.True(t, c.IsOneEpochBeforeHIP30(big.NewInt(2)))
	require.False(t, c.IsOneEpochBeforeHIP30(big.NewInt(3)))
}

func TestMainnetTBDFeaturesInactiveBeforeActivation(t *testing.T) {
	// Representative mainnet epoch well below every EpochTBD placeholder (10_000_000).
	epoch := big.NewInt(3000)
	cfg := MainnetChainConfig

	checks := []struct {
		name string
		got  bool
	}{
		{"CXMerkleProofReplayFix", cfg.IsCXMerkleProofReplayFixEpoch(epoch)},
		{"CXReceiptStateRollback", cfg.IsCXReceiptStateRollback(epoch)},
		{"RejectShard0CrossLink", cfg.IsRejectShard0CrossLink(epoch)},
		{"Allowlist", cfg.IsAllowlistEpoch(epoch)},
		{"LeaderRotationV2", cfg.IsLeaderRotationV2Epoch(epoch)},
		{"TimestampValidation", cfg.IsTimestampValidation(epoch)},
		{"DuplicateCrossLink", cfg.IsDuplicateCrossLinkRejection(epoch)},
		{"ShardStateValidation", cfg.IsShardStateValidation(epoch)},
		{"IsOneSecond", cfg.IsOneSecond(epoch)},
		{"EIP2537", cfg.IsEIP2537Precompile(epoch)},
		{"EIP1153", cfg.IsEIP1153TransientStorage(epoch)},
		{"EIP7939CLZ", cfg.IsEIP7939CLZ(epoch)},
		{"EIP5656", cfg.IsEIP5656Mcopy(epoch)},
		{"EIP3855", cfg.IsEIP3855(epoch)},
		{"EIP3860", cfg.IsEIP3860(epoch)},
		{"EIP6780", cfg.IsEIP6780(epoch)},
		{"Prague", cfg.IsPrague(epoch)},
		{"EIP8024", cfg.IsEIP8024(epoch)},
		{"ValidatorWrapperAddressBind", cfg.IsValidatorWrapperAddressBind(epoch)},
		{"SlashExternalStakeDenomFix", cfg.IsSlashExternalStakeDenomFix(epoch)},
		{"RejectDuplicateSlashEvidence", cfg.IsRejectDuplicateSlashEvidence(epoch)},
		{"SlashGroupOrderFix", cfg.IsSlashGroupOrderFix(epoch)},
		{"BLSProofBind", cfg.IsBLSProofBind(epoch)},
		{"SlashBallotSignerFix", cfg.IsSlashBallotSignerFix(epoch)},
		{"VerifyBeaconHeaderSlash", cfg.IsVerifyBeaconHeaderSlash(epoch)},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			require.False(t, check.got, "feature should be inactive before activation epoch")
		})
	}

	rules := cfg.Rules(epoch)
	require.False(t, rules.Is1153TransientStorage)
	require.False(t, rules.Is7939CLZ)
	require.False(t, rules.IsEIP5656Mcopy)
	require.False(t, rules.IsCXReceiptStateRollback)
}
