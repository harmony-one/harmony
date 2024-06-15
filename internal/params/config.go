package params

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// Well-known chain IDs.
var (
	MainnetChainID            = big.NewInt(1)
	TestnetChainID            = big.NewInt(2)
	PangaeaChainID            = big.NewInt(3)
	PartnerChainID            = big.NewInt(4)
	StressnetChainID          = big.NewInt(5)
	TestChainID               = big.NewInt(99)  // not a real network
	AllProtocolChangesChainID = big.NewInt(100) // not a real network

	// EthMainnetShard0ChainID to be reserved unique chain ID for eth compatible chains.
	EthMainnetShard0ChainID            = big.NewInt(1666600000)
	EthTestnetShard0ChainID            = big.NewInt(1666700000)
	EthPangaeaShard0ChainID            = big.NewInt(1666800000)
	EthPartnerShard0ChainID            = big.NewInt(1666900000)
	EthStressnetShard0ChainID          = big.NewInt(1667000000)
	EthTestShard0ChainID               = big.NewInt(1667100000) // not a real network
	EthAllProtocolChangesShard0ChainID = big.NewInt(1667200000) // not a real network
)

// EpochTBD is a large, “not anytime soon” epoch.  It used as a placeholder
// until the exact epoch is decided.
var EpochTBD = big.NewInt(10000000)
var once sync.Once

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:                               MainnetChainID,
		EthCompatibleChainID:                  EthMainnetShard0ChainID,
		EthCompatibleShard0ChainID:            EthMainnetShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(442), // Around Thursday Feb 4th 2020, 10AM PST
		CrossTxEpoch:                          big.NewInt(28),
		CrossLinkEpoch:                        big.NewInt(186),
		AggregatedRewardEpoch:                 big.NewInt(689), // Around Wed Sept 15th 2021 with 3.5s block time
		StakingEpoch:                          big.NewInt(186),
		PreStakingEpoch:                       big.NewInt(185),
		QuickUnlockEpoch:                      big.NewInt(191),
		FiveSecondsEpoch:                      big.NewInt(230),
		TwoSecondsEpoch:                       big.NewInt(366), // Around Tuesday Dec 8th 2020, 8AM PST
		SixtyPercentEpoch:                     big.NewInt(530), // Around Monday Apr 12th 2021, 22:30 UTC
		RedelegationEpoch:                     big.NewInt(290),
		NoEarlyUnlockEpoch:                    big.NewInt(530), // Around Monday Apr 12th 2021, 22:30 UTC
		VRFEpoch:                              big.NewInt(631), // Around Wed July 7th 2021
		PrevVRFEpoch:                          big.NewInt(689), // Around Wed Sept 15th 2021 with 3.5s block time
		MinDelegation100Epoch:                 big.NewInt(631), // Around Wed July 7th 2021
		MinCommissionRateEpoch:                big.NewInt(631), // Around Wed July 7th 2021
		MinCommissionPromoPeriod:              big.NewInt(100),
		EPoSBound35Epoch:                      big.NewInt(631), // Around Wed July 7th 2021
		EIP155Epoch:                           big.NewInt(28),
		S3Epoch:                               big.NewInt(28),
		DataCopyFixEpoch:                      big.NewInt(689), // Around Wed Sept 15th 2021 with 3.5s block time
		IstanbulEpoch:                         big.NewInt(314),
		ReceiptLogEpoch:                       big.NewInt(101),
		SHA3Epoch:                             big.NewInt(725),  // Around Mon Oct 11 2021, 19:00 UTC
		HIP6And8Epoch:                         big.NewInt(725),  // Around Mon Oct 11 2021, 19:00 UTC
		StakingPrecompileEpoch:                big.NewInt(871),  // Around Tue Feb 11 2022
		ChainIdFixEpoch:                       big.NewInt(1323), // Around Wed 8 Feb 11:30PM UTC
		SlotsLimitedEpoch:                     big.NewInt(999),  // Around Fri, 27 May 2022 09:41:02 UTC with 2s block time
		CrossShardXferPrecompileEpoch:         big.NewInt(1323), // Around Wed 8 Feb 11:30PM UTC
		AllowlistEpoch:                        EpochTBD,
		LeaderRotationInternalValidatorsEpoch: EpochTBD,
		LeaderRotationExternalValidatorsEpoch: EpochTBD,
		FeeCollectEpoch:                       big.NewInt(1535), // 2023-07-20 05:51:07+00:00
		ValidatorCodeFixEpoch:                 big.NewInt(1535), // 2023-07-20 05:51:07+00:00
		HIP30Epoch:                            big.NewInt(1673), // 2023-11-02 17:30:00+00:00
		BlockGas30MEpoch:                      big.NewInt(1673), // 2023-11-02 17:30:00+00:00
		TopMaxRateEpoch:                       big.NewInt(1976), // around 2024-06-20 00:06:05  UTC
		MaxRateEpoch:                          big.NewInt(1733),
		DevnetExternalEpoch:                   EpochTBD,
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:                               TestnetChainID,
		EthCompatibleChainID:                  EthTestnetShard0ChainID,
		EthCompatibleShard0ChainID:            EthTestnetShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(2),
		AggregatedRewardEpoch:                 big.NewInt(2),
		StakingEpoch:                          big.NewInt(2),
		PreStakingEpoch:                       big.NewInt(1),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(2),
		SixtyPercentEpoch:                     big.NewInt(2),
		RedelegationEpoch:                     big.NewInt(2),
		NoEarlyUnlockEpoch:                    big.NewInt(2),
		VRFEpoch:                              big.NewInt(2),
		PrevVRFEpoch:                          big.NewInt(2),
		MinDelegation100Epoch:                 big.NewInt(2),
		MinCommissionRateEpoch:                big.NewInt(2),
		MinCommissionPromoPeriod:              big.NewInt(2),
		EPoSBound35Epoch:                      big.NewInt(2),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		ReceiptLogEpoch:                       big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         big.NewInt(2),
		StakingPrecompileEpoch:                big.NewInt(2),
		SlotsLimitedEpoch:                     big.NewInt(2),
		ChainIdFixEpoch:                       big.NewInt(0),
		CrossShardXferPrecompileEpoch:         big.NewInt(2),
		AllowlistEpoch:                        big.NewInt(2),
		LeaderRotationInternalValidatorsEpoch: EpochTBD,
		LeaderRotationExternalValidatorsEpoch: EpochTBD,
		FeeCollectEpoch:                       big.NewInt(1296), // 2023-04-28 07:14:20+00:00
		ValidatorCodeFixEpoch:                 big.NewInt(1296), // 2023-04-28 07:14:20+00:00
		HIP30Epoch:                            big.NewInt(2176), // 2023-10-12 10:00:00+00:00
		BlockGas30MEpoch:                      big.NewInt(2176), // 2023-10-12 10:00:00+00:00
		TopMaxRateEpoch:                       EpochTBD,
		MaxRateEpoch:                          EpochTBD,
		DevnetExternalEpoch:                   EpochTBD,
	}
	// PangaeaChainConfig contains the chain parameters for the Pangaea network.
	// All features except for CrossLink are enabled at launch.
	PangaeaChainConfig = &ChainConfig{
		ChainID:                               PangaeaChainID,
		EthCompatibleChainID:                  EthPangaeaShard0ChainID,
		EthCompatibleShard0ChainID:            EthPangaeaShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(2),
		AggregatedRewardEpoch:                 big.NewInt(3),
		StakingEpoch:                          big.NewInt(2),
		PreStakingEpoch:                       big.NewInt(1),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(0),
		SixtyPercentEpoch:                     big.NewInt(0),
		RedelegationEpoch:                     big.NewInt(0),
		NoEarlyUnlockEpoch:                    big.NewInt(0),
		VRFEpoch:                              big.NewInt(0),
		PrevVRFEpoch:                          big.NewInt(0),
		MinDelegation100Epoch:                 big.NewInt(0),
		MinCommissionRateEpoch:                big.NewInt(0),
		MinCommissionPromoPeriod:              big.NewInt(10),
		EPoSBound35Epoch:                      big.NewInt(0),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		ReceiptLogEpoch:                       big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         big.NewInt(0),
		StakingPrecompileEpoch:                big.NewInt(2), // same as staking
		ChainIdFixEpoch:                       big.NewInt(0),
		SlotsLimitedEpoch:                     EpochTBD, // epoch to enable HIP-16
		CrossShardXferPrecompileEpoch:         big.NewInt(1),
		AllowlistEpoch:                        EpochTBD,
		LeaderRotationInternalValidatorsEpoch: EpochTBD,
		LeaderRotationExternalValidatorsEpoch: EpochTBD,
		FeeCollectEpoch:                       EpochTBD,
		ValidatorCodeFixEpoch:                 EpochTBD,
		HIP30Epoch:                            EpochTBD,
		BlockGas30MEpoch:                      big.NewInt(0),
		TopMaxRateEpoch:                       EpochTBD,
		MaxRateEpoch:                          EpochTBD,
		DevnetExternalEpoch:                   EpochTBD,
	}

	// PartnerChainConfig contains the chain parameters for the Partner network.
	// This is the Devnet config
	PartnerChainConfig = &ChainConfig{
		ChainID:                               PartnerChainID,
		EthCompatibleChainID:                  EthPartnerShard0ChainID,
		EthCompatibleShard0ChainID:            EthPartnerShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(2),
		AggregatedRewardEpoch:                 big.NewInt(3),
		StakingEpoch:                          big.NewInt(2),
		PreStakingEpoch:                       big.NewInt(1),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(0),
		SixtyPercentEpoch:                     EpochTBD,
		RedelegationEpoch:                     big.NewInt(0),
		NoEarlyUnlockEpoch:                    big.NewInt(0),
		VRFEpoch:                              big.NewInt(0),
		PrevVRFEpoch:                          big.NewInt(0),
		MinDelegation100Epoch:                 big.NewInt(0),
		MinCommissionRateEpoch:                big.NewInt(0),
		MinCommissionPromoPeriod:              big.NewInt(10),
		EPoSBound35Epoch:                      big.NewInt(0),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		ReceiptLogEpoch:                       big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         big.NewInt(0),
		StakingPrecompileEpoch:                big.NewInt(5),
		ChainIdFixEpoch:                       big.NewInt(5),
		SlotsLimitedEpoch:                     EpochTBD, // epoch to enable HIP-16
		CrossShardXferPrecompileEpoch:         big.NewInt(5),
		AllowlistEpoch:                        EpochTBD,
		LeaderRotationInternalValidatorsEpoch: big.NewInt(144),
		LeaderRotationExternalValidatorsEpoch: big.NewInt(144),
		FeeCollectEpoch:                       big.NewInt(5),
		ValidatorCodeFixEpoch:                 big.NewInt(5),
		HIP30Epoch:                            big.NewInt(7),
		BlockGas30MEpoch:                      big.NewInt(7),
		TopMaxRateEpoch:                       EpochTBD,
		MaxRateEpoch:                          EpochTBD,
		DevnetExternalEpoch:                   big.NewInt(144),
	}

	// StressnetChainConfig contains the chain parameters for the Stress test network.
	// All features except for CrossLink are enabled at launch.
	StressnetChainConfig = &ChainConfig{
		ChainID:                               StressnetChainID,
		EthCompatibleChainID:                  EthStressnetShard0ChainID,
		EthCompatibleShard0ChainID:            EthStressnetShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(2),
		AggregatedRewardEpoch:                 big.NewInt(3),
		StakingEpoch:                          big.NewInt(2),
		PreStakingEpoch:                       big.NewInt(1),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(0),
		SixtyPercentEpoch:                     big.NewInt(10),
		RedelegationEpoch:                     big.NewInt(0),
		NoEarlyUnlockEpoch:                    big.NewInt(0),
		VRFEpoch:                              big.NewInt(0),
		PrevVRFEpoch:                          big.NewInt(0),
		MinDelegation100Epoch:                 big.NewInt(0),
		MinCommissionRateEpoch:                big.NewInt(0),
		MinCommissionPromoPeriod:              big.NewInt(10),
		EPoSBound35Epoch:                      big.NewInt(0),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		ReceiptLogEpoch:                       big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         big.NewInt(0),
		StakingPrecompileEpoch:                big.NewInt(2),
		ChainIdFixEpoch:                       big.NewInt(0),
		SlotsLimitedEpoch:                     EpochTBD, // epoch to enable HIP-16
		CrossShardXferPrecompileEpoch:         big.NewInt(1),
		AllowlistEpoch:                        EpochTBD,
		FeeCollectEpoch:                       EpochTBD,
		LeaderRotationInternalValidatorsEpoch: EpochTBD,
		LeaderRotationExternalValidatorsEpoch: EpochTBD,
		ValidatorCodeFixEpoch:                 EpochTBD,
		HIP30Epoch:                            EpochTBD,
		BlockGas30MEpoch:                      big.NewInt(0),
		TopMaxRateEpoch:                       EpochTBD,
		MaxRateEpoch:                          EpochTBD,
		DevnetExternalEpoch:                   EpochTBD,
	}

	// LocalnetChainConfig contains the chain parameters to run for local development.
	LocalnetChainConfig = &ChainConfig{
		ChainID:                               TestnetChainID,
		EthCompatibleChainID:                  EthTestnetShard0ChainID,
		EthCompatibleShard0ChainID:            EthTestnetShard0ChainID,
		EthCompatibleEpoch:                    big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(2),
		AggregatedRewardEpoch:                 big.NewInt(3),
		StakingEpoch:                          big.NewInt(2),
		PreStakingEpoch:                       big.NewInt(0),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(0),
		SixtyPercentEpoch:                     EpochTBD, // Never enable it for localnet as localnet has no external validator setup
		RedelegationEpoch:                     big.NewInt(0),
		NoEarlyUnlockEpoch:                    big.NewInt(0),
		VRFEpoch:                              big.NewInt(0),
		PrevVRFEpoch:                          big.NewInt(0),
		MinDelegation100Epoch:                 big.NewInt(0),
		MinCommissionRateEpoch:                big.NewInt(0),
		MinCommissionPromoPeriod:              big.NewInt(10),
		EPoSBound35Epoch:                      big.NewInt(0),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		ReceiptLogEpoch:                       big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         EpochTBD, // Never enable it for localnet as localnet has no external validator setup
		StakingPrecompileEpoch:                big.NewInt(2),
		ChainIdFixEpoch:                       big.NewInt(0),
		SlotsLimitedEpoch:                     EpochTBD, // epoch to enable HIP-16
		CrossShardXferPrecompileEpoch:         big.NewInt(1),
		AllowlistEpoch:                        EpochTBD,
		LeaderRotationInternalValidatorsEpoch: big.NewInt(5),
		LeaderRotationExternalValidatorsEpoch: big.NewInt(6),
		FeeCollectEpoch:                       big.NewInt(2),
		ValidatorCodeFixEpoch:                 big.NewInt(2),
		HIP30Epoch:                            EpochTBD,
		BlockGas30MEpoch:                      big.NewInt(0),
		TopMaxRateEpoch:                       EpochTBD,
		MaxRateEpoch:                          EpochTBD,
		DevnetExternalEpoch:                   EpochTBD,
	}

	// AllProtocolChanges ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllProtocolChanges = &ChainConfig{
		AllProtocolChangesChainID,          // ChainID
		EthAllProtocolChangesShard0ChainID, // EthCompatibleChainID
		EthAllProtocolChangesShard0ChainID, // EthCompatibleShard0ChainID
		big.NewInt(0),                      // EthCompatibleEpoch
		big.NewInt(0),                      // CrossTxEpoch
		big.NewInt(0),                      // CrossLinkEpoch
		big.NewInt(0),                      // AggregatedRewardEpoch
		big.NewInt(0),                      // StakingEpoch
		big.NewInt(0),                      // PreStakingEpoch
		big.NewInt(0),                      // QuickUnlockEpoch
		big.NewInt(0),                      // FiveSecondsEpoch
		big.NewInt(0),                      // TwoSecondsEpoch
		big.NewInt(0),                      // SixtyPercentEpoch
		big.NewInt(0),                      // RedelegationEpoch
		big.NewInt(0),                      // NoEarlyUnlockEpoch
		big.NewInt(0),                      // VRFEpoch
		big.NewInt(0),                      // PrevVRFEpoch
		big.NewInt(0),                      // MinDelegation100Epoch
		big.NewInt(0),                      // MinCommissionRateEpoch
		big.NewInt(10),                     // MinCommissionPromoPeriod
		big.NewInt(0),                      // EPoSBound35Epoch
		big.NewInt(0),                      // EIP155Epoch
		big.NewInt(0),                      // S3Epoch
		big.NewInt(0),                      // DataCopyFixEpoch
		big.NewInt(0),                      // IstanbulEpoch
		big.NewInt(0),                      // ReceiptLogEpoch
		big.NewInt(0),                      // SHA3Epoch
		big.NewInt(0),                      // HIP6And8Epoch
		big.NewInt(0),                      // StakingPrecompileEpoch
		big.NewInt(0),                      // ChainIdFixEpoch
		big.NewInt(0),                      // SlotsLimitedEpoch
		big.NewInt(1),                      // CrossShardXferPrecompileEpoch
		big.NewInt(0),                      // AllowlistEpoch
		big.NewInt(1),                      // LeaderRotationExternalNonBeaconLeaders
		big.NewInt(1),                      // LeaderRotationExternalBeaconLeaders
		big.NewInt(0),                      // FeeCollectEpoch
		big.NewInt(0),                      // ValidatorCodeFixEpoch
		big.NewInt(0),                      // BlockGas30M
		big.NewInt(0),                      // BlockGas30M
		big.NewInt(0),                      // MaxRateEpoch
		big.NewInt(0),
		big.NewInt(0),
	}

	// TestChainConfig ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	TestChainConfig = &ChainConfig{
		TestChainID,          // ChainID
		EthTestShard0ChainID, // EthCompatibleChainID
		EthTestShard0ChainID, // EthCompatibleShard0ChainID
		big.NewInt(0),        // EthCompatibleEpoch
		big.NewInt(0),        // CrossTxEpoch
		big.NewInt(0),        // CrossLinkEpoch
		big.NewInt(0),        // AggregatedRewardEpoch
		big.NewInt(0),        // StakingEpoch
		big.NewInt(0),        // PreStakingEpoch
		big.NewInt(0),        // QuickUnlockEpoch
		big.NewInt(0),        // FiveSecondsEpoch
		big.NewInt(0),        // TwoSecondsEpoch
		big.NewInt(0),        // SixtyPercentEpoch
		big.NewInt(0),        // RedelegationEpoch
		big.NewInt(0),        // NoEarlyUnlockEpoch
		big.NewInt(0),        // VRFEpoch
		big.NewInt(0),        // PrevVRFEpoch
		big.NewInt(0),        // MinDelegation100Epoch
		big.NewInt(0),        // MinCommissionRateEpoch
		big.NewInt(10),       // MinCommissionPromoPeriod
		big.NewInt(0),        // EPoSBound35Epoch
		big.NewInt(0),        // EIP155Epoch
		big.NewInt(0),        // S3Epoch
		big.NewInt(0),        // DataCopyFixEpoch
		big.NewInt(0),        // IstanbulEpoch
		big.NewInt(0),        // ReceiptLogEpoch
		big.NewInt(0),        // SHA3Epoch
		big.NewInt(0),        // HIP6And8Epoch
		big.NewInt(0),        // StakingPrecompileEpoch
		big.NewInt(0),        // ChainIdFixEpoch
		big.NewInt(0),        // SlotsLimitedEpoch
		big.NewInt(1),        // CrossShardXferPrecompileEpoch
		big.NewInt(0),        // AllowlistEpoch
		big.NewInt(1),        // LeaderRotationExternalNonBeaconLeaders
		big.NewInt(1),        // LeaderRotationExternalBeaconLeaders
		big.NewInt(0),        // FeeCollectEpoch
		big.NewInt(0),        // ValidatorCodeFixEpoch
		big.NewInt(0),        // HIP30Epoch
		big.NewInt(0),        // BlockGas30M
		big.NewInt(0),        // MaxRateEpoch
		big.NewInt(0),
		big.NewInt(0),
	}

	// TestRules ...
	TestRules = TestChainConfig.Rules(new(big.Int))
)

