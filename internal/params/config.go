package params

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Genesis hashes to enforce below configs on.
var (
	// TODO: populate with real genesis hash (sharded)
	MainnetGenesisHash = common.HexToHash("0x")
	TestnetGenesisHash = common.HexToHash("0x")
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(1),
		CrossLinkEpoch: big.NewInt(21),
		EIP155Epoch:    big.NewInt(20),
		S3Epoch:        big.NewInt(20),
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(2),
		CrossLinkEpoch: big.NewInt(0),
		EIP155Epoch:    big.NewInt(0),
		S3Epoch:        big.NewInt(0),
	}

	// AllProtocolChanges ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllProtocolChanges = &ChainConfig{big.NewInt(100), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	// TestChainConfig ...
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	TestChainConfig = &ChainConfig{big.NewInt(99), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

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

	CrossLinkEpoch *big.Int `json:"homesteadBlock,omitempty"`

	EIP155Epoch *big.Int `json:"eip155Block,omitempty"` // EIP155 hard fork epoch (include EIP158 too)
	S3Epoch     *big.Int `json:"s3Block,omitempty"`     // S3 epoch is the first epoch containing S3 mainnet and all ethereum update up to Constantinople
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EIP155: %v CrossLink: %v}",
		c.ChainID,
		c.CrossLinkEpoch,
		c.EIP155Epoch,
	)
}

// IsEIP155 returns whether epoch is either equal to the EIP155 fork epoch or greater.
func (c *ChainConfig) IsEIP155(epoch *big.Int) bool {
	return isForked(c.EIP155Epoch, epoch)
}

// IsCrossLink returns whether epoch is either equal to the CrossLink fork epoch or greater.
func (c *ChainConfig) IsCrossLink(epoch *big.Int) bool {
	return isForked(c.CrossLinkEpoch, epoch)
}

// IsS3 returns whether epoch is either equal to the S3 fork epoch or greater.
func (c *ChainConfig) IsS3(epoch *big.Int) bool {
	return isForked(c.CrossLinkEpoch, epoch)
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
