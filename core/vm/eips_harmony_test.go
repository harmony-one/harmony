package vm

import (
	"math/big"
	"testing"

	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestOpChainID(t *testing.T) {
	const ethCompatibleChainID = 12345
	t.Run("disabled-chainid-fix", func(t *testing.T) {
		v := EVMInterpreter{
			evm: &EVM{
				Context: BlockContext{
					EpochNumber: big.NewInt(100),
				},
				chainConfig: &params.ChainConfig{
					ChainIdFixEpoch:      big.NewInt(1323),
					EthCompatibleChainID: big.NewInt(ethCompatibleChainID),
					ChainID:              big.NewInt(1),
				},
			},
		}

		stack := newstack()
		_, err := opChainID(nil, &v, &ScopeContext{Stack: stack})
		if err != nil {
			t.Fatalf("opChainID error: %v", err)
		}
		rs := stack.pop()
		require.Equal(t, rs.Uint64(), uint64(1))
	})

	t.Run("enabled-chainid-fix", func(t *testing.T) {
		v := EVMInterpreter{
			evm: &EVM{
				Context: BlockContext{
					EpochNumber: big.NewInt(1325),
				},
				chainConfig: &params.ChainConfig{
					ChainIdFixEpoch:      big.NewInt(1323),
					EthCompatibleChainID: big.NewInt(ethCompatibleChainID),
					ChainID:              big.NewInt(1),
				},
			},
		}

		stack := newstack()
		_, err := opChainID(nil, &v, &ScopeContext{Stack: stack})
		if err != nil {
			t.Fatalf("opChainID error: %v", err)
		}
		rs := stack.pop()
		require.Equal(t, rs.Uint64(), uint64(ethCompatibleChainID))
	})
}

func TestAutoEIPActivationRespectsEpochBoundaries(t *testing.T) {
	baseCfg := &params.ChainConfig{
		ChainID:                               big.NewInt(1),
		EthCompatibleChainID:                  big.NewInt(1),
		EthCompatibleShard0ChainID:            big.NewInt(1),
		EIP155Epoch:                           big.NewInt(0),
		S3Epoch:                               big.NewInt(0),
		CrossTxEpoch:                          big.NewInt(0),
		MinCommissionPromoPeriod:              big.NewInt(10),
		ReceiptLogEpoch:                       big.NewInt(0),
		PreStakingEpoch:                       big.NewInt(0),
		CrossLinkEpoch:                        big.NewInt(0),
		StakingEpoch:                          big.NewInt(1),
		QuickUnlockEpoch:                      big.NewInt(0),
		FiveSecondsEpoch:                      big.NewInt(0),
		RedelegationEpoch:                     big.NewInt(0),
		IstanbulEpoch:                         big.NewInt(0),
		TwoSecondsEpoch:                       big.NewInt(0),
		EthCompatibleEpoch:                    big.NewInt(0),
		SixtyPercentEpoch:                     big.NewInt(0),
		NoEarlyUnlockEpoch:                    big.NewInt(0),
		VRFEpoch:                              big.NewInt(0),
		MinDelegation100Epoch:                 big.NewInt(0),
		MinCommissionRateEpoch:                big.NewInt(0),
		EPoSBound35Epoch:                      big.NewInt(0),
		AggregatedRewardEpoch:                 big.NewInt(1),
		PrevVRFEpoch:                          big.NewInt(0),
		DataCopyFixEpoch:                      big.NewInt(0),
		SHA3Epoch:                             big.NewInt(0),
		HIP6And8Epoch:                         big.NewInt(0),
		StakingPrecompileEpoch:                big.NewInt(0),
		EIP2537PrecompileEpoch:                big.NewInt(0),
		EIP3855Epoch:                          big.NewInt(100),
		ChainIdFixEpoch:                       big.NewInt(0),
		SlotsLimitedEpoch:                     big.NewInt(0),
		CrossShardXferPrecompileEpoch:         big.NewInt(1),
		AllowlistEpoch:                        big.NewInt(0),
		LeaderRotationInternalValidatorsEpoch: big.NewInt(0),
		LeaderRotationExternalValidatorsEpoch: big.NewInt(0),
		LeaderRotationV2Epoch:                 big.NewInt(0),
		FeeCollectEpoch:                       big.NewInt(1),
		ValidatorCodeFixEpoch:                 big.NewInt(0),
		HIP30Epoch:                            big.NewInt(2),
		DevnetExternalEpoch:                   big.NewInt(0),
		TestnetExternalEpoch:                  big.NewInt(0),
		BlockGas30MEpoch:                      big.NewInt(0),
		MaxRateEpoch:                          big.NewInt(2),
		TopMaxRateEpoch:                       big.NewInt(0),
		HIP32Epoch:                            big.NewInt(0),
		IsOneSecondEpoch:                      big.NewInt(0),
		EIP1153TransientStorageEpoch:          big.NewInt(0),
		EIP7939CLZEpoch:                       big.NewInt(0),
		EIP5656McopyEpoch:                     big.NewInt(0),
		EIP3860Epoch:                          big.NewInt(100),
		EIP6780Epoch:                          big.NewInt(100),
		NTPEpoch:                              big.NewInt(0),
		TimestampValidationEpoch:              big.NewInt(0),
		PragueEpoch:                           big.NewInt(0),
		EIP8024Epoch:                          big.NewInt(100),
	}

	t.Run("before-activation-keeps-legacy-behavior", func(t *testing.T) {
		evm := NewEVM(
			BlockContext{EpochNumber: big.NewInt(99)},
			TxContext{},
			nil,
			baseCfg,
			Config{},
		)
		jt := evm.interpreter.cfg.JumpTable

		require.Equal(t, minStack(0, 0), jt[PUSH0].minStack)
		require.Equal(t, maxStack(0, 0), jt[PUSH0].maxStack)
		require.Equal(t, minStack(0, 0), jt[DUPN].minStack)
		require.Equal(t, minStack(0, 0), jt[SWAPN].minStack)
		require.Equal(t, minStack(0, 0), jt[EXCHANGE].minStack)

		stack := newstack()
		stack.push(uint256.NewInt(ethparams.MaxInitCodeSize + 1))
		stack.push(uint256.NewInt(0))
		stack.push(uint256.NewInt(0))
		_, err := jt[CREATE].dynamicGas(evm, nil, stack, NewMemory(), 0)
		require.NoError(t, err)
	})

	t.Run("at-activation-enables-new-behavior", func(t *testing.T) {
		evm := NewEVM(
			BlockContext{EpochNumber: big.NewInt(100)},
			TxContext{},
			nil,
			baseCfg,
			Config{},
		)
		jt := evm.interpreter.cfg.JumpTable

		require.Equal(t, minStack(0, 1), jt[PUSH0].minStack)
		require.Equal(t, maxStack(0, 1), jt[PUSH0].maxStack)
		require.Equal(t, minStack(1, 0), jt[DUPN].minStack)
		require.Equal(t, minStack(2, 0), jt[SWAPN].minStack)
		require.Equal(t, minStack(2, 0), jt[EXCHANGE].minStack)

		stack := newstack()
		stack.push(uint256.NewInt(ethparams.MaxInitCodeSize + 1))
		stack.push(uint256.NewInt(0))
		stack.push(uint256.NewInt(0))
		_, err := jt[CREATE].dynamicGas(evm, nil, stack, NewMemory(), 0)
		require.ErrorIs(t, err, ErrGasUintOverflow)
	})
}
