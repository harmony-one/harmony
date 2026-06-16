package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/quorum"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	corepkg "github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	coretypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	hmybls "github.com/harmony-one/harmony/crypto/bls"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/multibls"
	workerpkg "github.com/harmony-one/harmony/node/harmony/worker"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	stakingRoot "github.com/harmony-one/harmony/staking"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
	"github.com/stretchr/testify/require"
)

func fakeWrapperChainConfig(bindEpoch *big.Int) *params.ChainConfig {
	cfg := *params.MainnetChainConfig
	cfg.ValidatorWrapperAddressBindEpoch = new(big.Int).Set(bindEpoch)
	return &cfg
}

func TestFakeValidatorWrapperMintsUndelegationPayout(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	attackerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	attackEpoch := new(big.Int).Add(params.MainnetChainConfig.HIP32Epoch, big.NewInt(8))
	unlockEpoch := new(big.Int).Sub(attackEpoch, big.NewInt(staking.LockPeriodInEpoch))
	leaderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	validator := common.HexToAddress("0x0000000000000000000000000000000000000a71")
	attacker := crypto.PubkeyToAddress(attackerKey.PublicKey)
	leaderCoinbase := fakeWrapperCoinbase(leaderSigner)

	committeeState := fakeWrapperShardState(attackEpoch, validator, leaderSigner.Pub.Bytes)
	alloc := corepkg.GenesisAlloc{
		attacker: {
			Balance: fakeWrapperOnes(2_000_000),
		},
	}
	chainConfig := params.MainnetChainConfig
	chain, setupState, genesis := fakeWrapperChain(t, alloc, committeeState, chainConfig)
	processor := corepkg.NewStateProcessor(chain, chain)

	realWrapper := fakeWrapperRealValidator(validator, leaderSigner.Pub.Bytes, attackEpoch)

	// The target validator starts with its own clean wrapper and is included in normal payout iteration.
	setupState.SetValidatorFlag(validator)
	require.NoError(t, setupState.UpdateValidatorWrapper(validator, realWrapper))
	require.NoError(t, rawdb.WriteValidatorList(setupState.Database().DiskDB(), []common.Address{validator}))
	snapshot := staketest.CopyValidatorWrapper(*realWrapper)
	require.NoError(t, rawdb.WriteValidatorSnapshot(setupState.Database().DiskDB(), &snapshot, attackEpoch))

	payoutBlockNum := shardingconfig.MainnetSchedule.EpochLastBlock(attackEpoch.Uint64())
	setupBlockNum := payoutBlockNum - shardingconfig.RewardFrequency + 1
	setupHeader := fakeWrapperHeader(attackEpoch, setupBlockNum, leaderCoinbase, genesis.Hash())
	setupBlock, err := fakeWrapperProcessAndCommitBlock(
		t,
		chain,
		processor,
		setupState,
		setupHeader,
		nil,
		nil,
		leaderSigner,
		chainConfig,
	)
	require.NoError(t, err)

	stateBeforeDeploy, err := chain.StateAt(setupBlock.Root())
	require.NoError(t, err)
	validatorBeforeDeploy, err := stateBeforeDeploy.ValidatorWrapper(validator, false, true)
	require.NoError(t, err)

	// The starting wrapper belongs to the real validator and has no attacker payout entry.
	require.Equal(t, "real-validator", validatorBeforeDeploy.Name)
	require.Len(t, validatorBeforeDeploy.Delegations, 1)

	forgedPayout := fakeWrapperOnes(1_000_000)
	minimumDelegation := fakeWrapperOnes(100)

	// The fake wrapper points at the real validator but carries the attacker's unlocked undelegation.
	fakeRuntime := fakeWrapperRuntime(
		t,
		validator,
		attacker,
		leaderSigner.Pub.Bytes,
		minimumDelegation,
		forgedPayout,
		unlockEpoch,
		attackEpoch,
	)
	fakeInitcode := fakeWrapperValidatorInitcode(fakeRuntime)
	fakeContract := crypto.CreateAddress(attacker, 0)
	require.NotEqual(t, validator, fakeContract)
	gasPrice := big.NewInt(100e9)

	deployTx, err := fakeWrapperSignContractCreationTx(
		attackerKey,
		0,
		common.Big0,
		5_000_000,
		gasPrice,
		fakeInitcode,
		attackEpoch,
		chainConfig,
	)
	require.NoError(t, err)

	// The constructor writes the validator flag slot and returns the fake wrapper as the contract code.
	deployConsensus := fakeWrapperConsensus(t, chain, leaderSigner, chainConfig)
	require.NoError(t, deployConsensus.registry.GetTxPool().AddLocal(deployTx))
	deployBlock := fakeWrapperProposeAndInsertBlock(
		t, deployConsensus, chain, leaderSigner, setupBlock.GetCurrentCommitSig(), chainConfig,
	)
	require.Equal(t, setupBlockNum+1, deployBlock.NumberU64())
	require.Len(t, deployBlock.Transactions(), 1)
	require.Empty(t, deployBlock.StakingTransactions())

	deployedState, err := chain.StateAt(deployBlock.Root())
	require.NoError(t, err)
	require.True(t, deployedState.IsValidator(fakeContract))
	require.True(t, bytes.Equal(fakeRuntime, deployedState.GetCode(fakeContract)))

	// Loading the fake contract as a validator decodes the contract code into the attacker-supplied wrapper.
	decodedFromContract, err := deployedState.ValidatorWrapper(fakeContract, false, true)
	require.NoError(t, err)
	require.Equal(t, validator, decodedFromContract.Address)
	require.Zero(t, decodedFromContract.Delegations[1].Amount.Sign())
	require.Len(t, decodedFromContract.Delegations[1].Undelegations, 1)
	require.Zero(t, decodedFromContract.Delegations[1].Undelegations[0].Amount.Cmp(forgedPayout))

	delegateTx, err := fakeWrapperSignDelegateTx(
		attackerKey,
		1,
		fakeContract,
		attacker,
		minimumDelegation,
		1_000_000,
		gasPrice,
		chainConfig,
	)
	require.NoError(t, err)

	// The attacker signs a normal delegate transaction to the fake validator address.
	// The delegate tx names the fake contract, but the decoded wrapper names the real validator.
	delegateConsensus := fakeWrapperConsensus(t, chain, leaderSigner, chainConfig)
	require.NoError(t, delegateConsensus.registry.GetTxPool().AddLocal(delegateTx))
	delegateBlock := fakeWrapperProposeAndInsertBlock(t, delegateConsensus, chain, leaderSigner, deployBlock.GetCurrentCommitSig(), chainConfig)
	require.Equal(t, setupBlockNum+2, delegateBlock.NumberU64())
	require.Empty(t, delegateBlock.Transactions())
	require.Len(t, delegateBlock.StakingTransactions(), 1)

	afterDelegateState, err := chain.StateAt(delegateBlock.Root())
	require.NoError(t, err)
	overwritten, err := afterDelegateState.ValidatorWrapper(validator, false, true)
	require.NoError(t, err)

	// The delegate path has written the contract-supplied wrapper into the real validator account.
	require.Equal(t, "fake-wrapper", overwritten.Name)
	require.Zero(t, overwritten.Delegations[0].Amount.Cmp(fakeWrapperOnes(10_000)))
	require.Zero(t, overwritten.Delegations[1].Amount.Cmp(minimumDelegation))
	require.Len(t, overwritten.Delegations[1].Undelegations, 1)
	require.Zero(t, overwritten.Delegations[1].Undelegations[0].Amount.Cmp(forgedPayout))

	balanceBeforePayout := new(big.Int).Set(afterDelegateState.GetBalance(attacker))

	// Empty blocks carry the chain to the epoch-ending payout point without changing the forged wrapper.
	payoutConsensus := fakeWrapperConsensus(t, chain, leaderSigner, chainConfig)
	parentCommitSig := delegateBlock.GetCurrentCommitSig()
	var payoutBlock *coretypes.Block
	for chain.CurrentBlock().NumberU64() < payoutBlockNum {
		payoutBlock = fakeWrapperProposeAndInsertBlock(t, payoutConsensus, chain, leaderSigner, parentCommitSig, chainConfig)
		parentCommitSig = payoutBlock.GetCurrentCommitSig()
	}

	// The epoch-ending block iterates the validator list and loads the overwritten wrapper from the real validator.
	require.Equal(t, payoutBlockNum, payoutBlock.NumberU64())
	require.Empty(t, payoutBlock.Transactions())
	require.Empty(t, payoutBlock.StakingTransactions())
	require.NotEmpty(t, payoutBlock.Header().ShardState())

	finalState, err := chain.StateAt(payoutBlock.Root())
	require.NoError(t, err)
	finalWrapper, err := finalState.ValidatorWrapper(validator, false, true)
	require.NoError(t, err)

	// The forged undelegation is paid out even though the attacker only added the 100 ONE delegation.
	require.Zero(t, new(big.Int).Sub(finalState.GetBalance(attacker), balanceBeforePayout).Cmp(forgedPayout))
	require.Zero(t, finalWrapper.Delegations[1].Amount.Cmp(minimumDelegation))
	require.Empty(t, finalWrapper.Delegations[1].Undelegations)
}

