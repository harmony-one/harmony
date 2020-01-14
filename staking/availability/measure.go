package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/pkg/errors"
)

var (
	measure                    = new(big.Int).Div(big.NewInt(2), big.NewInt(3))
	errValidatorEpochDeviation = errors.New("validator snapshot epoch not exactly one epoch behind")
	errNegativeSign            = errors.New("impossible period of signing")
)

// SetInactiveUnavailableValidators sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func SetInactiveUnavailableValidators(
	addrs []common.Address, batch ethdb.Batch, bc consensus_engine.ChainReader,
) error {
	one, now := big.NewInt(1), bc.CurrentHeader().Epoch()
	for i := range addrs {

		snapshot, err := bc.ReadValidatorSnapshot(addrs[i])
		if err != nil {
			return err
		}
		stats, err := bc.ReadValidatorStats(addrs[i])
		if err != nil {
			return err
		}

		// NOTE Temp sanity check that each snapshot is indeed
		// happening correctly on epoch. Once comfortable with
		// protocol code robustness, then remove this code and
		// the associated .Epoch field of ValidatorSnapshot
		if diff := new(big.Int).Sub(now, snapshot.Epoch); diff.Cmp(one) != 0 {
			return errors.Wrapf(
				errValidatorEpochDeviation, "bc %s, snapshot %s",
				now.String(), snapshot.Epoch.String(),
			)
		}

		signed := new(big.Int).Sub(stats.NumBlocksSigned, snapshot.NumBlocksSigned)
		toSign := new(big.Int).Sub(stats.NumBlocksToSign, snapshot.NumBlocksToSign)

		if signed.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for signed period wrong: stat %s, snapshot %s",
				stats.NumBlocksSigned.String(), snapshot.NumBlocksSigned.String(),
			)
		}

		if toSign.Sign() == -1 {
			return errors.Wrapf(
				errNegativeSign, "diff for toSign period wrong: stat %s, snapshot %s",
				stats.NumBlocksToSign.String(), snapshot.NumBlocksToSign.String(),
			)
		}

		if r := new(big.Int).Div(signed, toSign); r.Cmp(measure) == -1 {
			wrapper, err := bc.ReadValidatorInformation(addrs[i])
			if err != nil {
				return err
			}
			wrapper.Active = false
			if writeErr := rawdb.WriteValidatorData(batch, wrapper); writeErr != nil {
				return writeErr
			}
		}
	}
	return nil
}
