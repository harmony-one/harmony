package economics

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/engine"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
)

// ComputedAPR ..
type ComputedAPR struct {
	Validator        common.Address
	TotalStakedToken *big.Int    `json:"total-staked-token"`
	StakeRatio       numeric.Dec `json:"stake-ratio"`
	APR              numeric.Dec `json:"computed-apr"`
}

// MarshalJSON ..
func (c *ComputedAPR) MarshalJSON() ([]byte, error) {
	type t struct {
		ComputedAPR
		Validator string `json:"earning-account"`
	}
	wrap := t{}
	wrap.TotalStakedToken = c.TotalStakedToken
	wrap.StakeRatio = c.StakeRatio
	wrap.APR = c.APR
	wrap.Validator = common2.MustAddressToBech32(c.Validator)
	return json.Marshal(wrap)

}

// UtilityMetric ..
type UtilityMetric struct {
	AccumulatorSnapshot     *big.Int      `json:"total-block-reward-accum"`
	CurrentStakedPercentage numeric.Dec   `json:"current-percent-token-staked"`
	Deviation               numeric.Dec   `json:"current-percent-network-deviation"`
	Adjustment              numeric.Dec   `json:"reward-bonus"`
	ActiveValidatorsAPR     []ComputedAPR `json:"active-validators-apr"`
}

// NewUtilityMetricSnapshot ..
func NewUtilityMetricSnapshot(
	beaconchain engine.ChainReader,
) (*UtilityMetric, error) {
	soFarDoledOut, computedAPRs, percentageStaked, err := Snapshot(
		beaconchain, time.Now().Unix(), true,
	)
	if err != nil {
		return nil, err
	}
	howMuchOff, adjustBy := Adjustment(*percentageStaked)
	return &UtilityMetric{
		soFarDoledOut, *percentageStaked, howMuchOff, adjustBy, computedAPRs,
	}, nil
}