func TestFakeValidatorWrapperBlockedByAddressBindFork(t *testing.T) {
	previousSchedule := shard.Schedule
	previousNetwork := nodeconfig.GetDefaultConfig().GetNetworkType()
	shard.Schedule = shardingconfig.MainnetSchedule
	nodeconfig.SetNetworkType(nodeconfig.Mainnet)
	t.Cleanup(func() {
		shard.Schedule = previousSchedule
		nodeconfig.SetNetworkType(previousNetwork)
	})

	attackerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainConfig := fakeWrapperChainConfig(big.NewInt(0))
	attackEpoch := new(big.Int).Add(chainConfig.HIP32Epoch, big.NewInt(8))
	unlockEpoch := new(big.Int).Sub(attackEpoch, big.NewInt(staking.LockPeriodInEpoch))
	leaderSigner := hmybls.WrapperFromPrivateKey(hmybls.RandPrivateKey())
	validator := common.HexToAddress("0x0000000000000000000000000000000000000a71")
	attacker := crypto.PubkeyToAddress(attackerKey.PublicKey)
	leaderCoinbase := fakeWrapperCoinbase(leaderSigner)

	committeeState := fakeWrapperShardState(attackEpoch, validator, leaderSigner.Pub.Bytes)
	alloc := corepkg.GenesisAlloc{
		attacker: {
			Balance: fakeWrapperOnes(2_000_000),
		},
	}
	chain, setupState, genesis := fakeWrapperChain(t, alloc, committeeState, chainConfig)
	processor := corepkg.NewStateProcessor(chain, chain)

	realWrapper := fakeWrapperRealValidator(validator, leaderSigner.Pub.Bytes, attackEpoch)
	setupState.SetValidatorFlag(validator)
	require.NoError(t, setupState.UpdateValidatorWrapper(validator, realWrapper))
	require.NoError(t, rawdb.WriteValidatorList(setupState.Database().DiskDB(), []common.Address{validator}))
	snapshot := staketest.CopyValidatorWrapper(*realWrapper)
	require.NoError(t, rawdb.WriteValidatorSnapshot(setupState.Database().DiskDB(), &snapshot, attackEpoch))

	payoutBlockNum := shardingconfig.MainnetSchedule.EpochLastBlock(attackEpoch.Uint64())
	setupBlockNum := payoutBlockNum - shardingconfig.RewardFrequency + 1
	setupHeader := fakeWrapperHeader(attackEpoch, setupBlockNum, leaderCoinbase, genesis.Hash())
	setupBlock, err := fakeWrapperProcessAndCommitBlock(
		t, chain, processor, setupState, setupHeader, nil, nil, leaderSigner, chainConfig,
	)
	require.NoError(t, err)

	forgedPayout := fakeWrapperOnes(1_000_000)
	minimumDelegation := fakeWrapperOnes(100)
	fakeRuntime := fakeWrapperRuntime(
		t, validator, attacker, leaderSigner.Pub.Bytes,
		minimumDelegation, forgedPayout, unlockEpoch, attackEpoch,
	)
	fakeInitcode := fakeWrapperValidatorInitcode(fakeRuntime)
	fakeContract := crypto.CreateAddress(attacker, 0)
	gasPrice := big.NewInt(100e9)

	deployTx, err := fakeWrapperSignContractCreationTx(
		attackerKey, 0, common.Big0, 5_000_000, gasPrice, fakeInitcode, attackEpoch, chainConfig,
	)
	require.NoError(t, err)

	deployConsensus := fakeWrapperConsensus(t, chain, leaderSigner, chainConfig)
	require.NoError(t, deployConsensus.registry.GetTxPool().AddLocal(deployTx))
	deployBlock := fakeWrapperProposeAndInsertBlock(
		t, deployConsensus, chain, leaderSigner, setupBlock.GetCurrentCommitSig(), chainConfig,
	)
	deployedState, err := chain.StateAt(deployBlock.Root())
	require.NoError(t, err)
	require.True(t, deployedState.IsValidator(fakeContract))
	deployedState.SetValidatorWrapperAddressBind(
		chainConfig.IsValidatorWrapperAddressBind(attackEpoch),
	)
	_, err = deployedState.ValidatorWrapper(fakeContract, false, true)
	require.ErrorIs(t, err, staking.ErrValidatorWrapperAddressMismatch)

	delegateTx, err := fakeWrapperSignDelegateTx(
		attackerKey, 1, fakeContract, attacker, minimumDelegation, 1_000_000, gasPrice, chainConfig,
	)
	require.NoError(t, err)

	delegateConsensus := fakeWrapperConsensus(t, chain, leaderSigner, chainConfig)
	// Pool validation runs VerifyAndDelegateFromMsg before the tx can be included.
	err = delegateConsensus.registry.GetTxPool().AddLocal(delegateTx)
	require.ErrorIs(t, err, staking.ErrValidatorWrapperAddressMismatch)

	afterDelegateState, err := chain.StateAt(deployBlock.Root())
	require.NoError(t, err)
	afterDelegateState.SetValidatorWrapperAddressBind(
		chainConfig.IsValidatorWrapperAddressBind(attackEpoch),
	)
	unchanged, err := afterDelegateState.ValidatorWrapper(validator, false, true)
	require.NoError(t, err)
	require.Equal(t, "real-validator", unchanged.Name)
	require.Len(t, unchanged.Delegations, 1)
}