func init() {
	MainnetChainConfig.mustValid()
	TestnetChainConfig.mustValid()
	PangaeaChainConfig.mustValid()
	PartnerChainConfig.mustValid()
	StressnetChainConfig.mustValid()
	LocalnetChainConfig.mustValid()
}

// TrustedCheckpoint represents a set of post-processed trie roots (CHT and
// BloomTrie) associated with the appropriate section index and head hash. It is
// used to start light syncing from this checkpoint and avoid downloading the
// entire header chain while still being able to securely access old headers/logs.
type TrustedCheckpoint struct {
	Name         string      `json:"-"`
	SectionIndex uint64      `json:"sectionIndex"`
	SectionHead  common.Hash `json:"sectionHead"`
	CHTRoot      common.Hash `json:"chtRoot"`
	BloomRoot    common.Hash `json:"bloomRoot"`
}

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	// ChainId identifies the current chain and is used for replay protection
	ChainID *big.Int `json:"chain-id"`

	// EthCompatibleChainID identifies the chain id used for ethereum compatible transactions
	EthCompatibleChainID *big.Int `json:"eth-compatible-chain-id"`

	// EthCompatibleShard0ChainID identifies the shard 0 chain id used for ethereum compatible transactions
	EthCompatibleShard0ChainID *big.Int `json:"eth-compatible-shard-0-chain-id"`

	// EthCompatibleEpoch is the epoch where ethereum-compatible transaction starts being
	// processed.
	EthCompatibleEpoch *big.Int `json:"eth-compatible-epoch,omitempty"`

	// CrossTxEpoch is the epoch where cross-shard transaction starts being
	// processed.
	CrossTxEpoch *big.Int `json:"cross-tx-epoch,omitempty"`

	// CrossLinkEpoch is the epoch where beaconchain starts containing
	// cross-shard links.
	CrossLinkEpoch *big.Int `json:"cross-link-epoch,omitempty"`

	// AggregatedRewardEpoch is the epoch when block rewards are distributed every 64 blocks
	AggregatedRewardEpoch *big.Int `json:"aggregated-reward-epoch,omitempty"`

	// StakingEpoch is the epoch when shard assign takes staking into account
	StakingEpoch *big.Int `json:"staking-epoch,omitempty"`

	// PreStakingEpoch is the epoch we allow staking transactions
	PreStakingEpoch *big.Int `json:"prestaking-epoch,omitempty"`

	// QuickUnlockEpoch is the epoch when undelegation will be unlocked at the current epoch
	QuickUnlockEpoch *big.Int `json:"quick-unlock-epoch,omitempty"`

	// FiveSecondsEpoch is the epoch when block time is reduced to 5 seconds
	// and block rewards adjusted to 17.5 ONE/block
	FiveSecondsEpoch *big.Int `json:"five-seconds-epoch,omitempty"`

	// TwoSecondsEpoch is the epoch when block time is reduced to 2 seconds
	// and block rewards adjusted to 7 ONE/block
	TwoSecondsEpoch *big.Int `json:"two-seconds-epoch,omitempty"`

	// SixtyPercentEpoch is the epoch when internal voting power reduced from 68% to 60%
	SixtyPercentEpoch *big.Int `json:"sixty-percent-epoch,omitempty"`

	// RedelegationEpoch is the epoch when redelegation is supported and undelegation locking time
	// is restored to 7 epoch
	RedelegationEpoch *big.Int `json:"redelegation-epoch,omitempty"`

	// NoEarlyUnlockEpoch is the epoch when the early unlock of undelegated token from validators who were elected for
	// more than 7 epochs is disabled
	NoEarlyUnlockEpoch *big.Int `json:"no-early-unlock-epoch,omitempty"`

	// VRFEpoch is the epoch when VRF randomness is enabled
	VRFEpoch *big.Int `json:"vrf-epoch,omitempty"`

	// PrevVRFEpoch is the epoch when previous VRF randomness can be fetched
	PrevVRFEpoch *big.Int `json:"prev-vrf-epoch,omitempty"`

	// MinDelegation100Epoch is the epoch when min delegation is reduced from 1000 ONE to 100 ONE
	MinDelegation100Epoch *big.Int `json:"min-delegation-100-epoch,omitempty"`

	// MinCommissionRateEpoch is the epoch when policy for minimum comission rate of 5% is started
	MinCommissionRateEpoch *big.Int `json:"min-commission-rate-epoch,omitempty"`

	// MinCommissionPromoPeriod is the number of epochs when newly elected validators can have 0% commission
	MinCommissionPromoPeriod *big.Int `json:"commission-promo-period,omitempty"`

	// EPoSBound35Epoch is the epoch when the EPoS bound parameter c is changed from 15% to 35%
	EPoSBound35Epoch *big.Int `json:"epos-bound-35-epoch,omitempty"`

	// EIP155 hard fork epoch (include EIP158 too)
	EIP155Epoch *big.Int `json:"eip155-epoch,omitempty"`

	// S3 epoch is the first epoch containing S3 mainnet and all ethereum update up to Constantinople
	S3Epoch *big.Int `json:"s3-epoch,omitempty"`

	// DataCopyFix epoch is the first epoch containing fix for evm datacopy bug.
	DataCopyFixEpoch *big.Int `json:"data-copy-fix-epoch,omitempty"`

	// Istanbul epoch
	IstanbulEpoch *big.Int `json:"istanbul-epoch,omitempty"`

	// ReceiptLogEpoch is the first epoch support receiptlog
	ReceiptLogEpoch *big.Int `json:"receipt-log-epoch,omitempty"`

	// IsSHA3Epoch is the first epoch in supporting SHA3 FIPS-202 standard
	SHA3Epoch *big.Int `json:"sha3-epoch,omitempty"`

	// IsHIP6And8Epoch is the first epoch to support HIP-6 and HIP-8
	HIP6And8Epoch *big.Int `json:"hip6_8-epoch,omitempty"`

	// StakingPrecompileEpoch is the first epoch to support the staking precompiles
	StakingPrecompileEpoch *big.Int `json:"staking-precompile-epoch,omitempty"`

	// ChainIdFixEpoch is the first epoch to return ethereum compatible chain id by ChainID() op code
	ChainIdFixEpoch *big.Int `json:"chain-id-fix-epoch,omitempty"`

	// SlotsLimitedEpoch is the first epoch to enable HIP-16.
	SlotsLimitedEpoch *big.Int `json:"slots-limit-epoch,omitempty"`

	// CrossShardXferPrecompileEpoch is the first epoch to feature cross shard transfer precompile
	CrossShardXferPrecompileEpoch *big.Int `json:"cross-shard-xfer-precompile-epoch,omitempty"`

	// AllowlistEpoch is the first epoch to support allowlist of HIP18
	AllowlistEpoch *big.Int

	LeaderRotationInternalValidatorsEpoch *big.Int `json:"leader-rotation-internal-validators,omitempty"`

	LeaderRotationExternalValidatorsEpoch *big.Int `json:"leader-rotation-external-validators,omitempty"`

	// FeeCollectEpoch is the first epoch that enables txn fees to be collected into the community-managed account.
	// It should >= StakingEpoch.
	// Before StakingEpoch, txn fees are paid to miner/leader.
	// Then before FeeCollectEpoch, txn fees are burned.
	// After FeeCollectEpoch, txn fees paid to FeeCollector account.
	FeeCollectEpoch *big.Int

	// ValidatorCodeFixEpoch is the first epoch that fixes the issue of validator code
	// being available in Solidity. This is a temporary fix until we have a better
	// solution.
	// Contracts can check the (presence of) validator code by calling the following:
	// extcodesize, extcodecopy and extcodehash.
	ValidatorCodeFixEpoch *big.Int `json:"validator-code-fix-epoch,omitempty"`

	// The epoch at which HIP30 goes into effect.
	// 1. Number of shards decrease from 4 to 2 (mainnet and localnet)
	// 2. Split emission into 75% for staking rewards, and 25% for recovery (all nets)
	// 3. Change from 250 to 200 nodes for remaining shards (mainnet and localnet)
	// 4. Change the minimum validator commission from 5 to 7% (all nets)
	HIP30Epoch *big.Int `json:"hip30-epoch,omitempty"`

	DevnetExternalEpoch *big.Int `json:"devnet-external-epoch,omitempty"`

	BlockGas30MEpoch *big.Int `json:"block-gas-30m-epoch,omitempty"`

	// MaxRateEpoch will make sure the validator max-rate is at least equal to the minRate + the validator max-rate-increase
	MaxRateEpoch *big.Int `json:"max-rate-epoch,omitempty"`

	// TopMaxRateEpoch will make sure the validator max-rate is less to 100% for the cases where the minRate + the validator max-rate-increase > 100%
	TopMaxRateEpoch *big.Int `json:"top-max-rate-epoch,omitempty"`
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EthCompatibleChainID: %v EIP155: %v CrossTx: %v Staking: %v CrossLink: %v ReceiptLog: %v SHA3Epoch: %v StakingPrecompileEpoch: %v ChainIdFixEpoch: %v CrossShardXferPrecompileEpoch: %v}",
		c.ChainID,
		c.EthCompatibleChainID,
		c.EIP155Epoch,
		c.CrossTxEpoch,
		c.StakingEpoch,
		c.CrossLinkEpoch,
		c.ReceiptLogEpoch,
		c.SHA3Epoch,
		c.StakingPrecompileEpoch,
		c.ChainIdFixEpoch,
		c.CrossShardXferPrecompileEpoch,
	)
}

