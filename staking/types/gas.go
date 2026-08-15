package types

import (
	"math"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/pkg/errors"
)

var (
	// ErrBatchTooLarge is returned when a batch staking tx exceeds MaxBatchStakingActions.
	ErrBatchTooLarge = errors.New("batch staking action count exceeds maximum")
)

// ExtraGasForStakingDirective returns gas for batch staking directives.
// data is the RLP-encoded stake message payload.
func ExtraGasForStakingDirective(directive Directive, data []byte) (uint64, error) {
	switch directive {
	case DirectiveBatchDelegate:
		msg, err := RLPDecodeStakeMsg(data, DirectiveBatchDelegate)
		if err != nil {
			return 0, err
		}
		batch, ok := msg.(*BatchDelegate)
		if !ok {
			return 0, ErrInvalidStakingKind
		}
		n := len(batch.Delegations)
		if n > MaxBatchStakingActions {
			return 0, ErrBatchTooLarge
		}
		return mulGas(uint64(n), params.TxGasPerBatchStakingAction)
	case DirectiveBatchUndelegate:
		msg, err := RLPDecodeStakeMsg(data, DirectiveBatchUndelegate)
		if err != nil {
			return 0, err
		}
		batch, ok := msg.(*BatchUndelegate)
		if !ok {
			return 0, ErrInvalidStakingKind
		}
		n := len(batch.Undelegations)
		if n > MaxBatchStakingActions {
			return 0, ErrBatchTooLarge
		}
		return mulGas(uint64(n), params.TxGasPerBatchStakingAction)
	case DirectiveUndelegateAll:
		return params.TxGasUndelegateAll, nil
	default:
		return 0, nil
	}
}

func mulGas(count, per uint64) (uint64, error) {
	if count == 0 {
		return 0, nil
	}
	if per != 0 && count > math.MaxUint64/per {
		return 0, errors.New("staking batch gas overflow")
	}
	return count * per, nil
}