func fakeWrapperRuntime(
	t *testing.T,
	validator common.Address,
	attacker common.Address,
	blsKey hmybls.SerializedPublicKey,
	minimumDelegation *big.Int,
	forgedPayout *big.Int,
	unlockEpoch *big.Int,
	attackEpoch *big.Int,
) []byte {
	t.Helper()

	wrapper := &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              validator,
			SlotPubKeys:          []hmybls.SerializedPublicKey{blsKey},
			LastEpochInCommittee: new(big.Int).Set(attackEpoch),
			MinSelfDelegation:    fakeWrapperOnes(10_000),
			MaxTotalDelegation:   fakeWrapperOnes(20_000),
			Status:               effective.Active,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					Rate:          numeric.ZeroDec(),
					MaxRate:       numeric.OneDec(),
					MaxChangeRate: numeric.OneDec(),
				},
				UpdateHeight: common.Big0,
			},
			Description: staking.Description{
				Name:            "fake-wrapper",
				Identity:        "fake-wrapper",
				Website:         "fake-wrapper",
				SecurityContact: "fake-wrapper",
				Details:         "fake-wrapper",
			},
			CreationHeight: common.Big0,
		},
		Delegations: staking.Delegations{
			staking.NewDelegation(validator, fakeWrapperOnes(10_000)),
			{
				DelegatorAddress: attacker,
				Amount:           common.Big0,
				Reward:           common.Big0,
				Undelegations: staking.Undelegations{
					{
						Amount: new(big.Int).Set(forgedPayout),
						Epoch:  new(big.Int).Set(unlockEpoch),
					},
				},
			},
		},
		BlockReward: common.Big0,
	}
	wrapper.Counters.NumBlocksToSign = common.Big0
	wrapper.Counters.NumBlocksSigned = common.Big0
	require.NoError(t, wrapper.SanityCheck())

	encoded, err := rlp.EncodeToBytes(wrapper)
	require.NoError(t, err)
	return encoded
}