// check if ChainConfig is valid, it will panic if config is invalid. it should only be called in init()
func (c *ChainConfig) mustValid() {
	require := func(cond bool, err string) {
		if !cond {
			panic(err)
		}
	}
	// to ensure at least RewardFrequency blocks have passed
	require(c.AggregatedRewardEpoch.Cmp(common.Big0) > 0,
		"must satisfy: AggregatedRewardEpoch > 0",
	)
	// before staking epoch, fees were sent to coinbase
	require(c.FeeCollectEpoch.Cmp(c.StakingEpoch) >= 0,
		"must satisfy: FeeCollectEpoch >= StakingEpoch")
	// obvious
	require(c.PreStakingEpoch.Cmp(c.StakingEpoch) < 0,
		"must satisfy: PreStakingEpoch < StakingEpoch")
	// delegations can be made starting at PreStakingEpoch
	require(c.StakingPrecompileEpoch.Cmp(c.PreStakingEpoch) >= 0,
		"must satisfy: StakingPrecompileEpoch >= PreStakingEpoch")
	// main functionality must come before the precompile
	// see AcceptsCrossTx for why > and not >=
	require(c.CrossShardXferPrecompileEpoch.Cmp(c.CrossTxEpoch) > 0,
		"must satisfy: CrossShardXferPrecompileEpoch > CrossTxEpoch")
	// the fix is applied only on the Solidity level, so you need eth compat
	require(c.ValidatorCodeFixEpoch.Cmp(c.EthCompatibleEpoch) >= 0,
		"must satisfy: ValidatorCodeFixEpoch >= EthCompatibleEpoch")
	// we accept validator creation transactions starting at PreStakingEpoch
	require(c.ValidatorCodeFixEpoch.Cmp(c.PreStakingEpoch) >= 0,
		"must satisfy: ValidatorCodeFixEpoch >= PreStakingEpoch")
	// staking epoch must pass for validator count reduction
	require(c.HIP30Epoch.Cmp(c.StakingEpoch) > 0,
		"must satisfy: HIP30Epoch > StakingEpoch")
	// min commission increase 2.0 must happen on or after 1.0
	require(c.HIP30Epoch.Cmp(c.MinCommissionRateEpoch) >= 0,
		"must satisfy: HIP30Epoch > MinCommissionRateEpoch")
	// the HIP30 split distribution of rewards is only implemented
	// for the post aggregated epoch
	require(c.HIP30Epoch.Cmp(c.AggregatedRewardEpoch) >= 0,
		"must satisfy: HIP30Epoch >= MinCommissionRateEpoch")
	// the migration of shard 2 and 3 balances assumes S3
	require(c.HIP30Epoch.Cmp(c.S3Epoch) >= 0,
		"must satisfy: HIP30Epoch >= S3Epoch")
	// capabilities required to transfer balance across shards
	require(c.HIP30Epoch.Cmp(c.CrossTxEpoch) > 0,
		"must satisfy: HIP30Epoch > CrossTxEpoch")
	// max rate (7%) fix is applied on or after hip30
	require(c.MaxRateEpoch.Cmp(c.HIP30Epoch) >= 0,
		"must satisfy: MaxRateEpoch >= HIP30Epoch")
}

