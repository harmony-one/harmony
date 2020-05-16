package chain

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

type engineImpl struct {
	beacon engine.ChainReader
}

// Engine is an algorithm-agnostic consensus engine.
var Engine = &engineImpl{nil}

func (e *engineImpl) Beaconchain() engine.ChainReader {
	return e.beacon
}

// SetBeaconchain assigns the beaconchain handle used
func (e *engineImpl) SetBeaconchain(beaconchain engine.ChainReader) {
	e.beacon = beaconchain
}

// SealHash returns the hash of a block prior to it being sealed.
func (e *engineImpl) SealHash(header *block.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	// TODO: update with new fields
	if err := rlp.Encode(hasher, []interface{}{
		header.ParentHash(),
		header.Coinbase(),
		header.Root(),
		header.TxHash(),
		header.ReceiptHash(),
		header.Bloom(),
		header.Number(),
		header.GasLimit(),
		header.GasUsed(),
		header.Time(),
		header.Extra(),
	}); err != nil {
		utils.Logger().Warn().Err(err).Msg("rlp.Encode failed")
	}
	hasher.Sum(hash[:0])
	return hash
}

// Seal is to seal final block.
func (e *engineImpl) Seal(chain engine.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	// TODO: implement final block sealing
	return nil
}

// Author returns the author of the block header.
func (e *engineImpl) Author(header *block.Header) (common.Address, error) {
	// TODO: implement this
	return common.Address{}, nil
}

