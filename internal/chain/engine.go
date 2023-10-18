package chain

import (
	"bytes"
	"math/big"
	"sort"
	"time"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"

	bls2 "github.com/harmony-one/bls/ffi/go/bls"
	blsvrf "github.com/harmony-one/harmony/crypto/vrf/bls"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/harmony-one/harmony/staking/availability"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	verifiedSigCache = 100
	epochCtxCache    = 20
	vrfBeta          = 32 // 32 bytes randomness
	vrfProof         = 96 // 96 bytes proof (bls sig)
)

type engineImpl struct {
	// Caching field
	epochCtxCache    *lru.Cache // epochCtxKey -> epochCtx
	verifiedSigCache *lru.Cache // verifiedSigKey -> struct{}{}
}

// NewEngine creates Engine with some cache
func NewEngine() *engineImpl {
	sigCache, _ := lru.New(verifiedSigCache)
	epochCtxCache, _ := lru.New(epochCtxCache)
	return &engineImpl{
		epochCtxCache:    epochCtxCache,
		verifiedSigCache: sigCache,
	}
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
// WARN: Do not use VerifyHeaders for now. Currently a header verification can only
// success when the previous header is written to block chain
// TODO: Revisit and correct this function when adding epochChain
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

// VerifyVRF implements Engine, checking the vrf is valid
func (e *engineImpl) VerifyVRF(
	bc engine.ChainReader, header *block.Header,
) error {
	if bc.ShardID() != header.ShardID() {
		return errors.Errorf(
			"[VerifyVRF] shardID not match %d %d", bc.ShardID(), header.ShardID(),
		)
	}

	if bc.Config().IsVRF(header.Epoch()) && len(header.Vrf()) != vrfBeta+vrfProof {
		return errors.Errorf(
			"[VerifyVRF] invalid vrf data format or no vrf proposed %x", header.Vrf(),
		)
	}
	if !bc.Config().IsVRF(header.Epoch()) {
		if len(header.Vrf()) != 0 {
			return errors.Errorf(
				"[VerifyVRF] vrf data present in pre-vrf epoch %x", header.Vrf(),
			)
		}
		return nil
	}

	leaderPubKey, err := GetLeaderPubKeyFromCoinbase(bc, header)

	if leaderPubKey == nil || err != nil {
		return err
	}

	vrfPk := blsvrf.NewVRFVerifier(leaderPubKey.Object)

	previousHeader := bc.GetHeaderByNumber(
		header.Number().Uint64() - 1,
	)
	if previousHeader == nil {
		return errors.New("[VerifyVRF] no parent header found")
	}

	previousHash := previousHeader.Hash()
	vrfProof := [vrfProof]byte{}
	copy(vrfProof[:], header.Vrf()[vrfBeta:])
	hash, err := vrfPk.ProofToHash(previousHash[:], vrfProof[:])

	if err != nil {
		return errors.New("[VerifyVRF] Failed VRF verification")
	}

	if !bytes.Equal(hash[:], header.Vrf()[:vrfBeta]) {
		return errors.New("[VerifyVRF] VRF proof is not valid")
	}

	return nil
}

// GetLeaderPubKeyFromCoinbase retrieve corresponding blsPublicKey from Coinbase Address
func GetLeaderPubKeyFromCoinbase(
	blockchain engine.ChainReader, h *block.Header,
) (*bls.PublicKeyWrapper, error) {
	shardState, err := blockchain.ReadShardState(h.Epoch())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read shard state %v %s",
			h.Epoch(),
			h.Coinbase().Hash().Hex(),
		)
	}

	committee, err := shardState.FindCommitteeByID(h.ShardID())
	if err != nil {
		return nil, err
	}

	committerKey := new(bls2.PublicKey)
	isStaking := blockchain.Config().IsStaking(h.Epoch())
	for _, member := range committee.Slots {
		if isStaking {
			// After staking the coinbase address will be the address of bls public key
			if utils.GetAddressFromBLSPubKeyBytes(member.BLSPublicKey[:]) == h.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		} else {
			if member.EcdsaAddress == h.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		}
	}
	return nil, errors.Errorf(
		"cannot find corresponding BLS Public Key coinbase %s",
		h.Coinbase().Hex(),
	)
}

