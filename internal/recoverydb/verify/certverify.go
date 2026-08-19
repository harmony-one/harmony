package verify

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/rawdb"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/shard"
)

// CertVerifier verifies commit certificates (aggregate BLS signature +
// bitmap) against the committee stored in a database's shard-state records,
// without constructing a chain harness. It replicates the exact semantics of
// engineImpl.verifySignature (internal/chain/engine.go:676-699): committee
// from ss<epoch>, quorum by stake in staking epochs, aggregate signature over
// ConstructCommitPayload.
type CertVerifier struct {
	db      ethdb.KeyValueReader
	config  *params.ChainConfig
	shardID uint32

	epochCache map[uint64]*epochCommittee
}

type epochCommittee struct {
	pubKeys  []bls_cosi.PublicKeyWrapper
	verifier quorum.Verifier
}

// NewCertVerifier builds a verifier over db for one shard.
func NewCertVerifier(db ethdb.KeyValueReader, config *params.ChainConfig, shardID uint32) *CertVerifier {
	return &CertVerifier{db: db, config: config, shardID: shardID, epochCache: map[uint64]*epochCommittee{}}
}

func (cv *CertVerifier) committee(epoch *big.Int) (*epochCommittee, error) {
	if c, ok := cv.epochCache[epoch.Uint64()]; ok {
		return c, nil
	}
	ss, err := rawdb.ReadShardState(cv.db, epoch)
	if err != nil {
		return nil, fmt.Errorf("certverify: read shard state for epoch %d: %w", epoch.Uint64(), err)
	}
	comm, err := ss.FindCommitteeByID(cv.shardID)
	if err != nil {
		return nil, fmt.Errorf("certverify: committee for shard %d epoch %d: %w", cv.shardID, epoch.Uint64(), err)
	}
	pubKeys, err := comm.BLSPublicKeys()
	if err != nil {
		return nil, fmt.Errorf("certverify: decode committee keys epoch %d: %w", epoch.Uint64(), err)
	}
	qr, err := quorum.NewVerifier(comm, epoch, cv.config.IsStaking(epoch))
	if err != nil {
		return nil, fmt.Errorf("certverify: quorum verifier epoch %d: %w", epoch.Uint64(), err)
	}
	c := &epochCommittee{pubKeys: pubKeys, verifier: qr}
	cv.epochCache[epoch.Uint64()] = c
	return c, nil
}

// VerifyHeaderCert verifies sig+bitmap as a quorum commit certificate over
// the given header.
func (cv *CertVerifier) VerifyHeaderCert(header *block.Header, sig bls_cosi.SerializedSignature, bitmap []byte) error {
	return cv.VerifyCert(header.Epoch(), header.Hash(), header.Number().Uint64(), header.ViewID().Uint64(), sig, bitmap)
}

// VerifyCert verifies sig+bitmap over the exact commit payload
// (blockNum ‖ blockHash [‖ viewID in staking epochs]).
func (cv *CertVerifier) VerifyCert(
	epoch *big.Int, blockHash common.Hash, blockNum, viewID uint64,
	sig bls_cosi.SerializedSignature, bitmap []byte,
) error {
	comm, err := cv.committee(epoch)
	if err != nil {
		return err
	}
	aggSig, mask, err := chain.DecodeSigBitmap(sig, bitmap, comm.pubKeys)
	if err != nil {
		return fmt.Errorf("certverify: decode signature/bitmap for block %d: %w", blockNum, err)
	}
	if !comm.verifier.IsQuorumAchievedByMask(mask) {
		return fmt.Errorf("certverify: block %d certificate does not reach quorum", blockNum)
	}
	payload := signature.ConstructCommitPayload(cv.config, epoch, blockHash, blockNum, viewID)
	if !aggSig.VerifyHash(mask.AggregatePublic, payload) {
		return fmt.Errorf("certverify: block %d aggregate signature does not verify over commit payload", blockNum)
	}
	return nil
}

// VerifyCommitSigBytes splits raw sig-and-bitmap bytes (the exact
// block-sig-N value layout) and verifies them over the header.
func (cv *CertVerifier) VerifyCommitSigBytes(header *block.Header, sigAndBitmap []byte) error {
	sig, bitmap, err := chain.ParseCommitSigAndBitmap(sigAndBitmap)
	if err != nil {
		return fmt.Errorf("certverify: parse commit sig+bitmap for block %d: %w", header.Number().Uint64(), err)
	}
	return cv.VerifyHeaderCert(header, sig, bitmap)
}

// ShardIDFromState sanity-reads the shard state to confirm the committee for
// this shard exists at the given epoch (used by preflights).
func (cv *CertVerifier) ShardIDFromState(epoch *big.Int) error {
	_, err := cv.committee(epoch)
	return err
}

var _ = shard.State{} // keep the shard import explicit for DecodeWrapper users
