package core

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

type applyStakingMessageTest struct {
	name          string
	tx            *staking.StakingTransaction
	expectedError error
}

var ApplyStakingMessageTests []applyStakingMessageTest
var key *ecdsa.PrivateKey

func init() {
	key, _ = crypto.GenerateKey()
	stakingValidatorMissing := errors.New("staking validator does not exist")
	ApplyStakingMessageTests = []applyStakingMessageTest{
		{
			tx:   signedCreateValidatorStakingTxn(key),
			name: "ApplyStakingMessage_CreateValidator",
		},
		{
			tx:            signedEditValidatorStakingTxn(key),
			expectedError: stakingValidatorMissing,
			name:          "ApplyStakingMessage_EditValidator",
		},
		{
			tx:            signedDelegateStakingTxn(key),
			expectedError: stakingValidatorMissing,
			name:          "ApplyStakingMessage_Delegate",
		},
		{
			tx:            signedUndelegateStakingTxn(key),
			expectedError: stakingValidatorMissing,
			name:          "ApplyStakingMessage_Undelegate",
		},
		{
			tx:            signedCollectRewardsStakingTxn(key),
			expectedError: errors.New("no rewards to collect"),
			name:          "ApplyStakingMessage_CollectRewards",
		},
	}
}

func TestApplyStakingMessages(t *testing.T) {
	for _, test := range ApplyStakingMessageTests {
		testApplyStakingMessage(test, t)
	}
}

func testApplyStakingMessage(test applyStakingMessageTest, t *testing.T) {
	chain, db, header, _ := getTestEnvironment(*key)
	gp := new(GasPool).AddGas(math.MaxUint64)
	t.Run(fmt.Sprintf("%s", test.name), func(t *testing.T) {
		// add a fake staking transaction
		msg, _ := StakingToMessage(test.tx, header.Number())

		// make EVM
		ctx := NewEVMContext(msg, header, chain, nil /* coinbase */)
		vmenv := vm.NewEVM(ctx, db, params.TestChainConfig, vm.Config{})

		// run the staking tx
		_, err := ApplyStakingMessage(vmenv, msg, gp, chain)
		if err != nil {
			if test.expectedError == nil {
				t.Errorf(fmt.Sprintf("Got error %v but expected none", err))
			} else if test.expectedError.Error() != err.Error() {
				t.Errorf(fmt.Sprintf("Got error %v, but expected %v", err, test.expectedError))
			}
		}
	})
}
