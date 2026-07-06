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

func TestBloomEpochConfigured(t *testing.T) {
	require.Equal(t, big.NewInt(7414), TestnetChainConfig.BloomEpoch)
	require.Equal(t, big.NewInt(53508), PartnerChainConfig.BloomEpoch)
	require.Equal(t, big.NewInt(5), LocalnetChainConfig.BloomEpoch)

	require.True(t, TestnetChainConfig.BloomEpoch.Cmp(maxEpoch(testnetBloomFeatureEpochs())) >= 0)
	require.True(t, PartnerChainConfig.BloomEpoch.Cmp(maxEpoch(devnetBloomFeatureEpochs())) >= 0)
	require.True(t, LocalnetChainConfig.BloomEpoch.Cmp(maxEpoch(localnetBloomFeatureEpochs())) >= 0)

	for _, epoch := range localnetBloomFeatureEpochs() {
		require.Equal(t, big.NewInt(5), epoch)
	}
}

func testnetBloomFeatureEpochs() []*big.Int {
	return []*big.Int{
		TestnetChainConfig.CXMerkleProofReplayFixEpoch,
		TestnetChainConfig.CXReceiptStateRollbackEpoch,
		TestnetChainConfig.TimestampValidationEpoch,
		TestnetChainConfig.SlashExternalStakeDenomFixEpoch,
		TestnetChainConfig.DuplicateCrossLinkEpoch,
		TestnetChainConfig.ShardStateValidationEpoch,
		TestnetChainConfig.RejectShard0CrossLinkEpoch,
		TestnetChainConfig.EIP2537PrecompileEpoch,
		TestnetChainConfig.EIP1153TransientStorageEpoch,
		TestnetChainConfig.EIP7939CLZEpoch,
		TestnetChainConfig.EIP5656McopyEpoch,
		TestnetChainConfig.EIP3855Epoch,
		TestnetChainConfig.EIP3860Epoch,
		TestnetChainConfig.EIP6780Epoch,
		TestnetChainConfig.EIP8024Epoch,
		TestnetChainConfig.RejectDuplicateSlashEvidenceEpoch,
		TestnetChainConfig.AllowlistEpoch,
		TestnetChainConfig.LeaderRotationV2Epoch,
		TestnetChainConfig.SlashGroupOrderFixEpoch,
		TestnetChainConfig.ValidatorWrapperAddressBindEpoch,
		TestnetChainConfig.BLSProofBindEpoch,
		TestnetChainConfig.SlashBallotSignerFixEpoch,
		TestnetChainConfig.VerifyBeaconHeaderSlashEpoch,
	}
}

func devnetBloomFeatureEpochs() []*big.Int {
	return []*big.Int{
		PartnerChainConfig.CXMerkleProofReplayFixEpoch,
		PartnerChainConfig.CXReceiptStateRollbackEpoch,
		PartnerChainConfig.TimestampValidationEpoch,
		PartnerChainConfig.SlashExternalStakeDenomFixEpoch,
		PartnerChainConfig.DuplicateCrossLinkEpoch,
		PartnerChainConfig.ShardStateValidationEpoch,
		PartnerChainConfig.RejectShard0CrossLinkEpoch,
		PartnerChainConfig.EIP2537PrecompileEpoch,
		PartnerChainConfig.EIP1153TransientStorageEpoch,
		PartnerChainConfig.EIP7939CLZEpoch,
		PartnerChainConfig.EIP5656McopyEpoch,
		PartnerChainConfig.EIP3855Epoch,
		PartnerChainConfig.EIP3860Epoch,
		PartnerChainConfig.EIP6780Epoch,
		PartnerChainConfig.EIP8024Epoch,
		PartnerChainConfig.RejectDuplicateSlashEvidenceEpoch,
		PartnerChainConfig.LeaderRotationV2Epoch,
		PartnerChainConfig.SlashGroupOrderFixEpoch,
		PartnerChainConfig.ValidatorWrapperAddressBindEpoch,
		PartnerChainConfig.BLSProofBindEpoch,
		PartnerChainConfig.SlashBallotSignerFixEpoch,
		PartnerChainConfig.VerifyBeaconHeaderSlashEpoch,
	}
}

func localnetBloomFeatureEpochs() []*big.Int {
	return []*big.Int{
		LocalnetChainConfig.CXMerkleProofReplayFixEpoch,
		LocalnetChainConfig.CXReceiptStateRollbackEpoch,
		LocalnetChainConfig.TimestampValidationEpoch,
		LocalnetChainConfig.SlashExternalStakeDenomFixEpoch,
		LocalnetChainConfig.DuplicateCrossLinkEpoch,
		LocalnetChainConfig.ShardStateValidationEpoch,
		LocalnetChainConfig.RejectShard0CrossLinkEpoch,
		LocalnetChainConfig.EIP2537PrecompileEpoch,
		LocalnetChainConfig.EIP1153TransientStorageEpoch,
		LocalnetChainConfig.EIP7939CLZEpoch,
		LocalnetChainConfig.EIP5656McopyEpoch,
		LocalnetChainConfig.EIP3855Epoch,
		LocalnetChainConfig.EIP3860Epoch,
		LocalnetChainConfig.EIP6780Epoch,
		LocalnetChainConfig.EIP8024Epoch,
		LocalnetChainConfig.RejectDuplicateSlashEvidenceEpoch,
		LocalnetChainConfig.AllowlistEpoch,
		LocalnetChainConfig.LeaderRotationV2Epoch,
		LocalnetChainConfig.SlashGroupOrderFixEpoch,
		LocalnetChainConfig.ValidatorWrapperAddressBindEpoch,
		LocalnetChainConfig.BLSProofBindEpoch,
		LocalnetChainConfig.SlashBallotSignerFixEpoch,
		LocalnetChainConfig.VerifyBeaconHeaderSlashEpoch,
	}
}

func maxEpoch(epochs []*big.Int) *big.Int {
	max := epochs[0]
	for _, epoch := range epochs[1:] {
		if epoch.Cmp(max) > 0 {
			max = epoch
		}
	}
	return max
}
