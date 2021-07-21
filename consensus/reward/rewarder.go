package reward

import (
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
)

// Payout ..
type Payout struct {
	Addr        common.Address
	NewlyEarned *big.Int
	EarningKey  bls.SerializedPublicKey
}

// CompletedRound ..
type CompletedRound struct {
	Total   *big.Int
	Payouts []Payout
}

// Reader ..
type Reader interface {
	ReadRoundResult() *CompletedRound
}
