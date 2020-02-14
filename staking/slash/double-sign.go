package slash

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// Moment ..
type Moment struct {
	Epoch        *big.Int `json:"epoch"`
	Height       *big.Int `json:"block-height"`
	ViewID       uint64   `json:"view-id"`
	ShardID      uint32   `json:"shard-id"`
	TimeUnixNano int64    `json:"time-unix-nano"`
}

// Evidence ..
type Evidence struct {
	Moment
	ProposalHeader *block.Header `json:"header"`
}

// ConflictingBallots ..
type ConflictingBallots struct {
	AlreadyCastBallot  votepower.Ballot `json:"already-cast-vote"`
	DoubleSignedBallot votepower.Ballot `json:"double-signed-vote"`
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	ConflictingBallots
	// the reporter who will get rewarded
	Evidence    Evidence       `json:"evidence"`
	Beneficiary common.Address `json:"beneficiary"`
}

// MarshalJSON ..
func (e Evidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Moment
		ProposalHeader string `json:"header"`
	}{e.Moment, e.ProposalHeader.String()})
}

// MarshalJSON ..
func (r Record) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ConflictingBallots
		Evidence    Evidence `json:"evidence"`
		Beneficiary string   `json:"beneficiary"`
	}{r.ConflictingBallots, r.Evidence, common2.MustAddressToBech32(r.Beneficiary)})
}

func (e Evidence) String() string {
	s, _ := json.Marshal(e)
	return string(s)
}

func (r Record) String() string {
	s, _ := json.Marshal(r)
	return string(s)
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
