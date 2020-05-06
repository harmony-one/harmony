package slash

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// CommitteeReader ..
type CommitteeReader interface {
	Config() *params.ChainConfig
	ReadShardState(epoch *big.Int) (*shard.State, error)
	CurrentBlock() *types.Block
}

// stateDB is the abstracted interface for *state.DB
type stateDB interface {
	ValidatorWrapper(addr common.Address) (*staking.ValidatorWrapper, error)
	AddBalance(addr common.Address, amount *big.Int)
}