// IsEIP155 returns whether epoch is either equal to the EIP155 fork epoch or greater.
func (c *ChainConfig) IsEIP155(epoch *big.Int) bool {
	return isForked(c.EIP155Epoch, epoch)
}

// AcceptsCrossTx returns whether cross-shard transaction is accepted in the
// given epoch.
//
// Note that this is different from comparing epoch against CrossTxEpoch.
// Cross-shard transaction is accepted from CrossTxEpoch+1 and on, in order to
// allow for all shards to roll into CrossTxEpoch and become able to handle
// ingress receipts.  In other words, cross-shard transaction fields are
// introduced and ingress receipts are processed at CrossTxEpoch, but the shard
// does not accept cross-shard transactions from clients until CrossTxEpoch+1.
func (c *ChainConfig) AcceptsCrossTx(epoch *big.Int) bool {
	crossTxEpoch := new(big.Int).Add(c.CrossTxEpoch, common.Big1)
	return isForked(crossTxEpoch, epoch)
}

// HasCrossTxFields returns whether blocks in the given epoch includes
// cross-shard transaction fields.
func (c *ChainConfig) HasCrossTxFields(epoch *big.Int) bool {
	return isForked(c.CrossTxEpoch, epoch)
}

// IsEthCompatible determines whether it is ethereum compatible epoch
func (c *ChainConfig) IsEthCompatible(epoch *big.Int) bool {
	return isForked(c.EthCompatibleEpoch, epoch)
}

