package structs

import (
	"math/big"

	"github.com/harmony-one/harmony/core/types"

	"github.com/ethereum/go-ethereum/common"
)

// StakeInfoReturnValue is the struct for the return value of listLockedAddresses func in stake contract.
type StakeInfoReturnValue struct {
	LockedAddresses  []common.Address
	BlsPubicKeys1    [][32]byte
	BlsPubicKeys2    [][32]byte
	BlsPubicKeys3    [][32]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int // The number of locking period the token will be locked.
	Amounts          []*big.Int
}

// StakeInfo stores the staking information for a staker.
type StakeInfo struct {
	BlsPublicKey    types.BlsPublicKey
	BlockNum        *big.Int
	LockPeriodCount *big.Int // The number of locking period the token will be locked.
	Amount          *big.Int
}

// PlayersInfo stores the result of getPlayers.
type PlayersInfo struct {
	Players  []common.Address
	Balances []*big.Int
}
