package slash

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// Evidence ..
type Evidence struct {
	Header  *block.Header `json:"header"`
	Epoch   *big.Int
	ShardID uint32
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	AlreadyCastBallot  votepower.Ballot `json:"already-cast-vote"`
	DoubleSignedBallot votepower.Ballot `json:"double-signed-vote"`
	// the reporter who will get rewarded
	Beneficiary common.Address `json:"beneficiary"`
	Epoch       *big.Int
}

// NewRecord ..
func NewRecord(
	header *block.Header,
	alreadySigned, doubleSigned *votepower.Ballot,
	beneficiary common.Address,
) Record {
	// header, *alreadySigned, *doubleSigned, beneficiary,
	return Record{}
}

// Verify checks that the signature is valid
func Verify(candidate *Record) error {
	fmt.Println("need to verify the slash", candidate)

	return nil
}

// Apply ..
func Apply(state *state.DB, slashes []Record, committee []shard.BlsPublicKey) error {
	rate := Rate(uint32(len(slashes)), uint32(len(committee)))

	// for _, slash := range slashes {

	// }

	fmt.Println("applying slash with a rate of", rate, slashes, committee)
	return nil
}

var (
	zero                = numeric.ZeroDec()
	oneDoubleSignerRate = numeric.MustNewDecFromStr("0.02")
)

// Rate is the slashing % rate
func Rate(doubleSignerCount, committeeSize uint32) numeric.Dec {
	if doubleSignerCount == 0 || committeeSize == 0 {
		return zero
	}
	switch doubleSignerCount {
	case 1:
		return oneDoubleSignerRate
	default:
		return numeric.NewDec(
			int64(doubleSignerCount),
		).Quo(numeric.NewDec(int64(committeeSize)))
	}
}