// IsAggregatedRewardEpoch determines whether it is the epoch when rewards are distributed every 64 blocks
func (c *ChainConfig) IsAggregatedRewardEpoch(epoch *big.Int) bool {
	return isForked(c.AggregatedRewardEpoch, epoch)
}

// IsStaking determines whether it is staking epoch
func (c *ChainConfig) IsStaking(epoch *big.Int) bool {
	return isForked(c.StakingEpoch, epoch)
}

// IsSlotsLimited determines whether HIP-16 is enabled
func (c *ChainConfig) IsSlotsLimited(epoch *big.Int) bool {
	return isForked(c.SlotsLimitedEpoch, epoch)
}

// IsFiveSeconds determines whether it is the epoch to change to 5 seconds block time
func (c *ChainConfig) IsFiveSeconds(epoch *big.Int) bool {
	return isForked(c.FiveSecondsEpoch, epoch)
}

// IsTwoSeconds determines whether it is the epoch to change to 3 seconds block time
func (c *ChainConfig) IsTwoSeconds(epoch *big.Int) bool {
	return isForked(c.TwoSecondsEpoch, epoch)
}

// IsSixtyPercent determines whether it is the epoch to reduce internal voting power to 60%
func (c *ChainConfig) IsSixtyPercent(epoch *big.Int) bool {
	return isForked(c.SixtyPercentEpoch, epoch)
}

