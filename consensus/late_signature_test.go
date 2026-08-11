package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

// TestExcludedLocalCommitKeys checks bitmap exclusion for local committee keys.
func TestExcludedLocalCommitKeys(t *testing.T) {
	pub1 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub2 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub3 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub1.Bytes.FromLibBLSPublicKey(pub1.Object)
	pub2.Bytes.FromLibBLSPublicKey(pub2.Object)
	pub3.Bytes.FromLibBLSPublicKey(pub3.Object)

	mask := bls.NewMask([]bls.PublicKeyWrapper{pub1, pub2, pub3})
	require.NoError(t, mask.SetKey(pub1.Bytes, true))
	require.NoError(t, mask.SetKey(pub3.Bytes, true))

	excluded := excludedLocalCommitKeys(mask, []bls.SerializedPublicKey{pub1.Bytes, pub2.Bytes})
	require.Equal(t, []bls.SerializedPublicKey{pub2.Bytes}, excluded)

	excluded = excludedLocalCommitKeys(mask, []bls.SerializedPublicKey{pub1.Bytes})
	require.Empty(t, excluded)

	// Key not in participants is ignored (err from KeyEnabled).
	outsider := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	outsider.Bytes.FromLibBLSPublicKey(outsider.Object)
	excluded = excludedLocalCommitKeys(mask, []bls.SerializedPublicKey{outsider.Bytes})
	require.Empty(t, excluded)
}

// TestReportLateVoteIfPastFinalized increments late and total only for the prior block.
func TestReportLateVoteIfPastFinalized(t *testing.T) {
	c := &Consensus{current: NewState(Normal, 0)}
	initMetrics()

	recvMsg := &FBFTMessage{
		MessageType: msg_pb.MessageType_COMMIT,
		BlockNum:    10,
		ViewID:      1,
	}
	// Not the immediately previous block — no-op.
	c.reportLateVoteIfPastFinalized(recvMsg, 12)
	c.reportLateVoteIfPastFinalized(nil, 11)

	phase := msg_pb.MessageType_COMMIT.String()
	beforeLate := lateSignatureCount(t, "leader", phase)
	beforeTotal := signatureTotalCount(t, "leader", phase)
	c.reportLateVoteIfPastFinalized(recvMsg, 11)
	require.Equal(t, beforeLate+1, lateSignatureCount(t, "leader", phase))
	require.Equal(t, beforeTotal+1, signatureTotalCount(t, "leader", phase))
}

// TestReportAcceptedVote increments the total counter for on-time votes.
func TestReportAcceptedVote(t *testing.T) {
	c := &Consensus{current: NewState(Normal, 0)}
	initMetrics()

	phase := msg_pb.MessageType_PREPARE.String()
	beforeLate := lateSignatureCount(t, "leader", phase)
	beforeTotal := signatureTotalCount(t, "leader", phase)
	c.reportAcceptedVote(phase)
	require.Equal(t, beforeLate, lateSignatureCount(t, "leader", phase))
	require.Equal(t, beforeTotal+1, signatureTotalCount(t, "leader", phase))
}

// TestReportLocalCommitInclusionsCountsTotal increments total for included and excluded local keys.
func TestReportLocalCommitInclusionsCountsTotal(t *testing.T) {
	pub1 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub2 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub3 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub1.Bytes.FromLibBLSPublicKey(pub1.Object)
	pub2.Bytes.FromLibBLSPublicKey(pub2.Object)
	pub3.Bytes.FromLibBLSPublicKey(pub3.Object)

	mask := bls.NewMask([]bls.PublicKeyWrapper{pub1, pub2})
	require.NoError(t, mask.SetKey(pub1.Bytes, true)) // pub2 excluded

	initMetrics()
	beforeLate := lateSignatureCount(t, "validator", "committed")
	beforeTotal := signatureTotalCount(t, "validator", "committed")

	var missing []bls.SerializedPublicKey
	reportLocalCommitInclusions(mask, []bls.SerializedPublicKey{pub1.Bytes, pub2.Bytes, pub3.Bytes}, func(pub bls.SerializedPublicKey) {
		missing = append(missing, pub)
	})

	require.Equal(t, []bls.SerializedPublicKey{pub2.Bytes}, missing)
	require.Equal(t, beforeLate+1, lateSignatureCount(t, "validator", "committed"))
	require.Equal(t, beforeTotal+2, signatureTotalCount(t, "validator", "committed"))
}

// TestRecordLastCommitSentGatesInclusionCheck skips checks without a matching sent COMMIT.
func TestRecordLastCommitSentGatesInclusionCheck(t *testing.T) {
	pub1 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub2 := bls.PublicKeyWrapper{Object: bls.RandPrivateKey().GetPublicKey()}
	pub1.Bytes.FromLibBLSPublicKey(pub1.Object)
	pub2.Bytes.FromLibBLSPublicKey(pub2.Object)

	mask := bls.NewMask([]bls.PublicKeyWrapper{pub1, pub2})
	require.NoError(t, mask.SetKey(pub2.Bytes, true)) // pub1 excluded

	hash := common.HexToHash("0xabc")
	c := &Consensus{current: NewState(Normal, 0)}
	initMetrics()

	// Without a matching last-sent commit, check is a no-op even if key excluded.
	c.checkOwnCommitInclusion(5, hash, mask)

	c.recordLastCommitSent(5, hash)
	// Still no-op: getPriKeysInCommittee fails with empty priKey.
	c.checkOwnCommitInclusion(5, hash, mask)

	// Wrong hash — no-op.
	c.recordLastCommitSent(5, common.HexToHash("0xdef"))
	c.checkOwnCommitInclusion(5, hash, mask)
}

func lateSignatureCount(t *testing.T, role, phase string) float64 {
	t.Helper()
	return counterValue(t, consensusLateSignatureCounterVec, role, phase)
}

func signatureTotalCount(t *testing.T, role, phase string) float64 {
	t.Helper()
	return counterValue(t, consensusSignatureTotalCounterVec, role, phase)
}

func counterValue(t *testing.T, vec *prometheus.CounterVec, role, phase string) float64 {
	t.Helper()
	metric, err := vec.GetMetricWithLabelValues(role, phase)
	require.NoError(t, err)
	var m dto.Metric
	require.NoError(t, metric.Write(&m))
	return m.GetCounter().GetValue()
}
