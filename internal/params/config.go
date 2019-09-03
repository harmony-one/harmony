package params

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = common.HexToHash("0x")
	TestnetGenesisHash = common.HexToHash("0x")
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(1),
		CrossLinkBlock: big.NewInt(655360), // 40 * 2^14
		EIP155Block:    big.NewInt(655360), // 40 * 2^14
		S3Block:        big.NewInt(655360), // 40 * 2^14
	}

	// TestnetChainConfig contains the chain parameters to run a node on the harmony test network.
	TestnetChainConfig = &ChainConfig{
		ChainID:        big.NewInt(2),
		CrossLinkBlock: big.NewInt(0),
		EIP155Block:    big.NewInt(0),
		S3Block:        big.NewInt(0),
	}

	// AllProtocolChanges ...
	AllProtocolChanges = &ChainConfig{big.NewInt(100), big.NewInt(0), big.NewInt(0), big.NewInt(0)}

	// TestChainConfig ...
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

	CrossLinkBlock *big.Int `json:"homesteadBlock,omitempty"`

	EIP155Block *big.Int `json:"eip155Block,omitempty"` // EIP155 HF block (include EIP158 too)
	S3Block     *big.Int `json:"s3Block,omitempty"`     // S3 block is the first block containing S3 mainnet and all ethereum update up to Constantinople
}

// EthashConfig is the consensus engine configs for proof-of-work based sealing.
type EthashConfig struct{}

// String implements the stringer interface, returning the consensus engine details.
func (c *EthashConfig) String() string {
	return "ethash"
}

// CliqueConfig is the consensus engine configs for proof-of-authority based sealing.
type CliqueConfig struct {
	Period uint64 `json:"period"` // Number of seconds between blocks to enforce
	Epoch  uint64 `json:"epoch"`  // Epoch length to reset votes and checkpoint
}

// String implements the stringer interface, returning the consensus engine details.
func (c *CliqueConfig) String() string {
	return "clique"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v EIP155: %v CrossLink: %v}",
		c.ChainID,
		c.CrossLinkBlock,
		c.EIP155Block,
	)
}

// IsEIP155 returns whether num is either equal to the EIP155 fork block or greater.
func (c *ChainConfig) IsEIP155(num *big.Int) bool {
	return isForked(c.EIP155Block, num)
}

// IsCrossLink returns whether num is either equal to the CrossLink fork block or greater.
func (c *ChainConfig) IsCrossLink(num *big.Int) bool {
	return isForked(c.CrossLinkBlock, num)
}

// IsS3 returns whether num is either equal to the S3 fork block or greater.
func (c *ChainConfig) IsS3(num *big.Int) bool {
	return isForked(c.CrossLinkBlock, num)
}

// GasTable returns the gas table corresponding to the current phase (homestead or homestead reprice).
//
// The returned GasTable's fields shouldn't, under any circumstances, be changed.
func (c *ChainConfig) GasTable(num *big.Int) GasTable {
	if num == nil {
		return GasTableR3
	}
	switch {
	case c.IsS3(num):
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
	if isForkIncompatible(c.EIP155Block, newcfg.EIP155Block, head) {
		return newCompatError("EIP155 fork block", c.EIP155Block, newcfg.EIP155Block)
	}
	if isForkIncompatible(c.CrossLinkBlock, newcfg.CrossLinkBlock, head) {
		return newCompatError("CrossLink fork block", c.CrossLinkBlock, newcfg.CrossLinkBlock)
	}
	if isForkIncompatible(c.S3Block, newcfg.S3Block, head) {
		return newCompatError("S3 fork block", c.S3Block, newcfg.S3Block)
	}
	return nil
}

// isForkIncompatible returns true if a fork scheduled at s1 cannot be rescheduled to
// block s2 because head is already past the fork.
func isForkIncompatible(s1, s2, head *big.Int) bool {
	return (isForked(s1, head) || isForked(s2, head)) && !configNumEqual(s1, s2)
}

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
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
func (c *ChainConfig) Rules(num *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return Rules{
		ChainID:     new(big.Int).Set(chainID),
		IsCrossLink: c.IsCrossLink(num),
		IsEIP155:    c.IsEIP155(num),
		IsS3:        c.IsS3(num),
	}
}
