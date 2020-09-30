package common

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
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

	// UnsupportedCurveTypeError ..
	UnsupportedCurveTypeError = types.Error{
		Code:      8,
		Message:   "unsupported curve type",
		Retriable: false,
	}

	// InvalidTransactionConstructionError ..
	InvalidTransactionConstructionError = types.Error{
		Code:      9,
		Message:   "invalid transaction construction",
		Retriable: false,
	}
)

// chopPath returns the source filename after the last slash.
// Inspired by https://github.com/jimlawless/whereami
func chopPath(original string) string {
	i := strings.LastIndex(original, "/")
	if i == -1 {
		return original
	}
	return original[i+1:]
}

// traceMessage return a string containing the file name, function name
// and the line number of a specified entry on the call stack.
// Inspired by https://github.com/jimlawless/whereami
func traceMessage(depthList ...int) string {
	var depth int
	if depthList == nil {
		depth = 1
	} else {
		depth = depthList[0]
	}
	function, file, line, _ := runtime.Caller(depth)
	return fmt.Sprintf("File: %s  Function: %s Line: %d",
		chopPath(file), runtime.FuncForPC(function).Name(), line,
	)
}

// NewError create a new error with a given detail structure
func NewError(rosettaError types.Error, detailStructure interface{}) *types.Error {
	newError := rosettaError
	details, err := types.MarshalMap(detailStructure)
	if err != nil {
		newError.Details = map[string]interface{}{
			"message": fmt.Sprintf("unable to get error details: %v", err.Error()),
		}
	} else {
		newError.Details = details
	}
	newError.Details["trace"] = traceMessage(2)
	newError.Details["rosetta_version"] = RosettaVersion
	newError.Details["node_version"] = nodeconfig.GetVersion()
	return &newError
}
