package availability

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/rawdb"
)

var (
	measure = new(big.Int).Div(big.NewInt(2), big.NewInt(3))
)

// SetInactiveUnavailableValidators sets the validator to
// inactive and thereby keeping it out of
// consideration in the pool of validators for
// whenever committee selection happens in future, the
// signing threshold is 66%
func SetInactiveUnavailableValidators(
	addrs []common.Address, batch ethdb.Batch, bc consensus_engine.ChainReader,
) error {
	for i := range addrs {

		stats, err := bc.ReadValidatorStats(addrs[i])
		if err != nil {
			return err
		}
		// TODO need to do this on a per epoch basis
		if r := new(big.Int).Div(
			stats.NumBlocksSigned, stats.NumBlocksToSign,
		); r.Cmp(measure) == -1 {
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
