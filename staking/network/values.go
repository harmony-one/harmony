package network

import (
	"encoding/json"
	"math/big"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/numeric"
)

// UtilityMetric ..
type UtilityMetric struct {
	AccumulatorSnapshot     *big.Int
	CurrentStakedPercentage numeric.Dec
	Deviation               numeric.Dec
	Adjustment              numeric.Dec
}

// MarshalJSON for UtilityMetric.
func (u UtilityMetric) MarshalJSON() ([]byte, error) {
	type UtilityMetric struct {
		AccumulatorSnapshot     *big.Int    `json:"accumulator"`
		CurrentStakedPercentage numeric.Dec `json:"stakedPercentage"`
		Deviation               numeric.Dec `json:"deviation"`
		Adjustment              numeric.Dec `json:"adjustment"`
	}
	var enc UtilityMetric
	enc.AccumulatorSnapshot = u.AccumulatorSnapshot
	enc.CurrentStakedPercentage = u.CurrentStakedPercentage
	enc.Deviation = u.Deviation
	enc.Adjustment = u.Adjustment
	return json.Marshal(&enc)
}
	
// NewUtilityMetricSnapshot ..
func NewUtilityMetricSnapshot(beaconchain engine.ChainReader) (*UtilityMetric, error) {
	soFarDoledOut, percentageStaked, err := WhatPercentStakedNow(
		beaconchain, time.Now().Unix(),
	)
	if err != nil {
		return nil, err
	}
	howMuchOff, adjustBy := Adjustment(*percentageStaked)
	return &UtilityMetric{
		soFarDoledOut, *percentageStaked, howMuchOff, adjustBy,
	}, nil
}
