package core

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
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
		_, err := ApplyStakingMessage(vmenv, msg, gp)
		if err != nil {
			if test.expectedError == nil {
				t.Errorf(fmt.Sprintf("Got error %v but expected none", err))
			} else if test.expectedError.Error() != err.Error() {
				t.Errorf(fmt.Sprintf("Got error %v, but expected %v", err, test.expectedError))
			}
		}
	})
}

func TestCollectGas(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	header.SetEpoch(new(big.Int).Set(params.LocalnetChainConfig.FeeCollectEpoch))

	// set the shard schedule so that fee collectors are available
	shard.Schedule = shardingconfig.LocalnetSchedule
	feeCollectors := shard.Schedule.InstanceForEpoch(header.Epoch()).FeeCollectors()
	if len(feeCollectors) == 0 {
		t.Fatal("No fee collectors set")
	}

	tx := types.NewTransaction(
		0, // nonce
		common.BytesToAddress([]byte("to")),
		0,                 // shardid
		big.NewInt(1e18),  // amount, 1 ONE
		50000,             // gasLimit, intentionally higher than the 21000 required
		big.NewInt(100e9), // gasPrice
		[]byte{},          // payload, intentionally empty
	)
	from, _ := tx.SenderAddress()
	initialBalance := big.NewInt(2e18)
	db.AddBalance(from, initialBalance)
	msg, _ := tx.AsMessage(types.NewEIP155Signer(common.Big2))
	ctx := NewEVMContext(msg, header, chain, nil /* coinbase is nil, no block reward */)
	ctx.TxType = types.SameShardTx

	vmenv := vm.NewEVM(ctx, db, params.TestChainConfig, vm.Config{})
	gasPool := new(GasPool).AddGas(math.MaxUint64)
	_, err := ApplyMessage(vmenv, msg, gasPool)
	if err != nil {
		t.Fatal(err)
	}

	// check that sender got the exact expected balance
	balance := db.GetBalance(from)
	fee := new(big.Int).Mul(tx.GasPrice(), new(big.Int).
		SetUint64(
			21000, // base fee for normal transfers
		))
	feePlusAmount := new(big.Int).Add(fee, tx.Value())
	expectedBalance := new(big.Int).Sub(initialBalance, feePlusAmount)
	if balance.Cmp(expectedBalance) != 0 {
		t.Errorf("Balance mismatch for sender: got %v, expected %v", balance, expectedBalance)
	}

	// check that the fee collectors got half of the fees each
	expectedFeePerCollector := new(big.Int).Mul(tx.GasPrice(),
		new(big.Int).SetUint64(21000/2))
	for collector := range feeCollectors {
		balance := db.GetBalance(collector)
		if balance.Cmp(expectedFeePerCollector) != 0 {
			t.Errorf("Balance mismatch for collector %v: got %v, expected %v",
				collector, balance, expectedFeePerCollector)
		}
	}

	// lastly, check the receiver's balance
	balance = db.GetBalance(*tx.To())
	if balance.Cmp(tx.Value()) != 0 {
		t.Errorf("Balance mismatch for receiver: got %v, expected %v", balance, tx.Value())
	}
}

func TestCollectGasRounding(t *testing.T) {
	// We want to test that the fee collectors get the correct amount of fees
	// even if the total fee is not a multiple of the fee ratio.
	// For example, if the fee ratio is 1:1, and the total fee is 1e9, then
	// the fee collectors should get 0.5e9 each.
	// If the gas is 1e9 + 1, then the fee collectors should get 0.5e9 each,
	// with the extra 1 being dropped. This test checks for that.
	// Such a situation can potentially occur, but is not an immediate concern
	// since we require transactions to have a minimum gas price of 100 gwei
	// which is always even (in wei) and can be divided across two collectors.
	// Hypothetically, a gas price of 1 wei * gas used of (21,000 + odd number)
	// could result in such a case which is well handled.
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	header.SetEpoch(new(big.Int).Set(params.LocalnetChainConfig.FeeCollectEpoch))

	// set the shard schedule so that fee collectors are available
	shard.Schedule = shardingconfig.LocalnetSchedule
	feeCollectors := shard.Schedule.InstanceForEpoch(header.Epoch()).FeeCollectors()
	if len(feeCollectors) == 0 {
		t.Fatal("No fee collectors set")
	}

	tx := types.NewTransaction(
		0, // nonce
		common.BytesToAddress([]byte("to")),
		0,                // shardid
		big.NewInt(1e18), // amount, 1 ONE
		5,                // gasLimit
		big.NewInt(1),    // gasPrice
		[]byte{},         // payload, intentionally empty
	)
	from, _ := tx.SenderAddress()
	initialBalance := big.NewInt(2e18)
	db.AddBalance(from, initialBalance)
	msg, _ := tx.AsMessage(types.NewEIP155Signer(common.Big2))
	ctx := NewEVMContext(msg, header, chain, nil /* coinbase is nil, no block reward */)
	ctx.TxType = types.SameShardTx

	vmenv := vm.NewEVM(ctx, db, params.TestChainConfig, vm.Config{})
	gasPool := new(GasPool).AddGas(math.MaxUint64)
	st := NewStateTransition(vmenv, msg, gasPool)
	// buy gas to set initial gas to 5: gasLimit * gasPrice
	if err := st.buyGas(); err != nil {
		t.Fatal(err)
	}
	// set left over gas to 0, so gasUsed is 5
	st.gas = 0
	st.collectGas()

	// check that the fee collectors got the fees in the provided ratio
	expectedFeePerCollector := big.NewInt(2) // GIF(5 / 2) = 2
	for collector := range feeCollectors {
		balance := db.GetBalance(collector)
		if balance.Cmp(expectedFeePerCollector) != 0 {
			t.Errorf("Balance mismatch for collector %v: got %v, expected %v",
				collector, balance, expectedFeePerCollector)
		}
	}
}
