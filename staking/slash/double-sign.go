package slash

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/shard"
)

// Slasher ..
type Slasher interface {
	ShouldSlash(shard.BlsPublicKey) bool
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	BlockHash   common.Hash
	BlockNumber *big.Int
	Signature   [96]byte // (aggregated) signature
	Bitmap      []byte   // corresponding bitmap mask for agg signature
	ShardID     uint32
	Epoch       *big.Int
	Beneficiary common.Address // the reporter who will get rewarded
}

// DidAnyoneDoubleSign ..
func DidAnyoneDoubleSign(d quorum.Decider) bool {
	return false
}
