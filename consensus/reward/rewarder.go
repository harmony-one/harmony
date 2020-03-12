package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/shard"
)

// PayoutRound ..
type PayoutRound struct {
	Addr        common.Address
	NewlyEarned *big.Int
}

// Payout ..
type Payout struct {
	Total *big.Int
	Round []PayoutRound
}

// Reader ..
type Reader interface {
	// ReadRoundResult ..
	ReadRoundResult() *Payout
	MissingSigners() shard.SlotList
}
