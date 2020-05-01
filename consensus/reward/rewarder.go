package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/shard"
)

// Payout ..
type Payout struct {
	ShardID     uint32
	Addr        common.Address
	NewlyEarned *big.Int
	EarningKey  shard.BLSPublicKey
}

// CompletedRound ..
type CompletedRound struct {
	Total            *big.Int
	BeaconchainAward []Payout
	ShardChainAward  []Payout
}

// Reader ..
type Reader interface {
	ReadRoundResult() *CompletedRound
	MissingSigners() shard.SlotList
}
