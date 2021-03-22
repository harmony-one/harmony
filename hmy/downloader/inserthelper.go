package downloader

import (
	"fmt"
	"hash"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
)

// sigVerifyError is the error type of failing verify the signature of the current block.
// Since this is a sanity field and is not included the block hash, it needs extra verification.
// The error types is used to differentiate the error of signature verification VS insert error.
type sigVerifyError struct {
	err error
}

func (err *sigVerifyError) Error() string {
	return fmt.Sprintf("failed verify signature: %v", err.err.Error())
}

// insertHelperImpl helps to verify and insert blocks, along with some caching mechanism.
type insertHelperImpl struct {
	bc blockChain

	deciderCache     *lru.Cache // Epoch -> quorum.Decider
	shardStateCache  *lru.Cache // Epoch -> *shard.State
	verifiedSigCache *lru.Cache // verifiedSigKey -> struct{}{}
}

func newInsertHelper(bc blockChain) insertHelper {
	deciderCache, _ := lru.New(5)
	shardStateCache, _ := lru.New(5)
	sigCache, _ := lru.New(20)
	return &insertHelperImpl{
		bc:               bc,
		deciderCache:     deciderCache,
		shardStateCache:  shardStateCache,
		verifiedSigCache: sigCache,
	}
}

func (ch *insertHelperImpl) verifyAndInsertBlocks(blocks types.Blocks) (int, error) {
	for i, block := range blocks {
		if err := ch.verifyAndInsertBlock(block); err != nil {
			return i, err
		}
	}
	return len(blocks), nil
}

func (ch *insertHelperImpl) verifyAndInsertBlock(block *types.Block) error {
	// verify the commit sig of current block
	if err := ch.verifyBlockSignature(block); err != nil {
		return &sigVerifyError{err}
	}
	ch.markBlockSigVerified(block, block.GetCurrentCommitSig())

	// verify header. Skip verify the previous seal if we have already verified
	verifySeal := !ch.isBlockLastSigVerified(block)
	if err := ch.bc.Engine().VerifyHeader(ch.bc, block.Header(), verifySeal); err != nil {
		return err
	}
	// Insert chain.
	if _, err := ch.bc.InsertChain(types.Blocks{block}, false); err != nil {
		return err
	}
	// Write commit sig data
	return ch.bc.WriteCommitSig(block.NumberU64(), block.GetCurrentCommitSig())
}

func (ch *insertHelperImpl) verifyBlockSignature(block *types.Block) error {
	// TODO: This is the duplicate logic to the implementation of verifySeal and consensus.
	//  Better refactor to the blockchain or engine structure
	decider, err := ch.getDeciderByEpoch(block.Epoch())
	if err != nil {
		return err
	}
	sig, mask, err := decodeCommitSig(block.GetCurrentCommitSig(), decider.Participants())
	if err != nil {
		return err
	}
	if !decider.IsQuorumAchievedByMask(mask) {
		return errors.New("quorum not achieved")
	}

	commitSigBytes := signature.ConstructCommitPayload(ch.bc, block.Epoch(), block.Hash(),
		block.NumberU64(), block.Header().ViewID().Uint64())
	if !sig.VerifyHash(mask.AggregatePublic, commitSigBytes) {
		return errors.New("aggregate signature failed verification")
	}
	return nil
}

func (ch *insertHelperImpl) writeBlockSignature(block *types.Block) error {
	return ch.bc.WriteCommitSig(block.NumberU64(), block.GetCurrentCommitSig())
}

func (ch *insertHelperImpl) getDeciderByEpoch(epoch *big.Int) (quorum.Decider, error) {
	epochUint := epoch.Uint64()
	if decider, ok := ch.deciderCache.Get(epochUint); ok && decider != nil {
		return decider.(quorum.Decider), nil
	}
	decider, err := ch.readDeciderByEpoch(epoch)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read quorum of epoch %v", epoch.Uint64())
	}
	ch.deciderCache.Add(epochUint, decider)
	return decider, nil
}

func (ch *insertHelperImpl) readDeciderByEpoch(epoch *big.Int) (quorum.Decider, error) {
	isStaking := ch.bc.Config().IsStaking(epoch)
	decider := ch.getNewDecider(isStaking)
	ss, err := ch.getShardState(epoch)
	if err != nil {
		return nil, err
	}
	subComm, err := ss.FindCommitteeByID(ch.shardID())
	if err != nil {
		return nil, err
	}
	pubKeys, err := subComm.BLSPublicKeys()
	if err != nil {
		return nil, err
	}
	decider.UpdateParticipants(pubKeys)
	if _, err := decider.SetVoters(subComm, epoch); err != nil {
		return nil, err
	}
	return decider, nil
}

func (ch *insertHelperImpl) getNewDecider(isStaking bool) quorum.Decider {
	if isStaking {
		return quorum.NewDecider(quorum.SuperMajorityVote, ch.bc.ShardID())
	} else {
		return quorum.NewDecider(quorum.SuperMajorityStake, ch.bc.ShardID())
	}
}

func (ch *insertHelperImpl) getShardState(epoch *big.Int) (*shard.State, error) {
	if ss, ok := ch.shardStateCache.Get(epoch.Uint64()); ok && ss != nil {
		return ss.(*shard.State), nil
	}
	ss, err := ch.bc.ReadShardState(epoch)
	if err != nil {
		return nil, err
	}
	ch.shardStateCache.Add(epoch.Uint64(), ss)
	return ss, nil
}

func (ch *insertHelperImpl) markBlockSigVerified(block *types.Block, sigAndBitmap []byte) {
	key := newVerifiedSigKey(block.Hash(), sigAndBitmap)
	ch.verifiedSigCache.Add(key, struct{}{})
}

func (ch *insertHelperImpl) isBlockLastSigVerified(block *types.Block) bool {
	lastSig := block.Header().LastCommitSignature()
	lastBM := block.Header().LastCommitBitmap()
	lastSigAndBM := append(lastSig[:], lastBM...)

	key := newVerifiedSigKey(block.Hash(), lastSigAndBM)
	_, ok := ch.verifiedSigCache.Get(key)
	return ok
}

func (ch *insertHelperImpl) shardID() uint32 {
	return ch.bc.ShardID()
}

func decodeCommitSig(commitBytes []byte, publicKeys multibls.PublicKeys) (*bls_core.Sign, *bls_cosi.Mask, error) {
	if len(commitBytes) < bls_cosi.BLSSignatureSizeInBytes {
		return nil, nil, fmt.Errorf("unexpected signature bytes size: %v / %v", len(commitBytes),
			bls_cosi.BLSSignatureSizeInBytes)
	}
	return chain.ReadSignatureBitmapByPublicKeys(commitBytes, publicKeys)
}

type verifiedSigKey struct {
	blockHash common.Hash
	sbHash    common.Hash // hash of block signature + bitmap
}

var hasherPool = sync.Pool{
	New: func() interface{} {
		return sha3.New256()
	},
}

func newVerifiedSigKey(blockHash common.Hash, sigAndBitmap []byte) verifiedSigKey {
	hasher := hasherPool.Get().(hash.Hash)
	defer func() {
		hasher.Reset()
		hasherPool.Put(hasher)
	}()

	var sbHash common.Hash
	hasher.Write(sigAndBitmap)
	hasher.Sum(sbHash[0:])

	return verifiedSigKey{
		blockHash: blockHash,
		sbHash:    sbHash,
	}
}
