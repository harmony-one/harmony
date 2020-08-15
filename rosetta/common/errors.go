package common

import (
	"github.com/coinbase/rosetta-sdk-go/types"
)

var (
	// CatchAllError ..
	CatchAllError = types.Error{
		Code:      0,
		Message:   "catch all error",
		Retriable: false,
	}

	// SanityCheckError ..
	SanityCheckError = types.Error{
		Code:      1,
		Message:   "sanity check error",
		Retriable: false,
	}

	// TransactionSubmissionError ..
	TransactionSubmissionError = types.Error{
		Code:      2,
		Message:   "transaction submission error",
		Retriable: true,
	}

	// StakingTransactionSubmissionError ..
	StakingTransactionSubmissionError = types.Error{
		Code:      3,
		Message:   "staking transaction submission error",
		Retriable: true,
	}
)