func fakeWrapperValidatorInitcode(runtime []byte) []byte {
	code := make([]byte, 0, 82+len(runtime))
	code = append(code, 0x7f)
	code = append(code, stakingRoot.IsValidator.Bytes()...)
	code = append(code, 0x7f)
	code = append(code, stakingRoot.IsValidatorKey.Bytes()...)
	code = append(code, 0x55)
	code = appendPush2(code, uint16(len(runtime)))
	code = appendPush2(code, 82)
	code = append(code, 0x60, 0x00, 0x39)
	code = appendPush2(code, uint16(len(runtime)))
	code = append(code, 0x60, 0x00, 0xf3)
	return append(code, runtime...)
}

func appendPush2(code []byte, value uint16) []byte {
	return append(code, 0x61, byte(value>>8), byte(value))
}

func fakeWrapperRealValidator(
	validator common.Address,
	blsKey hmybls.SerializedPublicKey,
	epoch *big.Int,
) *staking.ValidatorWrapper {
	wrapper := staketest.GetDefaultValidatorWrapperWithAddr(validator, []hmybls.SerializedPublicKey{blsKey})
	wrapper.Name = "real-validator"
	wrapper.Identity = "real-validator"
	wrapper.Website = "real-validator"
	wrapper.SecurityContact = "real-validator"
	wrapper.Details = "real-validator"
	wrapper.LastEpochInCommittee = new(big.Int).Set(epoch)
	wrapper.MinSelfDelegation = fakeWrapperOnes(10_000)
	wrapper.MaxTotalDelegation = fakeWrapperOnes(1_000_000)
	wrapper.Delegations[0].Amount = fakeWrapperOnes(100_000)
	wrapper.CommissionRates.Rate = numeric.ZeroDec()
	wrapper.CommissionRates.MaxRate = numeric.OneDec()
	wrapper.CommissionRates.MaxChangeRate = numeric.OneDec()
	return &wrapper
}

