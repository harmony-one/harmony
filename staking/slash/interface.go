package slash

import (
	"math/big"

	"github.com/servprotocolorg/harmony/core/types"
	"github.com/servprotocolorg/harmony/internal/params"
	"github.com/servprotocolorg/harmony/shard"
)

// CommitteeReader ..
type CommitteeReader interface {
	Config() *params.ChainConfig
	ReadShardState(epoch *big.Int) (*shard.State, error)
	CurrentBlock() *types.Block
}
