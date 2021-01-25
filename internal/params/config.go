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
	EthStressnetShard0ChainID          = big.NewInt(1661000000)
	EthTestShard0ChainID               = big.NewInt(1661100000) // not a real network
	EthAllProtocolChangesShard0ChainID = big.NewInt(1661200000) // not a real network
)

// EpochTBD is a large, “not anytime soon” epoch.  It used as a placeholder
// until the exact epoch is decided.
var EpochTBD = big.NewInt(10000000)
var once sync.Once

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:                    MainnetChainID,
		EthCompatibleChainID:       EthMainnetShard0ChainID,
		EthCompatibleShard0ChainID: EthMainnetShard0ChainID,
		EthCompatibleEpoch:         EpochTBD,
		CrossTxEpoch:               big.NewInt(28),
		CrossLinkEpoch:             big.NewInt(186),
		StakingEpoch:               big.NewInt(186),
		PreStakingEpoch:            big.NewInt(185),
		QuickUnlockEpoch:           big.NewInt(191),
		FiveSecondsEpoch:           big.NewInt(230),
		TwoSecondsEpoch:            big.NewInt(366), // Around Tuesday Dec 8th 2020, 8AM PST
		SixtyPercentEpoch:          EpochTBD,
		RedelegationEpoch:          big.NewInt(290),
		EIP155Epoch:                big.NewInt(28),
		S3Epoch:                    big.NewInt(28),
		IstanbulEpoch:              big.NewInt(314),
		ReceiptLogEpoch:            big.NewInt(101),
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:                    TestnetChainID,
		EthCompatibleChainID:       EthTestnetShard0ChainID,
		EthCompatibleShard0ChainID: EthTestnetShard0ChainID,
		EthCompatibleEpoch:         big.NewInt(73290),
		CrossTxEpoch:               big.NewInt(0),
		CrossLinkEpoch:             big.NewInt(2),
		StakingEpoch:               big.NewInt(2),
		PreStakingEpoch:            big.NewInt(1),
		QuickUnlockEpoch:           big.NewInt(0),
		FiveSecondsEpoch:           big.NewInt(16500),
		TwoSecondsEpoch:            big.NewInt(73000),
		SixtyPercentEpoch:          big.NewInt(73282),
		RedelegationEpoch:          big.NewInt(36500),
		EIP155Epoch:                big.NewInt(0),
		S3Epoch:                    big.NewInt(0),
		IstanbulEpoch:              big.NewInt(43800),
		ReceiptLogEpoch:            big.NewInt(0),
	}

	// PangaeaChainConfig contains the chain parameters for the Pangaea network.
	// All features except for CrossLink are enabled at launch.
	PangaeaChainConfig = &ChainConfig{
		ChainID:                    PangaeaChainID,
		EthCompatibleChainID:       EthPangaeaShard0ChainID,
		EthCompatibleShard0ChainID: EthPangaeaShard0ChainID,
		EthCompatibleEpoch:         big.NewInt(0),
		CrossTxEpoch:               big.NewInt(0),
		CrossLinkEpoch:             big.NewInt(2),
		StakingEpoch:               big.NewInt(2),
		PreStakingEpoch:            big.NewInt(1),
		QuickUnlockEpoch:           big.NewInt(0),
		FiveSecondsEpoch:           big.NewInt(0),
		TwoSecondsEpoch:            big.NewInt(0),
		SixtyPercentEpoch:          big.NewInt(0),
		RedelegationEpoch:          big.NewInt(0),
		EIP155Epoch:                big.NewInt(0),
		S3Epoch:                    big.NewInt(0),
		IstanbulEpoch:              big.NewInt(0),
		ReceiptLogEpoch:            big.NewInt(0),
	}

	// PartnerChainConfig contains the chain parameters for the Partner network.
	// All features except for CrossLink are enabled at launch.
	PartnerChainConfig = &ChainConfig{
		ChainID:                    PartnerChainID,
		EthCompatibleChainID:       EthPartnerShard0ChainID,
		EthCompatibleShard0ChainID: EthPartnerShard0ChainID,
		EthCompatibleEpoch:         big.NewInt(0),
		CrossTxEpoch:               big.NewInt(0),
		CrossLinkEpoch:             big.NewInt(2),
		StakingEpoch:               big.NewInt(2),
		PreStakingEpoch:            big.NewInt(1),
		QuickUnlockEpoch:           big.NewInt(0),
		FiveSecondsEpoch:           big.NewInt(0),
		TwoSecondsEpoch:            big.NewInt(0),
		SixtyPercentEpoch:          big.NewInt(0),
		RedelegationEpoch:          big.NewInt(0),
		EIP155Epoch:                big.NewInt(0),
		S3Epoch:                    big.NewInt(0),
		IstanbulEpoch:              big.NewInt(0),
		ReceiptLogEpoch:            big.NewInt(0),
	}

	// StressnetChainConfig contains the chain parameters for the Stress test network.
	// All features except for CrossLink are enabled at launch.
	StressnetChainConfig = &ChainConfig{
		ChainID:                    StressnetChainID,
		EthCompatibleChainID:       EthStressnetShard0ChainID,
		EthCompatibleShard0ChainID: EthStressnetShard0ChainID,
		EthCompatibleEpoch:         big.NewInt(0),
		CrossTxEpoch:               big.NewInt(0),
		CrossLinkEpoch:             big.NewInt(2),
		StakingEpoch:               big.NewInt(2),
		PreStakingEpoch:            big.NewInt(1),
		QuickUnlockEpoch:           big.NewInt(0),
		FiveSecondsEpoch:           big.NewInt(0),
		TwoSecondsEpoch:            big.NewInt(0),
		SixtyPercentEpoch:          EpochTBD, // Never enable it for STN as STN has no external validator setup
		RedelegationEpoch:          big.NewInt(0),
		EIP155Epoch:                big.NewInt(0),
		S3Epoch:                    big.NewInt(0),
		IstanbulEpoch:              big.NewInt(0),
		ReceiptLogEpoch:            big.NewInt(0),
	}

	// LocalnetChainConfig contains the chain parameters to run for local development.
	LocalnetChainConfig = &ChainConfig{
		ChainID:                    TestnetChainID,
		EthCompatibleChainID:       EthTestnetShard0ChainID,
		EthCompatibleShard0ChainID: EthTestnetShard0ChainID,
		EthCompatibleEpoch:         big.NewInt(0),
		CrossTxEpoch:               big.NewInt(0),
		CrossLinkEpoch:             big.NewInt(2),
		StakingEpoch:               big.NewInt(2),
		PreStakingEpoch:            big.NewInt(0),
		QuickUnlockEpoch:           big.NewInt(0),
		FiveSecondsEpoch:           big.NewInt(0),
		TwoSecondsEpoch:            big.NewInt(3),
		SixtyPercentEpoch:          EpochTBD, // Never enable it for localnet as localnet has no external validator setup
		RedelegationEpoch:          big.NewInt(0),
		EIP155Epoch:                big.NewInt(0),
		S3Epoch:                    big.NewInt(0),
		IstanbulEpoch:              big.NewInt(0),
		ReceiptLogEpoch:            big.NewInt(0),
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
		big.NewInt(0),                      // StakingEpoch
		big.NewInt(0),                      // PreStakingEpoch
		big.NewInt(0),                      // QuickUnlockEpoch
		big.NewInt(0),                      // FiveSecondsEpoch
		big.NewInt(0),                      // TwoSecondsEpoch
		big.NewInt(0),                      // SixtyPercentEpoch
		big.NewInt(0),                      // RedelegationEpoch
		big.NewInt(0),                      // EIP155Epoch
		big.NewInt(0),                      // S3Epoch
		big.NewInt(0),                      // IstanbulEpoch
		big.NewInt(0),                      // ReceiptLogEpoch
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
		big.NewInt(0),        // StakingEpoch
		big.NewInt(0),        // PreStakingEpoch
		big.NewInt(0),        // QuickUnlockEpoch
		big.NewInt(0),        // FiveSecondsEpoch
		big.NewInt(0),        // TwoSecondsEpoch
		big.NewInt(0),        // SixtyPercentEpoch
		big.NewInt(0),        // RedelegationEpoch
		big.NewInt(0),        // EIP155Epoch
		big.NewInt(0),        // S3Epoch
		big.NewInt(0),        // IstanbulEpoch
		big.NewInt(0),        // ReceiptLogEpoch
	}

	// TestRules ...
	TestRules = TestChainConfig.Rules(new(big.Int))
)

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

	// EIP155 hard fork epoch (include EIP158 too)
	EIP155Epoch *big.Int `json:"eip155-epoch,omitempty"`

	// S3 epoch is the first epoch containing S3 mainnet and all ethereum update up to Constantinople
	S3Epoch *big.Int `json:"s3-epoch,omitempty"`

	// Istanbul epoch
	IstanbulEpoch *big.Int `json:"istanbul-epoch,omitempty"`

	// ReceiptLogEpoch is the first epoch support receiptlog
	ReceiptLogEpoch *big.Int `json:"receipt-log-epoch,omitempty"`
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EthCompatibleChainID: %v EIP155: %v CrossTx: %v Staking: %v CrossLink: %v ReceiptLog: %v}",
		c.ChainID,
		c.EthCompatibleChainID,
		c.EIP155Epoch,
		c.CrossTxEpoch,
		c.StakingEpoch,
		c.CrossLinkEpoch,
		c.ReceiptLogEpoch,
	)
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

