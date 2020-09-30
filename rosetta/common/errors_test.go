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
		UnsupportedCurveTypeError,
		InvalidTransactionConstructionError,
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
		UnsupportedCurveTypeError,
		InvalidTransactionConstructionError,
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

func TestNewError(t *testing.T) {
	testErr := NewError(CatchAllError, map[string]interface{}{
		"message": "boom",
	})
	if testErr == &CatchAllError {
		t.Fatal("expected return of a copy of error")
	}
	if testErr.Details == nil {
		t.Fatal("suppose to have details")
	}
	if testErr.Details["trace"] == nil {
		t.Error("expect trace")
	}
}
