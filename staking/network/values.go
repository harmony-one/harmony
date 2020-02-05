package network

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"
)

// UtilityMetric ..
type UtilityMetric struct {
	AccumulatorSnapshot     *big.Int
	CurrentStakedPercentage *big.Int
	Deviation               numeric.Dec
	Adjustment              numeric.Dec
}