// IsRedelegation determines whether it is the epoch to support redelegation
func (c *ChainConfig) IsRedelegation(epoch *big.Int) bool {
	return isForked(c.RedelegationEpoch, epoch)
}

// IsNoEarlyUnlock determines whether it is the epoch to stop early unlock
func (c *ChainConfig) IsNoEarlyUnlock(epoch *big.Int) bool {
	return isForked(c.NoEarlyUnlockEpoch, epoch)
}

// IsVRF determines whether it is the epoch to enable vrf
func (c *ChainConfig) IsVRF(epoch *big.Int) bool {
	return isForked(c.VRFEpoch, epoch)
}

// IsPrevVRF determines whether it is the epoch to enable previous vrf
func (c *ChainConfig) IsPrevVRF(epoch *big.Int) bool {
	return isForked(c.PrevVRFEpoch, epoch)
}

// IsMinDelegation100 determines whether it is the epoch to reduce min delegation to 100
func (c *ChainConfig) IsMinDelegation100(epoch *big.Int) bool {
	return isForked(c.MinDelegation100Epoch, epoch)
}

// IsMinCommissionRate determines whether it is the epoch to start the policy of 5% min commission
func (c *ChainConfig) IsMinCommissionRate(epoch *big.Int) bool {
	return isForked(c.MinCommissionRateEpoch, epoch)
}

