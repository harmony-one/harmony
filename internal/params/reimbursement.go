package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Reimbursement struct {
	TotalAmount *big.Int
	Duration    *big.Int
	Receiver    common.Address
}

var (
	zero                  = big.NewInt(0)
	oneUint               = big.NewInt(1e18)                               // decimals of ONE is 18.
	totalReimbursement    = new(big.Int).Mul(big.NewInt(2.391e9), oneUint) // 2.391 billion ONE
	reimbursementDuration = big.NewInt(3600 * 24 * 365 * 3)                // 3 years

	MainnetReimbursement = Reimbursement{
		TotalAmount: totalReimbursement,
		Duration:    reimbursementDuration,
		Receiver:    common.Address{},
	}

	TestnetReimbursement = Reimbursement{
		TotalAmount: totalReimbursement,
		Duration:    reimbursementDuration,
		Receiver:    common.Address{},
	}
)