func fakeWrapperSignContractCreationTx(
	key *ecdsa.PrivateKey,
	nonce uint64,
	value *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte,
	epoch *big.Int,
	chainConfig *params.ChainConfig,
) (*coretypes.Transaction, error) {
	tx := coretypes.NewContractCreation(nonce, shard.BeaconChainShardID, value, gasLimit, gasPrice, data)
	return coretypes.SignTx(tx, coretypes.MakeSigner(chainConfig, epoch), key)
}

func fakeWrapperSignDelegateTx(
	key *ecdsa.PrivateKey,
	nonce uint64,
	validator common.Address,
	delegator common.Address,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	chainConfig *params.ChainConfig,
) (*staking.StakingTransaction, error) {
	payload := func() (staking.Directive, interface{}) {
		return staking.DirectiveDelegate, &staking.Delegate{
			ValidatorAddress: validator,
			DelegatorAddress: delegator,
			Amount:           new(big.Int).Set(amount),
		}
	}
	tx, err := staking.NewStakingTransaction(nonce, gasLimit, gasPrice, payload)
	if err != nil {
		return nil, err
	}
	return staking.Sign(tx, staking.NewEIP155Signer(chainConfig.ChainID), key)
}

func fakeWrapperConsensus(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	signer hmybls.PrivateKeyWrapper,
	chainConfig *params.ChainConfig,
) *Consensus {
	t.Helper()

	txPoolConfig := corepkg.DefaultTxPoolConfig
	txPoolConfig.Journal = ""
	txPool := corepkg.NewTxPool(
		txPoolConfig,
		chainConfig,
		chain,
		coretypes.NewTransactionErrorSink(),
	)
	t.Cleanup(txPool.Stop)

	reg := registry.New().
		SetBlockchain(chain).
		SetBeaconchain(chain).
		SetTxPool(txPool).
		SetCxPool(corepkg.NewCxPool(corepkg.CxPoolSize)).
		SetWorker(workerpkg.New(chain, chain)).
		SetAddressToBLSKey(fakeWrapperAddressToBLSKey{})

	consensus, err := New(
		nil,
		chain.ShardID(),
		multibls.GetPrivateKeys(signer.Pri),
		reg,
		quorum.NewDecider(quorum.SuperMajorityStake, chain.ShardID()),
		1,
		false,
	)
	require.NoError(t, err)

	consensus.SetLeaderPubKey(signer.Pub)
	consensus.SetViewIDs(chain.CurrentBlock().NumberU64())
	return consensus
}

