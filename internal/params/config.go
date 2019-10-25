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
	TestChainID               = big.NewInt(99)  // not a real network
	AllProtocolChangesChainID = big.NewInt(100) // not a real network
)

// EpochTBD is a large, “not anytime soon” epoch.  It used as a placeholder
// until the exact epoch is decided.
var EpochTBD = big.NewInt(10000000)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:        MainnetChainID,
		CrossTxEpoch:   big.NewInt(28),
		CrossLinkEpoch: EpochTBD,
		StakingEpoch:   EpochTBD,
		EIP155Epoch:    big.NewInt(28),
		S3Epoch:        big.NewInt(28),
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:        TestnetChainID,
		CrossTxEpoch:   big.NewInt(0),
		CrossLinkEpoch: big.NewInt(2),
		StakingEpoch:   big.NewInt(3),
		EIP155Epoch:    big.NewInt(0),
		S3Epoch:        big.NewInt(0),
	}

	// PangaeaChainConfig contains the chain parameters for the Pangaea network.
	// All features except for CrossLink are enabled at launch.
	PangaeaChainConfig = &ChainConfig{
		ChainID:        PangaeaChainID,
		CrossTxEpoch:   big.NewInt(0),
		CrossLinkEpoch: EpochTBD,
		StakingEpoch:   EpochTBD,
		EIP155Epoch:    big.NewInt(0),
		S3Epoch:        big.NewInt(0),
	}

	// AllProtocolChanges ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllProtocolChanges = &ChainConfig{
		AllProtocolChangesChainID, // ChainID
		big.NewInt(0),             // CrossTxEpoch
		big.NewInt(0),             // CrossLinkEpoch
		big.NewInt(0),             // StakingEpoch
		big.NewInt(0),             // EIP155Epoch
		big.NewInt(0),             // S3Epoch
	}

	// TestChainConfig ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	TestChainConfig = &ChainConfig{
		TestChainID,   // ChainID
		big.NewInt(0), // CrossTxEpoch
		big.NewInt(0), // CrossLinkEpoch
		big.NewInt(0), // StakingEpoch
		big.NewInt(0), // EIP155Epoch
		big.NewInt(0), // S3Epoch
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
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection

	// CrossTxEpoch is the epoch where cross-shard transaction starts being
	// processed.
	CrossTxEpoch *big.Int `json:"crossTxEpoch,omitempty"`

	// CrossLinkEpoch is the epoch where beaconchain starts containing
	// cross-shard links.
	CrossLinkEpoch *big.Int `json:"crossLinkEpoch,omitempty"`

	// StakingEpoch is the epoch we allow staking transactions
	StakingEpoch *big.Int `json:"stakingEpoch,omitempty"`

	EIP155Epoch *big.Int `json:"eip155Epoch,omitempty"` // EIP155 hard fork epoch (include EIP158 too)
	S3Epoch     *big.Int `json:"s3Epoch,omitempty"`     // S3 epoch is the first epoch containing S3 mainnet and all ethereum update up to Constantinople
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EIP155: %v CrossTx: %v Staking: %v CrossLink: %v}",
		c.ChainID,
		c.EIP155Epoch,
		c.CrossTxEpoch,
		c.StakingEpoch,
		c.CrossLinkEpoch,
	)
}

// IsEIP155 returns whether epoch is either equal to the EIP155 fork epoch or greater.
func (c *ChainConfig) IsEIP155(epoch *big.Int) bool {
	return isForked(c.EIP155Epoch, epoch)
}

// IsCrossTx returns whether cross-shard transaction is enabled in the given
// epoch.
//
// Note that this is different from comparing epoch against CrossTxEpoch.
// Cross-shard transaction is enabled from CrossTxEpoch+1 and on, in order to
// allow for all shards to roll into CrossTxEpoch and become able to handle
// ingress receipts.  In other words, cross-shard transaction fields are
// introduced at CrossTxEpoch, but these fields are not used until
// CrossTxEpoch+1, when cross-shard transactions are actually accepted by the
// network.
func (c *ChainConfig) IsCrossTx(epoch *big.Int) bool {
	crossTxEpoch := new(big.Int).Add(c.CrossTxEpoch, common.Big1)
	return isForked(crossTxEpoch, epoch)
}

