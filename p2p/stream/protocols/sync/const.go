package sync

import "time"

const (
	// GetBlockHashesAmountCap is the cap of GetBlockHashes reqeust
	GetBlockHashesAmountCap = 50

	// GetBlocksByNumAmountCap is the cap of request of a single GetBlocksByNum request
	GetBlocksByNumAmountCap = 10

	// GetBlocksByHashesAmountCap is the cap of request of single GetBlocksByHashes request
	GetBlocksByHashesAmountCap = 10

	// minAdvertiseInterval is the minimum advertise interval
	minAdvertiseInterval = 1 * time.Minute
)
