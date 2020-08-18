package common

import (
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
)

// WARNING: Careful for client side dependencies when changing errors!
func TestErrorCodes(t *testing.T) {
	// WARNING: Order Matters
	errors := []types.Error{
		CatchAllError,
		SanityCheckError,
		InvalidNetworkError,
		TransactionSubmissionError,
		StakingTransactionSubmissionError,
		BlockNotFoundError,
		TransactionNotFoundError,
		ReceiptNotFoundError,
	}

	for i, err := range errors {
		if int(err.Code) != i {
			t.Errorf("Expected error code of %v to be %v", err, i)
		}
	}
}

// WARNING: Careful for client side dependencies when changing errors!
func TestRetryableError(t *testing.T) {
	retriableErrors := []types.Error{
		TransactionSubmissionError,
		StakingTransactionSubmissionError,
	}
	unRetriableErrors := []types.Error{
		CatchAllError,
		SanityCheckError,
		InvalidNetworkError,
		BlockNotFoundError,
		TransactionNotFoundError,
		ReceiptNotFoundError,
	}

	for _, err := range retriableErrors {
		if err.Retriable != true {
			t.Errorf("Expected error code %v to be retriable", err.Code)
		}
	}

	for _, err := range unRetriableErrors {
		if err.Retriable != false {
			t.Errorf("Expected error code %v to not be retriable", err.Code)
		}
	}
}