// IsStaking determines whether it is staking epoch
func (c *ChainConfig) IsStaking(epoch *big.Int) bool {
	stkEpoch := new(big.Int).Add(c.StakingEpoch, common.Big1)
	return isForked(stkEpoch, epoch)
}

// IsCrossLink returns whether epoch is either equal to the CrossLink fork epoch or greater.
func (c *ChainConfig) IsCrossLink(epoch *big.Int) bool {
	return isForked(c.CrossLinkEpoch, epoch)
}

// IsS3 returns whether epoch is either equal to the S3 fork epoch or greater.
func (c *ChainConfig) IsS3(epoch *big.Int) bool {
	return isForked(c.S3Epoch, epoch)
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

// CheckCompatible checks whether scheduled fork transitions have been imported
// with a mismatching chain configuration.
func (c *ChainConfig) CheckCompatible(newcfg *ChainConfig, height uint64) *ConfigCompatError {
	bhead := new(big.Int).SetUint64(height)

	// Iterate checkCompatible to find the lowest conflict.
	var lasterr *ConfigCompatError
	for {
		err := c.checkCompatible(newcfg, bhead)
		if err == nil || (lasterr != nil && err.RewindTo == lasterr.RewindTo) {
			break
		}
		lasterr = err
		bhead.SetUint64(err.RewindTo)
	}
	return lasterr
}

func (c *ChainConfig) checkCompatible(newcfg *ChainConfig, head *big.Int) *ConfigCompatError {
	// TODO: check compatibility and reversion based on epochs.
	//if isForkIncompatible(c.EIP155Epoch, newcfg.EIP155Epoch, head) {
	//	return newCompatError("EIP155 fork block", c.EIP155Epoch, newcfg.EIP155Epoch)
	//}
	//if isForkIncompatible(c.CrossLinkEpoch, newcfg.CrossLinkEpoch, head) {
	//	return newCompatError("CrossLink fork block", c.CrossLinkEpoch, newcfg.CrossLinkEpoch)
	//}
	//if isForkIncompatible(c.S3Epoch, newcfg.S3Epoch, head) {
	//	return newCompatError("S3 fork block", c.S3Epoch, newcfg.S3Epoch)
	//}
	return nil
}

// isForkIncompatible returns true if a fork scheduled at s1 cannot be rescheduled to
// epoch s2 because head is already past the fork.
func isForkIncompatible(s1, s2, epoch *big.Int) bool {
	return (isForked(s1, epoch) || isForked(s2, epoch)) && !configNumEqual(s1, s2)
}

// isForked returns whether a fork scheduled at epoch s is active at the given head epoch.
func isForked(s, epoch *big.Int) bool {
	if s == nil || epoch == nil {
		return false
	}
	return s.Cmp(epoch) <= 0
}

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// ConfigCompatError is raised if the locally-stored blockchain is initialised with a
// ChainConfig that would alter the past.
type ConfigCompatError struct {
	What string
	// block numbers of the stored and new configurations
	StoredConfig, NewConfig *big.Int
	// the block number to which the local chain must be rewound to correct the error
	RewindTo uint64
}

func newCompatError(what string, storedblock, newblock *big.Int) *ConfigCompatError {
	var rew *big.Int
	switch {
	case storedblock == nil:
		rew = newblock
	case newblock == nil || storedblock.Cmp(newblock) < 0:
		rew = storedblock
	default:
		rew = newblock
	}
	err := &ConfigCompatError{what, storedblock, newblock, 0}
	if rew != nil && rew.Sign() > 0 {
		err.RewindTo = rew.Uint64() - 1
	}
	return err
}

func (err *ConfigCompatError) Error() string {
	return fmt.Sprintf("mismatching %s in database (have %d, want %d, rewindto %d)", err.What, err.StoredConfig, err.NewConfig, err.RewindTo)
}

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainID                     *big.Int
	IsCrossLink, IsEIP155, IsS3 bool
}

// Rules ensures c's ChainID is not nil.
func (c *ChainConfig) Rules(epoch *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return Rules{
		ChainID:     new(big.Int).Set(chainID),
		IsCrossLink: c.IsCrossLink(epoch),
		IsEIP155:    c.IsEIP155(epoch),
		IsS3:        c.IsS3(epoch),
	}
}