// IsStaking determines whether it is staking epoch
func (c *ChainConfig) IsStaking(epoch *big.Int) bool {
	return isForked(c.StakingEpoch, epoch)
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

// IsIstanbul returns whether epoch is either equal to the Istanbul fork epoch or greater.
func (c *ChainConfig) IsIstanbul(epoch *big.Int) bool {
	return isForked(c.IstanbulEpoch, epoch)
}

// IsReceiptLog returns whether epoch is either equal to the ReceiptLog fork epoch or greater.
func (c *ChainConfig) IsReceiptLog(epoch *big.Int) bool {
	return isForked(c.ReceiptLogEpoch, epoch)
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
	ChainID                                               *big.Int
	EthChainID                                            *big.Int
	IsCrossLink, IsEIP155, IsS3, IsReceiptLog, IsIstanbul bool
}

// Rules ensures c's ChainID is not nil.
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
		ChainID:      new(big.Int).Set(chainID),
		EthChainID:   new(big.Int).Set(ethChainID),
		IsCrossLink:  c.IsCrossLink(epoch),
		IsEIP155:     c.IsEIP155(epoch),
		IsS3:         c.IsS3(epoch),
		IsReceiptLog: c.IsReceiptLog(epoch),
		IsIstanbul:   c.IsIstanbul(epoch),
	}
}
