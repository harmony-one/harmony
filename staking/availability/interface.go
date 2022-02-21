package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Reader ..
type Reader interface {
	ReadValidatorSnapshot(
		addr common.Address,
	) (*staking.ValidatorSnapshot, error)
}

// RoundHeader is the interface of block.Header for calculating the BallotResult.
type RoundHeader interface {
	Number() *big.Int
	ShardID() uint32
	LastCommitBitmap() []byte
}

// ValidatorState is the interface of state.DB
type ValidatorState interface {
	ValidatorWrapper(common.Address, bool, bool) (*staking.ValidatorWrapper, error)
	UpdateValidatorWrapper(common.Address, *staking.ValidatorWrapper) error
}