type fakeWrapperAddressToBLSKey struct{}

func (fakeWrapperAddressToBLSKey) GetAddressForBLSKey(
	publicKeys multibls.PublicKeys,
	shardState registry.FindCommitteeByID,
	blskey *bls_core.PublicKey,
	epoch *big.Int,
) common.Address {
	return fakeWrapperAddressToBLSKey{}.GetAddresses(publicKeys, shardState, epoch)[blskey.SerializeToHexStr()]
}

func (fakeWrapperAddressToBLSKey) GetAddresses(
	publicKeys multibls.PublicKeys,
	shardState registry.FindCommitteeByID,
	_ *big.Int,
) map[string]common.Address {
	addresses := map[string]common.Address{}
	committee, err := shardState.FindCommitteeByID(shard.BeaconChainShardID)
	if err != nil {
		return addresses
	}
	for _, publicKey := range publicKeys {
		shardKey := hmybls.FromLibBLSPublicKeyUnsafe(publicKey.Object)
		if shardKey == nil {
			continue
		}
		addr, err := committee.AddressForBLSKey(*shardKey)
		if err != nil {
			continue
		}
		addresses[publicKey.Bytes.Hex()] = *addr
	}
	return addresses
}

func fakeWrapperProposeAndInsertBlock(
	t *testing.T,
	consensus *Consensus,
	chain *corepkg.BlockChainImpl,
	signer hmybls.PrivateKeyWrapper,
	parentCommitSig []byte,
	chainConfig *params.ChainConfig,
) *coretypes.Block {
	t.Helper()
	block, err := fakeWrapperProposeAndInsertBlockAllowError(
		t, consensus, chain, signer, parentCommitSig, chainConfig,
	)
	require.NoError(t, err)
	return block
}

func fakeWrapperProposeAndInsertBlockAllowError(
	t *testing.T,
	consensus *Consensus,
	chain *corepkg.BlockChainImpl,
	signer hmybls.PrivateKeyWrapper,
	parentCommitSig []byte,
	chainConfig *params.ChainConfig,
) (*coretypes.Block, error) {
	t.Helper()

	consensus.SetViewIDs(chain.CurrentBlock().NumberU64())
	commitSigs := make(chan []byte, 1)
	commitSigs <- parentCommitSig

	block, err := consensus.ProposeNewBlock(time.Now(), commitSigs)
	if err != nil {
		return nil, err
	}
	block.SetCurrentCommitSig(fakeWrapperCommitSig(t, block, signer, chainConfig))
	if err := consensus.verifyBlock(block); err != nil {
		return nil, err
	}

	_, err = chain.InsertChain([]*coretypes.Block{block}, true)
	if err != nil {
		return nil, err
	}
	require.Equal(t, block.Hash(), chain.CurrentBlock().Hash())
	return block, nil
}

func fakeWrapperProcessAndCommitBlock(
	t *testing.T,
	chain *corepkg.BlockChainImpl,
	processor *corepkg.StateProcessor,
	statedb *state.DB,
	header *block.Header,
	txs []*coretypes.Transaction,
	stxs []*staking.StakingTransaction,
	signer hmybls.PrivateKeyWrapper,
	chainConfig *params.ChainConfig,
) (*coretypes.Block, error) {
	t.Helper()

	inputBlock := coretypes.NewBlockWithHeader(header).WithBody(txs, stxs, nil, nil)
	receipts, outcxs, stakeMsgs, _, usedGas, payout, _, err := processor.Process(
		inputBlock,
		statedb,
		vm.Config{},
		false,
	)
	if err != nil {
		return nil, err
	}

	header.SetGasUsed(usedGas)
	header.SetRoot(statedb.IntermediateRoot(chainConfig.IsS3(header.Epoch())))
	finalBlock := coretypes.NewBlock(header, txs, receipts, outcxs, nil, stxs)
	finalBlock.SetCurrentCommitSig(fakeWrapperCommitSig(t, finalBlock, signer, chainConfig))

	status, err := chain.WriteBlockWithState(finalBlock, receipts, outcxs, stakeMsgs, payout, statedb)
	require.NoError(t, err)
	require.Equal(t, corepkg.CanonStatTy, status)
	return finalBlock, nil
}