// Prepare is to prepare ...
// TODO(RJ): fix it.
func (e *engineImpl) Prepare(chain engine.ChainReader, header *block.Header) error {
	// TODO: implement prepare method
	return nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of the bft engine.
// Note that each block header contains the bls signature of the parent block
func (e *engineImpl) VerifyHeader(chain engine.ChainReader, header *block.Header, seal bool) error {
	parentHeader := chain.GetHeader(header.ParentHash(), header.Number().Uint64()-1)
	if parentHeader == nil {
		return engine.ErrUnknownAncestor
	}
	if seal {
		if err := e.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (e *engineImpl) VerifyHeaders(chain engine.ChainReader, headers []*block.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := e.VerifyHeader(chain, header, seals[i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()

	return abort, results
}

// ReadPublicKeysFromLastBlock finds the public keys of last block's committee
func ReadPublicKeysFromLastBlock(
	bc engine.ChainReader, header *block.Header,
) ([]*bls.PublicKey, error) {
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	return GetPublicKeys(bc, parentHeader, false)
}

// VerifyShardState implements Engine, checking the shardstate is valid at epoch transition
func (e *engineImpl) VerifyShardState(
	bc engine.ChainReader, beacon engine.ChainReader, header *block.Header,
) error {
	if bc.ShardID() != header.ShardID() {
		return errors.Errorf(
			"[VerifyShardState] shardID not match %d %d", bc.ShardID(), header.ShardID(),
		)
	}
	headerShardStateBytes := header.ShardState()
	// TODO: figure out leader withhold shardState
	if len(headerShardStateBytes) == 0 {
		return nil
	}
	shardState, err := bc.SuperCommitteeForNextEpoch(beacon, header, true)
	if err != nil {
		return err
	}

	isStaking := false
	if shardState.Epoch != nil && bc.Config().IsStaking(shardState.Epoch) {
		isStaking = true
	}
	shardStateBytes, err := shard.EncodeWrapper(*shardState, isStaking)
	if err != nil {
		return errors.Wrapf(
			err, "[VerifyShardState] ShardState Encoding had error",
		)
	}

	if !bytes.Equal(shardStateBytes, headerShardStateBytes) {
		return errors.New("shard state header did not match as expected")
	}

	return nil
}

// VerifySeal implements Engine, checking whether the given block's parent block satisfies
// the PoS difficulty requirements, i.e. >= 2f+1 valid signatures from the committee
// Note that each block header contains the bls signature of the parent block
func (e *engineImpl) VerifySeal(chain engine.ChainReader, header *block.Header) error {
	if chain.CurrentHeader().Number().Uint64() <= uint64(1) {
		return nil
	}
	publicKeys, err := ReadPublicKeysFromLastBlock(chain, header)

	if err != nil {
		return errors.New("[VerifySeal] Cannot retrieve publickeys from last block")
	}
	sig := header.LastCommitSignature()
	payload := append(sig[:], header.LastCommitBitmap()...)
	aggSig, mask, err := ReadSignatureBitmapByPublicKeys(payload, publicKeys)
	if err != nil {
		return errors.New(
			"[VerifySeal] Unable to deserialize the LastCommitSignature" +
				" and LastCommitBitmap in Block Header",
		)
	}
	parentHash := header.ParentHash()
	parentHeader := chain.GetHeader(parentHash, header.Number().Uint64()-1)
	if chain.Config().IsStaking(parentHeader.Epoch()) {
		slotList, err := chain.ReadShardState(parentHeader.Epoch())
		if err != nil {
			return errors.Wrapf(err, "cannot decoded shard state")
		}
		subComm, err := slotList.FindCommitteeByID(parentHeader.ShardID())
		if err != nil {
			return err
		}
		// TODO(audit): reuse a singleton decider and not recreate it for every single block
		d := quorum.NewDecider(
			quorum.SuperMajorityStake, subComm.ShardID,
		)
		d.SetMyPublicKeyProvider(func() (*multibls.PublicKey, error) {
			return nil, nil
		})

		if _, err := d.SetVoters(subComm, slotList.Epoch); err != nil {
			return err
		}
		if !d.IsQuorumAchievedByMask(mask) {
			return errors.New(
				"[VerifySeal] Not enough voting power in LastCommitSignature from Block Header",
			)
		}
	} else {
		parentQuorum, err := QuorumForBlock(chain, parentHeader, false)
		if err != nil {
			return errors.Wrapf(err,
				"cannot calculate quorum for block %s", header.Number())
		}
		if count := utils.CountOneBits(mask.Bitmap); count < int64(parentQuorum) {
			return errors.Errorf(
				"[VerifySeal] need %d signature in LastCommitSignature have %d",
				parentQuorum, count,
			)
		}
	}

	lastCommitPayload := signature.ConstructCommitPayload(chain,
		parentHeader.Epoch(), parentHeader.Hash(), parentHeader.Number().Uint64(), parentHeader.ViewID().Uint64())
	if !aggSig.VerifyHash(mask.AggregatePublic, lastCommitPayload) {
		const msg = "[VerifySeal] Unable to verify aggregated signature from last block"
		return errors.New(msg)
	}
	return nil
}

// Finalize implements Engine, accumulating the block rewards,
// setting the final state and assembling the block.
func (e *engineImpl) Finalize(
	chain engine.ChainReader, header *block.Header,
	state *state.DB, txs []*types.Transaction,
	receipts []*types.Receipt, outcxs []*types.CXReceipt,
	incxs []*types.CXReceiptsProof, stks staking.StakingTransactions,
	doubleSigners slash.Records,
) (*types.Block, reward.Reader, error) {

	isBeaconChain := header.ShardID() == shard.BeaconChainShardID
	isNewEpoch := len(header.ShardState()) > 0
	inPreStakingEra := chain.Config().IsPreStaking(header.Epoch())
	inStakingEra := chain.Config().IsStaking(header.Epoch())

	// Process Undelegations, set LastEpochInCommittee and set EPoS status
	// Needs to be before AccumulateRewardsAndCountSigs
	if isBeaconChain && isNewEpoch && inPreStakingEra {
		if err := payoutUndelegations(chain, header, state); err != nil {
			return nil, nil, err
		}

		// Needs to be after payoutUndelegations because payoutUndelegations
		// depends on the old LastEpochInCommittee
		if err := setLastEpochInCommittee(header, state); err != nil {
			return nil, nil, err
		}

		curShardState, err := chain.ReadShardState(chain.CurrentBlock().Epoch())
		if err != nil {
			return nil, nil, err
		}
		// Needs to be before AccumulateRewardsAndCountSigs because
		// ComputeAndMutateEPOSStatus depends on the signing counts that's
		// consistent with the counts when the new shardState was proposed.
		// Refer to committee.IsEligibleForEPoSAuction()
		for _, addr := range curShardState.StakedValidators().Addrs {
			if err := availability.ComputeAndMutateEPOSStatus(
				chain, state, addr,
			); err != nil {
				return nil, nil, err
			}
		}
	}

	// Accumulate block rewards and commit the final state root
	// Header seems complete, assemble into a block and return
	payout, err := AccumulateRewardsAndCountSigs(
		chain, state, header, e.Beaconchain(),
	)
	if err != nil {
		return nil, nil, errors.New("cannot pay block reward")
	}

	// Apply slashes
	if isBeaconChain && inStakingEra && len(doubleSigners) > 0 {
		if err := applySlashes(chain, header, state, doubleSigners); err != nil {
			return nil, nil, err
		}
	} else if len(doubleSigners) > 0 {
		return nil, nil, errors.New("slashes proposed in non-beacon chain or non-staking epoch")
	}

	// Finalize the state root
	header.SetRoot(state.IntermediateRoot(chain.Config().IsS3(header.Epoch())))
	return types.NewBlock(header, txs, receipts, outcxs, incxs, stks), payout, nil
}

// Withdraw unlocked tokens to the delegators' accounts
func payoutUndelegations(
	chain engine.ChainReader, header *block.Header, state *state.DB,
) error {
	currentHeader := chain.CurrentHeader()
	nowEpoch, blockNow := currentHeader.Epoch(), currentHeader.Number()
	utils.AnalysisStart("payoutUndelegations", nowEpoch, blockNow)
	defer utils.AnalysisEnd("payoutUndelegations", nowEpoch, blockNow)

	validators, err := chain.ReadValidatorList()
	countTrack := map[common.Address]int{}
	if err != nil {
		const msg = "[Finalize] failed to read all validators"
		return errors.New(msg)
	}
	// Payout undelegated/unlocked tokens
	for _, validator := range validators {
		wrapper, err := state.ValidatorWrapper(validator)
		if err != nil {
			return errors.New(
				"[Finalize] failed to get validator from state to finalize",
			)
		}
		for i := range wrapper.Delegations {
			delegation := &wrapper.Delegations[i]
			totalWithdraw := delegation.RemoveUnlockedUndelegations(
				header.Epoch(), wrapper.LastEpochInCommittee,
			)
			state.AddBalance(delegation.DelegatorAddress, totalWithdraw)
		}
		countTrack[validator] = len(wrapper.Delegations)
	}

	utils.Logger().Info().
		Uint64("epoch", header.Epoch().Uint64()).
		Uint64("block-number", header.Number().Uint64()).
		Interface("count-track", countTrack).
		Msg("paid out delegations")

	return nil
}

func setLastEpochInCommittee(header *block.Header, state *state.DB) error {
	newShardState, err := header.GetShardState()
	if err != nil {
		const msg = "[Finalize] failed to read shard state"
		return errors.New(msg)
	}
	for _, addr := range newShardState.StakedValidators().Addrs {
		wrapper, err := state.ValidatorWrapper(addr)
		if err != nil {
			return errors.New(
				"[Finalize] failed to get validator from state to finalize",
			)
		}
		wrapper.LastEpochInCommittee = newShardState.Epoch
	}
	return nil
}

func applySlashes(
	chain engine.ChainReader,
	header *block.Header,
	state *state.DB,
	doubleSigners slash.Records,
) error {
	type keyStruct struct {
		height  uint64
		viewID  uint64
		shardID uint32
		epoch   uint64
	}

	groupedRecords := map[keyStruct]slash.Records{}

	// First group slashes by same signed blocks
	for i := range doubleSigners {
		thisKey := keyStruct{
			height:  doubleSigners[i].Evidence.Height,
			viewID:  doubleSigners[i].Evidence.ViewID,
			shardID: doubleSigners[i].Evidence.Moment.ShardID,
			epoch:   doubleSigners[i].Evidence.Moment.Epoch.Uint64(),
		}

		if _, ok := groupedRecords[thisKey]; ok {
			groupedRecords[thisKey] = append(groupedRecords[thisKey], doubleSigners[i])
		} else {
			groupedRecords[thisKey] = slash.Records{doubleSigners[i]}
		}
	}

	sortedKeys := []keyStruct{}

	for key := range groupedRecords {
		sortedKeys = append(sortedKeys, key)
	}

	// Sort them so the slashes are always consistent
	sort.SliceStable(sortedKeys, func(i, j int) bool {
		if sortedKeys[i].shardID < sortedKeys[j].shardID {
			return true
		} else if sortedKeys[i].height < sortedKeys[j].height {
			return true
		} else if sortedKeys[i].viewID < sortedKeys[j].viewID {
			return true
		}
		return false
	})

	// Do the slashing by groups in the sorted order
	for _, key := range sortedKeys {
		records := groupedRecords[key]
		superCommittee, err := chain.ReadShardState(big.NewInt(int64(key.epoch)))

		if err != nil {
			return errors.New("could not read shard state")
		}

		subComm, err := superCommittee.FindCommitteeByID(key.shardID)

		if err != nil {
			return errors.New("could not find shard committee")
		}

		// Apply the slashes, invariant: assume been verified as legit slash by this point
		var slashApplied *slash.Application
		votingPower, err := lookupVotingPower(
			big.NewInt(int64(key.epoch)), subComm,
		)
		if err != nil {
			return errors.Wrapf(err, "could not lookup cached voting power in slash application")
		}
		rate := slash.Rate(votingPower, records)
		utils.Logger().Info().
			Str("rate", rate.String()).
			RawJSON("records", []byte(records.String())).
			Msg("now applying slash to state during block finalization")
		if slashApplied, err = slash.Apply(
			chain,
			state,
			records,
			rate,
		); err != nil {
			return errors.New("[Finalize] could not apply slash")
		}

		utils.Logger().Info().
			Str("rate", rate.String()).
			RawJSON("records", []byte(records.String())).
			RawJSON("applied", []byte(slashApplied.String())).
			Msg("slash applied successfully")
	}
	return nil
}

// QuorumForBlock returns the quorum for the given block header.
func QuorumForBlock(
	chain engine.ChainReader, h *block.Header, reCalculate bool,
) (quorum int, err error) {
	ss := new(shard.State)
	if reCalculate {
		ss, _ = committee.WithStakingEnabled.Compute(h.Epoch(), chain)
	} else {
		ss, err = chain.ReadShardState(h.Epoch())
		if err != nil {
			return 0, errors.Wrapf(
				err, "failed to read shard state of epoch %d", h.Epoch().Uint64(),
			)
		}
	}

	subComm, err := ss.FindCommitteeByID(h.ShardID())
	if err != nil {
		return 0, errors.Errorf("cannot find shard %d in shard state", h.ShardID())
	}
	return (len(subComm.Slots))*2/3 + 1, nil
}

// Similiar to VerifyHeader, which is only for verifying the block headers of one's own chain, this verification
// is used for verifying "incoming" block header against commit signature and bitmap sent from the other chain cross-shard via libp2p.
// i.e. this header verification api is more flexible since the caller specifies which commit signature and bitmap to use
// for verifying the block header, which is necessary for cross-shard block header verification. Example of such is cross-shard transaction.
func (e *engineImpl) VerifyHeaderWithSignature(chain engine.ChainReader, header *block.Header, commitSig []byte, commitBitmap []byte, reCalculate bool) error {
	if chain.Config().IsStaking(header.Epoch()) {
		// Never recalculate after staking is enabled
		reCalculate = false
	}
	publicKeys, err := GetPublicKeys(chain, header, reCalculate)
	if err != nil {
		return errors.New("[VerifyHeaderWithSignature] Cannot get publickeys for block header")
	}

	payload := append(commitSig[:], commitBitmap[:]...)
	aggSig, mask, err := ReadSignatureBitmapByPublicKeys(payload, publicKeys)
	if err != nil {
		return errors.Wrapf(
			err,
			"[VerifyHeaderWithSignature] Unable to deserialize signatures",
		)
	}

	if e := header.Epoch(); chain.Config().IsStaking(e) {
		slotList, err := chain.ReadShardState(e)
		if err != nil {
			return errors.Wrapf(err, "cannot read shard state")
		}

		subComm, err := slotList.FindCommitteeByID(header.ShardID())
		if err != nil {
			return err
		}
		// TODO(audit): reuse a singleton decider and not recreate it for every single block
		d := quorum.NewDecider(quorum.SuperMajorityStake, subComm.ShardID)
		d.SetMyPublicKeyProvider(func() (*multibls.PublicKey, error) {
			return nil, nil
		})

		if _, err := d.SetVoters(subComm, e); err != nil {
			return err
		}
		if !d.IsQuorumAchievedByMask(mask) {
			return errors.New(
				"[VerifySeal] Not enough voting power in commitSignature from Block Header",
			)
		}
	} else {
		quorumCount, err := QuorumForBlock(chain, header, reCalculate)
		if err != nil {
			return errors.Wrapf(err,
				"cannot calculate quorum for block %s", header.Number())
		}
		if count := utils.CountOneBits(mask.Bitmap); count < int64(quorumCount) {
			return errors.New(
				"[VerifyHeaderWithSignature] Not enough signature in commitSignature from Block Header",
			)
		}
	}
	commitPayload := signature.ConstructCommitPayload(chain,
		header.Epoch(), header.Hash(), header.Number().Uint64(), header.ViewID().Uint64())

	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return errors.New("[VerifySeal] Unable to verify aggregated signature for block")
	}
	return nil
}

// GetPublicKeys finds the public keys of the committee that signed the block header
func GetPublicKeys(
	chain engine.ChainReader, header *block.Header, reCalculate bool,
) ([]*bls.PublicKey, error) {
	shardState := new(shard.State)
	var err error
	if reCalculate {
		shardState, _ = committee.WithStakingEnabled.Compute(header.Epoch(), chain)
	} else {
		shardState, err = chain.ReadShardState(header.Epoch())
		if err != nil {
			return nil, errors.Wrapf(
				err, "failed to read shard state of epoch %d", header.Epoch().Uint64(),
			)
		}
	}

	subCommittee, err := shardState.FindCommitteeByID(header.ShardID())
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"cannot find shard in the shard state at block %d shard %d",
			header.Number(),
			header.ShardID(),
		)
	}
	return subCommittee.BLSPublicKeys()
}