// IsEPoSBound35 determines whether it is the epoch to extend the EPoS bound to 35%
func (c *ChainConfig) IsEPoSBound35(epoch *big.Int) bool {
	return isForked(c.EPoSBound35Epoch, epoch)
}

// IsPreStaking determines whether staking transactions are allowed
func (c *ChainConfig) IsPreStaking(epoch *big.Int) bool {
	return isForked(c.PreStakingEpoch, epoch)
}

// IsQuickUnlock determines whether it's the epoch when the undelegation should be unlocked at end of current epoch
func (c *ChainConfig) IsQuickUnlock(epoch *big.Int) bool {
	return isForked(c.QuickUnlockEpoch, epoch)
}

// IsCrossLink returns whether epoch is either equal to the CrossLink fork epoch or greater.
func (c *ChainConfig) IsCrossLink(epoch *big.Int) bool {
	return isForked(c.CrossLinkEpoch, epoch)
}

// IsS3 returns whether epoch is either equal to the S3 fork epoch or greater.
func (c *ChainConfig) IsS3(epoch *big.Int) bool {
	return isForked(c.S3Epoch, epoch)
}

// IsDataCopyFixEpoch returns whether epoch has the fix for DataCopy evm bug.
func (c *ChainConfig) IsDataCopyFixEpoch(epoch *big.Int) bool {
	return isForked(c.DataCopyFixEpoch, epoch)
}

// IsIstanbul returns whether epoch is either equal to the Istanbul fork epoch or greater.
func (c *ChainConfig) IsIstanbul(epoch *big.Int) bool {
	return isForked(c.IstanbulEpoch, epoch)
}

// IsReceiptLog returns whether epoch is either equal to the ReceiptLog fork epoch or greater.
func (c *ChainConfig) IsReceiptLog(epoch *big.Int) bool {
	return isForked(c.ReceiptLogEpoch, epoch)
}

// IsSHA3 returns whether epoch is either equal to the IsSHA3 fork epoch or greater.
func (c *ChainConfig) IsSHA3(epoch *big.Int) bool {
	return isForked(c.SHA3Epoch, epoch)
}

// IsHIP6And8Epoch determines whether it is the epoch to support
// HIP-6: reduce the internal voting power from 60% to 49%
// HIP-8: increase external nodes from 800 to 900
func (c *ChainConfig) IsHIP6And8Epoch(epoch *big.Int) bool {
	return isForked(c.HIP6And8Epoch, epoch)
}

// IsStakingPrecompileEpoch determines whether staking
// precompiles are available in the EVM
func (c *ChainConfig) IsStakingPrecompile(epoch *big.Int) bool {
	return isForked(c.StakingPrecompileEpoch, epoch)
}

// IsCrossShardXferPrecompile determines whether the
// Cross Shard Transfer Precompile is available in the EVM
func (c *ChainConfig) IsCrossShardXferPrecompile(epoch *big.Int) bool {
	return isForked(c.CrossShardXferPrecompileEpoch, epoch)
}

