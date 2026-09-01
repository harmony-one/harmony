package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/stretchr/testify/require"
)

type countingDownloader struct {
	n int
}

func (c *countingDownloader) DownloadAsync() {
	c.n++
}

func newSyncingConsensus(t *testing.T, blockNum uint64) (*Consensus, *countingDownloader) {
	t.Helper()
	_, _, cs, _, err := GenerateConsensusForTesting()
	require.NoError(t, err)
	downloader := &countingDownloader{}
	cs.dHelper = downloader
	cs.setBlockNum(blockNum)
	cs.current.SetMode(Syncing)
	return cs, downloader
}

func TestOnPreparedIgnoresFarAheadMessages(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	cs.onPrepared(&FBFTMessage{
		MessageType: msg_pb.MessageType_PREPARED,
		BlockNum:    100 + MaxBlockNumDiff + 1,
		ViewID:      201,
		BlockHash:   common.HexToHash("0x01"),
		Payload:     []byte("not-a-valid-bitmap"),
	})

	require.Equal(t, 1, downloader.n)
	require.Nil(t, cs.aggregatedPrepareSig)
	require.Equal(t, FBFTAnnounce, cs.getConsensusPhase())
}

func TestOnPreparedProcessesCurrentBlock(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	cs.onPrepared(&FBFTMessage{
		MessageType: msg_pb.MessageType_PREPARED,
		BlockNum:    100,
		ViewID:      100,
		BlockHash:   common.HexToHash("0x01"),
		Payload:     []byte("not-a-valid-bitmap"),
	})

	require.Equal(t, 0, downloader.n)
	require.Nil(t, cs.aggregatedPrepareSig)
	require.Equal(t, FBFTAnnounce, cs.getConsensusPhase())
}

func TestOnPreparedProcessesMessageAtCap(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	cs.onPrepared(&FBFTMessage{
		MessageType: msg_pb.MessageType_PREPARED,
		BlockNum:    100 + MaxBlockNumDiff,
		ViewID:      200,
		BlockHash:   common.HexToHash("0x01"),
		Payload:     []byte("not-a-valid-bitmap"),
	})

	require.Equal(t, 1, downloader.n)
	require.Nil(t, cs.aggregatedPrepareSig)
}

func TestOnCommittedIgnoresFarAheadMessages(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	farHash := common.HexToHash("0x02")
	cs.onCommitted(&FBFTMessage{
		MessageType: msg_pb.MessageType_COMMITTED,
		BlockNum:    100 + MaxBlockNumDiff + 1,
		ViewID:      201,
		BlockHash:   farHash,
		Payload:     []byte("not-a-valid-commit"),
	})

	require.Equal(t, 1, downloader.n)
	require.Empty(t, cs.fBFTLog.GetNotVerifiedCommittedMessages(100+MaxBlockNumDiff+1, 201, farHash))
}

func TestOnCommittedCachesNearbyFutureMessages(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	nearHash := common.HexToHash("0x03")
	cs.onCommitted(&FBFTMessage{
		MessageType: msg_pb.MessageType_COMMITTED,
		BlockNum:    101,
		ViewID:      101,
		BlockHash:   nearHash,
		Payload:     []byte("commit-payload"),
	})

	require.Equal(t, 1, downloader.n)
	require.Len(t, cs.fBFTLog.GetNotVerifiedCommittedMessages(101, 101, nearHash), 1)
}

func TestOnCommittedCachesMessageAtCap(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	hash := common.HexToHash("0x04")
	blockNum := uint64(100 + MaxBlockNumDiff)
	cs.onCommitted(&FBFTMessage{
		MessageType: msg_pb.MessageType_COMMITTED,
		BlockNum:    blockNum,
		ViewID:      blockNum,
		BlockHash:   hash,
		Payload:     []byte("commit-payload"),
	})

	require.Equal(t, 1, downloader.n)
	require.Len(t, cs.fBFTLog.GetNotVerifiedCommittedMessages(blockNum, blockNum, hash), 1)
}

func TestOnCommittedCachesPreviousBlock(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	hash := common.HexToHash("0x05")
	cs.onCommitted(&FBFTMessage{
		MessageType: msg_pb.MessageType_COMMITTED,
		BlockNum:    99,
		ViewID:      99,
		BlockHash:   hash,
		Payload:     []byte("commit-payload"),
	})

	require.Equal(t, 0, downloader.n)
	require.Len(t, cs.fBFTLog.GetNotVerifiedCommittedMessages(99, 99, hash), 1)
}

func TestOnCommittedIgnoresOlderThanPreviousBlock(t *testing.T) {
	cs, downloader := newSyncingConsensus(t, 100)

	hash := common.HexToHash("0x06")
	cs.onCommitted(&FBFTMessage{
		MessageType: msg_pb.MessageType_COMMITTED,
		BlockNum:    98,
		ViewID:      98,
		BlockHash:   hash,
		Payload:     []byte("commit-payload"),
	})

	require.Equal(t, 0, downloader.n)
	require.Empty(t, cs.fBFTLog.GetNotVerifiedCommittedMessages(98, 98, hash))
}
