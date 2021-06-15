package common

import (
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
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

	// InvalidStakingConstructionError ..
	InvalidStakingConstructionError = types.Error{
		Code:      10,
		Message:   "invalid staking transaction construction",
		Retriable: false,
	}

	// ErrCallParametersInvalid ..
	ErrCallParametersInvalid = types.Error{
		Code:      11,
		Message:   "invalid call parameters",
		Retriable: false,
	}

	// ErrCallExecute ..
	ErrCallExecute = types.Error{
		Code:      12,
		Message:   "call execute error",
		Retriable: false,
	}

	// ErrCallMethodInvalid ..
	ErrCallMethodInvalid = types.Error{
		Code:      13,
		Message:   "call method invalid",
		Retriable: false,
	}

	// ErrGetStakingInfo ..
	ErrGetStakingInfo = types.Error{
		Code:      14,
		Message:   "get staking info error",
		Retriable: false,
	}
)

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
	newError.Details["trace"] = utils.GetCallStackInfo(2)
	newError.Details["rosetta_version"] = RosettaVersion
	newError.Details["node_version"] = nodeconfig.GetVersion()
	return &newError
}