// IsChainIdFix returns whether epoch is either equal to the ChainId Fix fork epoch or greater.
func (c *ChainConfig) IsChainIdFix(epoch *big.Int) bool {
	return isForked(c.ChainIdFixEpoch, epoch)
}

// IsAllowlistEpoch determines whether IsAllowlist of HIP18 is enabled
func (c *ChainConfig) IsAllowlistEpoch(epoch *big.Int) bool {
	return isForked(c.AllowlistEpoch, epoch)
}

func (c *ChainConfig) IsLeaderRotationInternalValidators(epoch *big.Int) bool {
	return isForked(c.LeaderRotationInternalValidatorsEpoch, epoch)
}

func (c *ChainConfig) IsBlockGas30M(epoch *big.Int) bool {
	return isForked(c.BlockGas30MEpoch, epoch)
}

func (c *ChainConfig) IsLeaderRotationExternalValidatorsAllowed(epoch *big.Int) bool {
	return isForked(c.LeaderRotationExternalValidatorsEpoch, epoch)
}

// IsFeeCollectEpoch determines whether Txn Fees will be collected into the community-managed account.
func (c *ChainConfig) IsFeeCollectEpoch(epoch *big.Int) bool {
	return isForked(c.FeeCollectEpoch, epoch)
}

func (c *ChainConfig) IsValidatorCodeFix(epoch *big.Int) bool {
	return isForked(c.ValidatorCodeFixEpoch, epoch)
}

func (c *ChainConfig) IsHIP30(epoch *big.Int) bool {
	return isForked(c.HIP30Epoch, epoch)
}

func (c *ChainConfig) IsDevnetExternalEpoch(epoch *big.Int) bool {
	return isForked(c.DevnetExternalEpoch, epoch)
}

func (c *ChainConfig) IsMaxRate(epoch *big.Int) bool {
	return isForked(c.MaxRateEpoch, epoch)
}

func (c *ChainConfig) IsTopMaxRate(epoch *big.Int) bool {
	return isForked(c.TopMaxRateEpoch, epoch)
}

// During this epoch, shards 2 and 3 will start sending
// their balances over to shard 0 or 1.
func (c *ChainConfig) IsOneEpochBeforeHIP30(epoch *big.Int) bool {
	return new(big.Int).Sub(c.HIP30Epoch, epoch).Cmp(common.Big1) == 0
}

// UpdateEthChainIDByShard update the ethChainID based on shard ID.
func UpdateEthChainIDByShard(shardID uint32) {
	once.Do(func() {
		MainnetChainConfig.EthCompatibleChainID = big.NewInt(0).Add(MainnetChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		TestnetChainConfig.EthCompatibleChainID = big.NewInt(0).Add(TestnetChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		PangaeaChainConfig.EthCompatibleChainID = big.NewInt(0).Add(PangaeaChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		PartnerChainConfig.EthCompatibleChainID = big.NewInt(0).Add(PartnerChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		StressnetChainConfig.EthCompatibleChainID = big.NewInt(0).Add(StressnetChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		LocalnetChainConfig.EthCompatibleChainID = big.NewInt(0).Add(LocalnetChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
		AllProtocolChanges.EthCompatibleChainID = big.NewInt(0).Add(AllProtocolChanges.EthCompatibleChainID, big.NewInt(int64(shardID)))
		TestChainConfig.EthCompatibleChainID = big.NewInt(0).Add(TestChainConfig.EthCompatibleChainID, big.NewInt(int64(shardID)))
	})
}

// IsEthCompatible returns whether the chainID is for ethereum compatible txn or not
func IsEthCompatible(chainID *big.Int) bool {
	return chainID.Cmp(EthMainnetShard0ChainID) >= 0
}

// GasTable returns the gas table corresponding to the current phase (homestead or homestead reprice).
//
// The returned GasTable's fields shouldn't, under any circumstances, be changed.
func (c *ChainConfig) GasTable(epoch *big.Int) GasTable {
	if epoch == nil {
		return GasTableR3
	}
	switch {
	case c.IsS3(epoch):
		return GasTableS3
	default:
		return GasTableR3
	}
}

// isForked returns whether a fork scheduled at epoch s is active at the given head epoch.
func isForked(s, epoch *big.Int) bool {
	if s == nil || epoch == nil {
		return false
	}
	return s.Cmp(epoch) <= 0
}

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainID    *big.Int
	EthChainID *big.Int
	// gas
	IsS3,
	// precompiles
	IsIstanbul, IsVRF, IsPrevVRF, IsSHA3,
	IsStakingPrecompile, IsCrossShardXferPrecompile,
	// eip-155 chain id fix
	IsChainIdFix bool
	IsValidatorCodeFix bool
}

// Rules ensures c's ChainID is not nil.
// The Rules object is only used by The EVM
func (c *ChainConfig) Rules(epoch *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	ethChainID := c.EthCompatibleChainID
	if ethChainID == nil {
		ethChainID = new(big.Int)
	}
	return Rules{
		ChainID:                    new(big.Int).Set(chainID),
		EthChainID:                 new(big.Int).Set(ethChainID),
		IsS3:                       c.IsS3(epoch),
		IsIstanbul:                 c.IsIstanbul(epoch),
		IsVRF:                      c.IsVRF(epoch),
		IsPrevVRF:                  c.IsPrevVRF(epoch),
		IsSHA3:                     c.IsSHA3(epoch),
		IsStakingPrecompile:        c.IsStakingPrecompile(epoch),
		IsCrossShardXferPrecompile: c.IsCrossShardXferPrecompile(epoch),
		IsChainIdFix:               c.IsChainIdFix(epoch),
		IsValidatorCodeFix:         c.IsValidatorCodeFix(epoch),
	}
}