func fakeWrapperCommitSig(
	t *testing.T,
	block *coretypes.Block,
	signer hmybls.PrivateKeyWrapper,
	chainConfig *params.ChainConfig,
) []byte {
	t.Helper()

	mask := hmybls.NewMask([]hmybls.PublicKeyWrapper{*signer.Pub})
	require.NoError(t, mask.SetBit(0, true))

	payload := consensus_sig.ConstructCommitPayload(
		chainConfig,
		block.Epoch(),
		block.Hash(),
		block.NumberU64(),
		block.Header().ViewID().Uint64(),
	)
	sig := signer.Pri.SignHash(payload)
	return append(sig.Serialize(), mask.Mask()...)
}

func fakeWrapperChain(
	t *testing.T,
	alloc corepkg.GenesisAlloc,
	attackState shard.State,
	chainConfig *params.ChainConfig,
) (*corepkg.BlockChainImpl, *state.DB, *coretypes.Block) {
	t.Helper()

	genesisState := attackState
	genesisState.Epoch = common.Big0

	db := rawdb.NewMemoryDatabase()
	gspec := corepkg.Genesis{
		Config:     chainConfig,
		Factory:    blockfactory.ForMainnet,
		Alloc:      alloc,
		ShardID:    shard.BeaconChainShardID,
		GasLimit:   params.TestGenesisGasLimit,
		ShardState: genesisState,
	}
	genesis := gspec.MustCommit(db)

	encodedState, err := shard.EncodeWrapper(attackState, true)
	require.NoError(t, err)
	require.NoError(t, rawdb.WriteShardStateBytes(db, attackState.Epoch, encodedState))

	chain, err := corepkg.NewBlockChain(
		db,
		nil,
		nil,
		&corepkg.CacheConfig{SnapshotLimit: 0},
		gspec.Config,
		chain2.NewEngine(),
		vm.Config{},
	)
	require.NoError(t, err)

	statedb, err := chain.StateAt(genesis.Root())
	require.NoError(t, err)
	return chain, statedb, genesis
}

func fakeWrapperShardState(
	epoch *big.Int,
	validator common.Address,
	blsKey hmybls.SerializedPublicKey,
) shard.State {
	stake := numeric.OneDec()
	return shard.State{
		Epoch: new(big.Int).Set(epoch),
		Shards: []shard.Committee{
			{
				ShardID: shard.BeaconChainShardID,
				Slots: shard.SlotList{
					{
						EcdsaAddress:   validator,
						BLSPublicKey:   blsKey,
						EffectiveStake: &stake,
					},
				},
			},
		},
	}
}

func fakeWrapperHeader(
	epoch *big.Int,
	number uint64,
	coinbase common.Address,
	parentHash common.Hash,
) *block.Header {
	return blockfactory.ForMainnet.NewHeader(epoch).With().
		Number(new(big.Int).SetUint64(number)).
		Epoch(epoch).
		ShardID(shard.BeaconChainShardID).
		ViewID(common.Big1).
		Coinbase(coinbase).
		ParentHash(parentHash).
		GasLimit(params.TestGenesisGasLimit).
		Time(new(big.Int).SetUint64(number * 2)).
		Header()
}

func fakeWrapperCoinbase(signer hmybls.PrivateKeyWrapper) common.Address {
	coinbase := common.Address{}
	blsPubKeyBytes := signer.Pub.Object.GetAddress()
	coinbase.SetBytes(blsPubKeyBytes[:])
	return coinbase
}

func fakeWrapperOnes(amount int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(amount), big.NewInt(denominations.One))
}
