package blockfactory

import (
	"math/big"

	"github.com/harmony-one/harmony/block"
	blockif "github.com/harmony-one/harmony/block/interface"
	v0 "github.com/harmony-one/harmony/block/v0"
	v1 "github.com/harmony-one/harmony/block/v1"
	v2 "github.com/harmony-one/harmony/block/v2"
	v3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/internal/params"
)

// Factory is a data structure factory for a specific chain configuration.
type Factory interface {
	// NewHeader creates a new, empty header object for the given epoch.
	NewHeader(epoch *big.Int) *block.Header
}

type factory struct {
	chainConfig *params.ChainConfig
}

// NewFactory creates a new factory for the given chain configuration.
func NewFactory(chainConfig *params.ChainConfig) Factory {
	return &factory{chainConfig: chainConfig}
}

func (f *factory) NewHeader(epoch *big.Int) *block.Header {
	var impl blockif.Header
	switch {
	case f.chainConfig.IsPreStaking(epoch) || f.chainConfig.IsStaking(epoch):
		impl = v3.NewHeader()
	case f.chainConfig.IsCrossLink(epoch):
		impl = v2.NewHeader()
	case f.chainConfig.HasCrossTxFields(epoch):
		impl = v1.NewHeader()
	default:
		impl = v0.NewHeader()
	}
	impl.SetEpoch(epoch)
	return &block.Header{Header: impl}
}

// Factories corresponding to well-known chain configurations.
var (
	ForTest      = NewFactory(params.TestChainConfig)
	ForTestnet   = NewFactory(params.TestnetChainConfig)
	ForMainnet   = NewFactory(params.MainnetChainConfig)
	ForPangaea   = NewFactory(params.PangaeaChainConfig)
	ForPartner   = NewFactory(params.PartnerChainConfig)
	ForStressnet = NewFactory(params.StressnetChainConfig)
)

// NewTestHeader creates a new, empty header object for epoch 0 using the test
// factory.  Use for unit tests.
func NewTestHeader() *block.Header {
	return ForTest.NewHeader(new(big.Int))
}
