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
	) (*staking.ValidatorWrapper, error)
}

// Header is the interface of block.Header for calculating the BallotResult.
type Header interface {
	Number() *big.Int
	ShardID() uint32
	LastCommitBitmap() []byte
}

// StateDB is the interface of state.DB
type StateDB interface {
	ValidatorWrapper(common.Address) (*staking.ValidatorWrapper, error)
	UpdateValidatorWrapper(common.Address, *staking.ValidatorWrapper) error
}
