package chain

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

type engineImpl struct{}

// Engine is an algorithm-agnostic consensus engine.
var Engine = &engineImpl{}

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

// Similiar to VerifyHeader, but used for verifying the block header against some commit signature for
// more flexibility on the api. Example of usage is the new block verification with cx transaction receipts proof
// where the bls signature is given by another shard through beacon chain, not from the child block header.
func (e *engineImpl) VerifyHeaderWithSignature(chain engine.ChainReader, header *block.Header, payload []byte, commitBitmap []byte) error {
	if chain.CurrentHeader().Number().Uint64() <= uint64(1) {
		return nil
	}
	publicKeys, err := retrievePublicKeys(chain, header)
	if err != nil {
		return ctxerror.New("[VerifySeal] Cannot retrieve publickeys for block").WithCause(err)
	}

	aggSig, mask, err := ReadSignatureBitmapByPublicKeys(payload, publicKeys)
	if err != nil {
		return ctxerror.New("[VerifySeal] Unable to deserialize the commitSignature and commitBitmap in Block Header").WithCause(err)
	}
	hash := header.Hash()
	quorum, err := QuorumForBlock(chain, header)
	if err != nil {
		return errors.Wrapf(err,
			"cannot calculate quorum for block %s", header.Number())
	}
	if count := utils.CountOneBits(mask.Bitmap); count < quorum {
		return ctxerror.New("[VerifySeal] Not enough signature in commitSignature from Block Header",
			"need", quorum, "got", count)
	}

	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, header.Number().Uint64()-1)
	commitPayload := append(blockNumHash, hash[:]...)

	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return ctxerror.New("[VerifySeal] Unable to verify aggregated signature for block", "blockNum", header.Number().Uint64()-1, "blockHash", hash)
	}
	return nil
}

// retrievePublicKeys finds the public keys of current block's committee
func retrievePublicKeys(bc engine.ChainReader, header *block.Header) ([]*bls.PublicKey, error) {
	shardState := core.GetShardState(header.Epoch())
	committee := shardState.FindCommitteeByID(header.ShardID())
	if committee == nil {
		return nil, ctxerror.New("cannot find shard in the shard state",
			"blockNumber", header.Number(),
			"shardID", header.ShardID(),
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range committee.NodeList {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", member.BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	return committerKeys, nil
}

// retrievePublicKeysFromLastBlock finds the public keys of last block's committee
func retrievePublicKeysFromLastBlock(bc engine.ChainReader, header *block.Header) ([]*bls.PublicKey, error) {
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return nil, ctxerror.New("cannot find parent block header in DB",
			"parentHash", header.ParentHash())
	}
	parentShardState := core.GetShardState(parentHeader.Epoch())
	parentCommittee := parentShardState.FindCommitteeByID(parentHeader.ShardID())
	if parentCommittee == nil {
		return nil, ctxerror.New("cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number(),
			"shardID", parentHeader.ShardID(),
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range parentCommittee.NodeList {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", member.BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	return committerKeys, nil
}

// VerifySeal implements Engine, checking whether the given block's parent block satisfies
// the PoS difficulty requirements, i.e. >= 2f+1 valid signatures from the committee
// Note that each block header contains the bls signature of the parent block
func (e *engineImpl) VerifySeal(chain engine.ChainReader, header *block.Header) error {
	if chain.CurrentHeader().Number().Uint64() <= uint64(1) {
		return nil
	}
	publicKeys, err := retrievePublicKeysFromLastBlock(chain, header)
	if err != nil {
		return ctxerror.New("[VerifySeal] Cannot retrieve publickeys from last block").WithCause(err)
	}
	sig := header.LastCommitSignature()
	payload := append(sig[:], header.LastCommitBitmap()...)
	aggSig, mask, err := ReadSignatureBitmapByPublicKeys(payload, publicKeys)
	if err != nil {
		return ctxerror.New("[VerifySeal] Unable to deserialize the LastCommitSignature and LastCommitBitmap in Block Header").WithCause(err)
	}
	parentHash := header.ParentHash()
	parentHeader := chain.GetHeader(parentHash, header.Number().Uint64()-1)
	parentQuorum, err := QuorumForBlock(chain, parentHeader)
	if err != nil {
		return errors.Wrapf(err,
			"cannot calculate quorum for block %s", header.Number())
	}
	if count := utils.CountOneBits(mask.Bitmap); count < parentQuorum {
		return ctxerror.New("[VerifySeal] Not enough signature in LastCommitSignature from Block Header",
			"need", parentQuorum, "got", count)
	}

	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, header.Number().Uint64()-1)
	lastCommitPayload := append(blockNumHash, parentHash[:]...)

	if !aggSig.VerifyHash(mask.AggregatePublic, lastCommitPayload) {
		return ctxerror.New("[VerifySeal] Unable to verify aggregated signature from last block", "lastBlockNum", header.Number().Uint64()-1, "lastBlockHash", parentHash)
	}
	return nil
}

// Finalize implements Engine, accumulating the block rewards,
// setting the final state and assembling the block.
func (e *engineImpl) Finalize(chain engine.ChainReader, header *block.Header, state *state.DB, txs []*types.Transaction, receipts []*types.Receipt, outcxs []*types.CXReceipt, incxs []*types.CXReceiptsProof) (*types.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	// Header seems complete, assemble into a block and return
	if err := AccumulateRewards(chain, state, header); err != nil {
		return nil, ctxerror.New("cannot pay block reward").WithCause(err)
	}
	header.SetRoot(state.IntermediateRoot(chain.Config().IsS3(header.Epoch())))
	return types.NewBlock(header, txs, receipts, outcxs, incxs), nil
}

// QuorumForBlock returns the quorum for the given block header.
func QuorumForBlock(
	chain engine.ChainReader, h *block.Header,
) (quorum int, err error) {
	ss, err := chain.ReadShardState(h.Epoch())
	if err != nil {
		return 0, errors.Wrapf(err,
			"cannot read shard state for epoch %s", h.Epoch())
	}
	c := ss.FindCommitteeByID(h.ShardID())
	if c == nil {
		return 0, errors.Errorf(
			"cannot find shard %d in shard state", h.ShardID())
	}
	return (len(c.NodeList))*2/3 + 1, nil
}
