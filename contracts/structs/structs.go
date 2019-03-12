package structs

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// StakeInfoReturnValue is the struct for the return value of listLockedAddresses func in stake contract.
type StakeInfoReturnValue struct {
	LockedAddresses  []common.Address
	BlsAddresses     [][20]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int // The number of locking period the token will be locked.
	Amounts          []*big.Int
}

// StakeInfo stores the staking information for a staker.
type StakeInfo struct {
	BlsAddress      [20]byte
	BlockNum        *big.Int
	LockPeriodCount *big.Int // The number of locking period the token will be locked.
	Amount          *big.Int
}
