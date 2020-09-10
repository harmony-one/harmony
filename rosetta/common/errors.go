package common

import (
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/harmony-one/harmony/rpc"
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

	// InvalidNetworkError ..
	InvalidNetworkError = types.Error{
		Code:      2,
		Message:   "invalid network error",
		Retriable: false,
	}

	// TransactionSubmissionError ..
	TransactionSubmissionError = types.Error{
		Code:      3,
		Message:   "transaction submission error",
		Retriable: true,
	}

	// StakingTransactionSubmissionError ..
	StakingTransactionSubmissionError = types.Error{
		Code:      4,
		Message:   "staking transaction submission error",
		Retriable: true,
	}

	// BlockNotFoundError ..
	BlockNotFoundError = types.Error{
		Code:      5,
		Message:   "block not found error",
		Retriable: false,
	}

	// TransactionNotFoundError ..
	TransactionNotFoundError = types.Error{
		Code:      6,
		Message:   "transaction or staking transaction not found",
		Retriable: false,
	}

	// ReceiptNotFoundError ..
	ReceiptNotFoundError = types.Error{
		Code:      7,
		Message:   "receipt not found",
		Retriable: false,
	}
)

// NewError create a new error with a given detail structure
func NewError(rosettaError types.Error, detailStructure interface{}) *types.Error {
	newError := rosettaError
	details, err := rpc.NewStructuredResponse(detailStructure)
	if err != nil {
		newError.Details = map[string]interface{}{
			"message": fmt.Sprintf("unable to get error details: %v", err.Error()),
		}
	} else {
		newError.Details = details
	}
	return &newError
}
