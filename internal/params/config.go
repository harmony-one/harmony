package params

import (
	"fmt"
	"math/big"

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
)

// EpochTBD is a large, “not anytime soon” epoch.  It used as a placeholder
// until the exact epoch is decided.
var EpochTBD = big.NewInt(10000000)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:         MainnetChainID,
		CrossTxEpoch:    big.NewInt(28),
		CrossLinkEpoch:  big.NewInt(186),
		StakingEpoch:    big.NewInt(186),
		PreStakingEpoch: big.NewInt(185),
		EIP155Epoch:     big.NewInt(28),
		S3Epoch:         big.NewInt(28),
		ReceiptLogEpoch: big.NewInt(101),
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:         TestnetChainID,
		CrossTxEpoch:    big.NewInt(0),
		CrossLinkEpoch:  big.NewInt(2),
		StakingEpoch:    big.NewInt(2),
		PreStakingEpoch: big.NewInt(1),
		EIP155Epoch:     big.NewInt(0),
		S3Epoch:         big.NewInt(0),
		ReceiptLogEpoch: big.NewInt(0),
	}

	// PangaeaChainConfig contains the chain parameters for the Pangaea network.
	// All features except for CrossLink are enabled at launch.
	PangaeaChainConfig = &ChainConfig{
		ChainID:         PangaeaChainID,
		CrossTxEpoch:    big.NewInt(0),
		CrossLinkEpoch:  big.NewInt(2),
		StakingEpoch:    big.NewInt(2),
		PreStakingEpoch: big.NewInt(1),
		EIP155Epoch:     big.NewInt(0),
		S3Epoch:         big.NewInt(0),
		ReceiptLogEpoch: big.NewInt(0),
	}

	// PartnerChainConfig contains the chain parameters for the Partner network.
	// All features except for CrossLink are enabled at launch.
	PartnerChainConfig = &ChainConfig{
		ChainID:         PartnerChainID,
		CrossTxEpoch:    big.NewInt(0),
		CrossLinkEpoch:  big.NewInt(2),
		StakingEpoch:    big.NewInt(2),
		PreStakingEpoch: big.NewInt(1),
		EIP155Epoch:     big.NewInt(0),
		S3Epoch:         big.NewInt(0),
		ReceiptLogEpoch: big.NewInt(0),
	}

	// StressnetChainConfig contains the chain parameters for the Stress test network.
	// All features except for CrossLink are enabled at launch.
	StressnetChainConfig = &ChainConfig{
		ChainID:         StressnetChainID,
		CrossTxEpoch:    big.NewInt(0),
		CrossLinkEpoch:  big.NewInt(2),
		StakingEpoch:    big.NewInt(2),
		PreStakingEpoch: big.NewInt(1),
		EIP155Epoch:     big.NewInt(0),
		S3Epoch:         big.NewInt(0),
		ReceiptLogEpoch: big.NewInt(0),
	}

	// LocalnetChainConfig contains the chain parameters to run for local development.
	LocalnetChainConfig = &ChainConfig{
		ChainID:         TestnetChainID,
		CrossTxEpoch:    big.NewInt(0),
		CrossLinkEpoch:  big.NewInt(2),
		StakingEpoch:    big.NewInt(2),
		PreStakingEpoch: big.NewInt(0),
		EIP155Epoch:     big.NewInt(0),
		S3Epoch:         big.NewInt(0),
		ReceiptLogEpoch: big.NewInt(0),
	}

	// AllProtocolChanges ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllProtocolChanges = &ChainConfig{
		AllProtocolChangesChainID, // ChainID
		big.NewInt(0),             // CrossTxEpoch
		big.NewInt(0),             // CrossLinkEpoch
		big.NewInt(0),             // StakingEpoch
		big.NewInt(0),             // PreStakingEpoch
		big.NewInt(0),             // EIP155Epoch
		big.NewInt(0),             // S3Epoch
		big.NewInt(0),             // ReceiptLogEpoch
	}

	// TestChainConfig ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	TestChainConfig = &ChainConfig{
		TestChainID,   // ChainID
		big.NewInt(0), // CrossTxEpoch
		big.NewInt(0), // CrossLinkEpoch
		big.NewInt(0), // StakingEpoch
		big.NewInt(0), // PreStakingEpoch
		big.NewInt(0), // EIP155Epoch
		big.NewInt(0), // S3Epoch
		big.NewInt(0), // ReceiptLogEpoch
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

	// EIP155 hard fork epoch (include EIP158 too)
	EIP155Epoch *big.Int `json:"eip155-epoch,omitempty"`

	// S3 epoch is the first epoch containing S3 mainnet and all ethereum update up to Constantinople
	S3Epoch *big.Int `json:"s3-epoch,omitempty"`

	// ReceiptLogEpoch is the first epoch support receiptlog
	ReceiptLogEpoch *big.Int `json:"receipt-log-epoch,omitempty"`
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EIP155: %v CrossTx: %v Staking: %v CrossLink: %v ReceiptLog: %v}",
		c.ChainID,
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

// IsStaking determines whether it is staking epoch
func (c *ChainConfig) IsStaking(epoch *big.Int) bool {
	return isForked(c.StakingEpoch, epoch)
}

// IsPreStaking determines whether staking transactions are allowed
func (c *ChainConfig) IsPreStaking(epoch *big.Int) bool {
	return isForked(c.PreStakingEpoch, epoch)
}

// IsCrossLink returns whether epoch is either equal to the CrossLink fork epoch or greater.
func (c *ChainConfig) IsCrossLink(epoch *big.Int) bool {
	return isForked(c.CrossLinkEpoch, epoch)
}

// IsS3 returns whether epoch is either equal to the S3 fork epoch or greater.
func (c *ChainConfig) IsS3(epoch *big.Int) bool {
	return isForked(c.S3Epoch, epoch)
}

// IsReceiptLog returns whether epoch is either equal to the ReceiptLog fork epoch or greater.
func (c *ChainConfig) IsReceiptLog(epoch *big.Int) bool {
	return isForked(c.ReceiptLogEpoch, epoch)
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
	ChainID                                   *big.Int
	IsCrossLink, IsEIP155, IsS3, IsReceiptLog bool
}

// Rules ensures c's ChainID is not nil.
func (c *ChainConfig) Rules(epoch *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return Rules{
		ChainID:      new(big.Int).Set(chainID),
		IsCrossLink:  c.IsCrossLink(epoch),
		IsEIP155:     c.IsEIP155(epoch),
		IsS3:         c.IsS3(epoch),
		IsReceiptLog: c.IsReceiptLog(epoch),
	}
}