// VerifySeal implements Engine, checking whether the given block's parent block satisfies
// the PoS difficulty requirements, i.e. >= 2f+1 valid signatures from the committee
// Note that each block header contains the bls signature of the parent block
func (e *engineImpl) VerifySeal(chain engine.ChainReader, header *block.Header) error {
	if chain.CurrentHeader().Number().Uint64() <= uint64(1) {
		return nil
	}
	if header == nil {
		return errors.New("[VerifySeal] nil block header")
	}

	parentHash := header.ParentHash()
	parentHeader := chain.GetHeader(parentHash, header.Number().Uint64()-1)
	if parentHeader == nil {
		return errors.New("[VerifySeal] no parent header found")
	}

	pas := payloadArgsFromHeader(parentHeader)
	sas := sigArgs{
		sig:    header.LastCommitSignature(),
		bitmap: header.LastCommitBitmap(),
	}

	if err := e.verifySignatureCached(chain, pas, sas); err != nil {
		return errors.Wrapf(err, "verify signature for parent %s", parentHash.String())
	}
	return nil
}

// Finalize implements Engine, accumulating the block rewards,
// setting the final state and assembling the block.
// sigsReady signal indicates whether the commit sigs are populated in the header object.
func (e *engineImpl) Finalize(
	chain engine.ChainReader, beacon engine.ChainReader, header *block.Header,
	state *state.DB, txs []*types.Transaction,
	receipts []*types.Receipt, outcxs []*types.CXReceipt,
	incxs []*types.CXReceiptsProof, stks staking.StakingTransactions,
	doubleSigners slash.Records, sigsReady chan bool, viewID func() uint64,
) (*types.Block, reward.Reader, error) {

	isBeaconChain := header.ShardID() == shard.BeaconChainShardID
	inStakingEra := chain.Config().IsStaking(header.Epoch())

	// Process Undelegations, set LastEpochInCommittee and set EPoS status
	// Needs to be before AccumulateRewardsAndCountSigs
	if IsCommitteeSelectionBlock(chain, header) {
		startTime := time.Now()
		if err := payoutUndelegations(chain, header, state); err != nil {
			return nil, nil, err
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("PayoutUndelegations")

		// Needs to be after payoutUndelegations because payoutUndelegations
		// depends on the old LastEpochInCommittee

		startTime = time.Now()
		if err := setElectionEpochAndMinFee(chain, header, state, chain.Config()); err != nil {
			return nil, nil, err
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("SetElectionEpochAndMinFee")

		curShardState, err := chain.ReadShardState(chain.CurrentBlock().Epoch())
		if err != nil {
			return nil, nil, err
		}
		startTime = time.Now()
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
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("ComputeAndMutateEPOSStatus")
	}

	// Accumulate block rewards and commit the final state root
	// Header seems complete, assemble into a block and return
	remainder, payout, err := AccumulateRewardsAndCountSigs(
		chain, state, header, beacon, sigsReady,
	)
	if err != nil {
		return nil, nil, err
	}

	// Apply slashes
	if isBeaconChain && inStakingEra && len(doubleSigners) > 0 {
		if err := applySlashes(chain, header, state, doubleSigners); err != nil {
			return nil, nil, err
		}
	} else if len(doubleSigners) > 0 {
		return nil, nil, errors.New("slashes proposed in non-beacon chain or non-staking epoch")
	}

	// ViewID setting needs to happen after commig sig reward logic for pipelining reason.
	// TODO: make the viewID fetch from caller of the block proposal.
	header.SetViewID(new(big.Int).SetUint64(viewID()))

	// Add the emission recovery split to the balance
	if chain.Config().IsHIP30(header.Epoch()) {
		// convert to ONE - note that numeric.Dec
		// is designed for staking decimals and not
		// ONE balances so we use big.Int for this math
		remainderOne := new(big.Int).Div(
			remainder.Int, big.NewInt(denominations.One),
		)
		// this goes directly to the balance (on shard 0, of course)
		// because the reward mechanism isn't built to handle
		// rewards not obtained from any delegations
		state.AddBalance(
			shard.Schedule.InstanceForEpoch(header.Epoch()).
				HIP30RecoveryAddress(),
			remainderOne,
		)
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
	lockPeriod := GetLockPeriodInEpoch(chain, header.Epoch())
	noEarlyUnlock := chain.Config().IsNoEarlyUnlock(header.Epoch())
	for _, validator := range validators {
		wrapper, err := state.ValidatorWrapper(validator, true, false)
		if err != nil {
			return errors.New(
				"[Finalize] failed to get validator from state to finalize",
			)
		}
		for i := range wrapper.Delegations {
			delegation := &wrapper.Delegations[i]
			totalWithdraw := delegation.RemoveUnlockedUndelegations(
				header.Epoch(), wrapper.LastEpochInCommittee, lockPeriod, noEarlyUnlock,
			)
			if totalWithdraw.Sign() != 0 {
				state.AddBalance(delegation.DelegatorAddress, totalWithdraw)
			}
		}
		countTrack[validator] = len(wrapper.Delegations)
	}

	utils.Logger().Debug().
		Uint64("epoch", header.Epoch().Uint64()).
		Uint64("block-number", header.Number().Uint64()).
		Interface("count-track", countTrack).
		Msg("paid out delegations")

	return nil
}

// IsCommitteeSelectionBlock checks if the given header is for the committee selection block
// which can only occur on beacon chain and if epoch > pre-staking epoch.
func IsCommitteeSelectionBlock(chain engine.ChainReader, header *block.Header) bool {
	isBeaconChain := header.ShardID() == shard.BeaconChainShardID
	inPreStakingEra := chain.Config().IsPreStaking(header.Epoch())
	return isBeaconChain && header.IsLastBlockInEpoch() && inPreStakingEra
}

func setElectionEpochAndMinFee(chain engine.ChainReader, header *block.Header, state *state.DB, config *params.ChainConfig) error {
	newShardState, err := header.GetShardState()
	if err != nil {
		const msg = "[Finalize] failed to read shard state"
		return errors.New(msg)
	}
	// these 2 should be created outside of loop to optimize
	minRate := availability.MinCommissionRate(
		config.IsMinCommissionRate(newShardState.Epoch),
		config.IsHIP30(newShardState.Epoch),
	)
	minRateNotZero := !minRate.Equal(numeric.ZeroDec())
	// elected validators have their fee updated, if required to do so
	isElected := make(
		map[common.Address]struct{},
		len(newShardState.StakedValidators().Addrs),
	)
	for _, addr := range newShardState.StakedValidators().Addrs {
		wrapper, err := state.ValidatorWrapper(addr, true, false)
		if err != nil {
			return errors.New(
				"[Finalize] failed to get validator from state to finalize",
			)
		}
		// Set last epoch in committee
		wrapper.LastEpochInCommittee = newShardState.Epoch
		if minRateNotZero {
			// Set first election epoch (applies only if previously unset)
			state.SetValidatorFirstElectionEpoch(addr, newShardState.Epoch)
			// Update minimum commission fee
			if _, err := availability.UpdateMinimumCommissionFee(
				newShardState.Epoch, state, addr, minRate,
				config.MinCommissionPromoPeriod.Uint64(),
			); err != nil {
				return err
			}
		}
		isElected[addr] = struct{}{}
	}
	// due to a bug in the old implementation of the minimum fee,
	// unelected validators did not have their fee updated even
	// when the protocol required them to do so. here we fix it,
	// but only after the HIP-30 hard fork is effective.
	if config.IsHIP30(newShardState.Epoch) {
		for _, addr := range chain.ValidatorCandidates() {
			// skip elected validator
			if _, ok := isElected[addr]; ok {
				continue
			}
			if _, err := availability.UpdateMinimumCommissionFee(
				newShardState.Epoch, state, addr, minRate,
				config.MinCommissionPromoPeriod.Uint64(),
			); err != nil {
				return err
			}
		}
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
		groupedRecords[thisKey] = append(groupedRecords[thisKey], doubleSigners[i])
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

	// The Leader of the block gets all slashing rewards.
	slashRewardBeneficiary := header.Coinbase()

	// Do the slashing by groups in the sorted order
	for _, key := range sortedKeys {
		records := groupedRecords[key]

		utils.Logger().Info().
			RawJSON("records", []byte(records.String())).
			Msg("now applying slash to state during block finalization")

		// Apply the slashes, invariant: assume been verified as legit slash by this point
		slashApplied, err := slash.Apply(
			chain,
			state,
			records,
			slashRewardBeneficiary,
		)

		if err != nil {
			return errors.New("[Finalize] could not apply slash")
		}

		utils.Logger().Info().
			RawJSON("records", []byte(records.String())).
			RawJSON("applied", []byte(slashApplied.String())).
			Msg("slash applied successfully")
	}
	return nil
}

// VerifyHeaderSignature verifies the signature of the given header.
// Similiar to VerifyHeader, which is only for verifying the block headers of one's own chain, this verification
// is used for verifying "incoming" block header against commit signature and bitmap sent from the other chain cross-shard via libp2p.
// i.e. this header verification api is more flexible since the caller specifies which commit signature and bitmap to use
// for verifying the block header, which is necessary for cross-shard block header verification. Example of such is cross-shard transaction.
func (e *engineImpl) VerifyHeaderSignature(chain engine.ChainReader, header *block.Header, commitSig bls_cosi.SerializedSignature, commitBitmap []byte) error {
	if chain.CurrentHeader().Number().Uint64() <= uint64(1) {
		return nil
	}
	pas := payloadArgsFromHeader(header)
	sas := sigArgs{commitSig, commitBitmap}

	return e.verifySignatureCached(chain, pas, sas)
}

// VerifyCrossLink verifies the signature of the given CrossLink.
func (e *engineImpl) VerifyCrossLink(chain engine.ChainReader, cl types.CrossLink) error {
	if cl.BlockNum() <= 1 {
		return errors.New("crossLink BlockNumber should greater than 1")
	}
	if !chain.Config().IsCrossLink(cl.Epoch()) {
		return errors.Errorf("not cross-link epoch: %v", cl.Epoch())
	}

	pas := payloadArgsFromCrossLink(cl)
	sas := sigArgs{cl.Signature(), cl.Bitmap()}

	return e.verifySignatureCached(chain, pas, sas)
}

func (e *engineImpl) verifySignatureCached(chain engine.ChainReader, pas payloadArgs, sas sigArgs) error {
	verifiedKey := newVerifiedSigKey(pas.blockHash, sas.sig, sas.bitmap)
	if _, ok := e.verifiedSigCache.Get(verifiedKey); ok {
		return nil
	}
	// Not in cache, do verify.
	if err := e.verifySignature(chain, pas, sas); err != nil {
		return err
	}
	e.verifiedSigCache.Add(verifiedKey, struct{}{})
	return nil
}

func (e *engineImpl) verifySignature(chain engine.ChainReader, pas payloadArgs, sas sigArgs) error {
	ec, err := e.getEpochCtxCached(chain, pas.shardID, pas.epoch.Uint64())
	if err != nil {
		return err
	}
	var (
		pubKeys      = ec.pubKeys
		qrVerifier   = ec.qrVerifier
		commitSig    = sas.sig
		commitBitmap = sas.bitmap
	)
	aggSig, mask, err := DecodeSigBitmap(commitSig, commitBitmap, pubKeys)
	if err != nil {
		return errors.Wrap(err, "deserialize signature and bitmap")
	}
	if !qrVerifier.IsQuorumAchievedByMask(mask) {
		return errors.New("not enough signature collected")
	}
	commitPayload := pas.constructPayload(chain)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return errors.New("Unable to verify aggregated signature for block")
	}
	return nil
}

func (e *engineImpl) getEpochCtxCached(chain engine.ChainReader, shardID uint32, epoch uint64) (epochCtx, error) {
	ecKey := epochCtxKey{
		shardID: shardID,
		epoch:   epoch,
	}
	cached, ok := e.epochCtxCache.Get(ecKey)
	if ok && cached != nil {
		return cached.(epochCtx), nil
	}
	ec, err := readEpochCtxFromChain(chain, ecKey)
	if err != nil {
		return epochCtx{}, err
	}
	e.epochCtxCache.Add(ecKey, ec)
	return ec, nil
}

// Support 512 at most validator nodes
const bitmapKeyBytes = 64

// verifiedSigKey is the key for caching header verification results
type verifiedSigKey struct {
	blockHash common.Hash
	signature bls_cosi.SerializedSignature
	bitmap    [bitmapKeyBytes]byte
}

func newVerifiedSigKey(blockHash common.Hash, sig bls_cosi.SerializedSignature, bitmap []byte) verifiedSigKey {
	var keyBM [bitmapKeyBytes]byte
	copy(keyBM[:], bitmap)

	return verifiedSigKey{
		blockHash: blockHash,
		signature: sig,
		bitmap:    keyBM,
	}
}

// payloadArgs is the arguments for constructing the payload for signature verification.
type payloadArgs struct {
	blockHash common.Hash
	shardID   uint32
	epoch     *big.Int
	number    uint64
	viewID    uint64
}

func payloadArgsFromHeader(header *block.Header) payloadArgs {
	return payloadArgs{
		blockHash: header.Hash(),
		shardID:   header.ShardID(),
		epoch:     header.Epoch(),
		number:    header.Number().Uint64(),
		viewID:    header.ViewID().Uint64(),
	}
}

func payloadArgsFromCrossLink(cl types.CrossLink) payloadArgs {
	return payloadArgs{
		blockHash: cl.Hash(),
		shardID:   cl.ShardID(),
		epoch:     cl.Epoch(),
		number:    cl.Number().Uint64(),
		viewID:    cl.ViewID().Uint64(),
	}
}

func (args payloadArgs) constructPayload(chain engine.ChainReader) []byte {
	return signature.ConstructCommitPayload(chain.Config(), args.epoch, args.blockHash, args.number, args.viewID)
}

type sigArgs struct {
	sig    bls_cosi.SerializedSignature
	bitmap []byte
}

type (
	// epochCtxKey is the key for caching epochCtx
	epochCtxKey struct {
		shardID uint32
		epoch   uint64
	}

	// epochCtx is the epoch's context used for signature verification.
	// The value is fixed for each epoch and is cached in engineImpl.
	epochCtx struct {
		qrVerifier quorum.Verifier
		pubKeys    []bls.PublicKeyWrapper
	}
)

func readEpochCtxFromChain(chain engine.ChainReader, key epochCtxKey) (epochCtx, error) {
	var (
		epoch         = new(big.Int).SetUint64(key.epoch)
		targetShardID = key.shardID
	)
	ss, err := readShardState(chain, epoch, targetShardID)
	if err != nil {
		return epochCtx{}, err
	}
	shardComm, err := ss.FindCommitteeByID(targetShardID)
	if err != nil {
		return epochCtx{}, err
	}
	pubKeys, err := shardComm.BLSPublicKeys()
	if err != nil {
		return epochCtx{}, err
	}
	isStaking := chain.Config().IsStaking(epoch)
	qrVerifier, err := quorum.NewVerifier(shardComm, epoch, isStaking)
	if err != nil {
		return epochCtx{}, err
	}
	return epochCtx{
		qrVerifier: qrVerifier,
		pubKeys:    pubKeys,
	}, nil
}

func readShardState(chain engine.ChainReader, epoch *big.Int, targetShardID uint32) (*shard.State, error) {
	// When doing cross shard, we need recalculate the shard state since we don't have
	// shard state of other shards
	if needRecalculateStateShard(chain, epoch, targetShardID) {
		shardState, err := committee.WithStakingEnabled.Compute(epoch, chain)
		if err != nil {
			return nil, errors.Wrapf(err, "compute shard state for epoch %v", epoch)
		}
		return shardState, nil

	} else {
		//shardState, err := chain.ReadShardState(epoch)
		shardState, err := committee.WithStakingEnabled.ReadFromDB(epoch, chain)
		if err != nil {
			return nil, errors.Wrapf(err, "read shard state for epoch %v", epoch)
		}
		return shardState, nil
	}
}

// only recalculate for non-staking epoch and targetShardID is not the same
// as engine
func needRecalculateStateShard(chain engine.ChainReader, epoch *big.Int, targetShardID uint32) bool {
	if chain.Config().IsStaking(epoch) {
		return false
	}
	return targetShardID != chain.ShardID()
}

// GetLockPeriodInEpoch returns the delegation lock period for the given chain
func GetLockPeriodInEpoch(chain engine.ChainReader, epoch *big.Int) int {
	lockPeriod := staking.LockPeriodInEpoch
	if chain.Config().IsRedelegation(epoch) {
		lockPeriod = staking.LockPeriodInEpoch
	} else if chain.Config().IsQuickUnlock(epoch) {
		lockPeriod = staking.LockPeriodInEpochV2
	}
	return lockPeriod
}
