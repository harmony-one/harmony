package stagedstreamsync

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/stream/common/streammanager"
	syncproto "github.com/harmony-one/harmony/p2p/stream/protocols/sync"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

type syncProtocol interface {
	GetCurrentBlockNumber(ctx context.Context, opts ...syncproto.Option) (uint64, sttypes.StreamID, error)
	GetBlocksByNumber(ctx context.Context, bns []uint64, opts ...syncproto.Option) ([]*types.Block, sttypes.StreamID, error)
	GetRawBlocksByNumber(ctx context.Context, bns []uint64, opts ...syncproto.Option) ([][]byte, [][]byte, sttypes.StreamID, error)
	GetBlockHashes(ctx context.Context, bns []uint64, opts ...syncproto.Option) ([]common.Hash, sttypes.StreamID, error)
	GetBlocksByHashes(ctx context.Context, hs []common.Hash, opts ...syncproto.Option) ([]*types.Block, sttypes.StreamID, error)
	GetReceipts(ctx context.Context, hs []common.Hash, opts ...syncproto.Option) (receipts []types.Receipts, stid sttypes.StreamID, err error)
	GetNodeData(ctx context.Context, hs []common.Hash, opts ...syncproto.Option) (data [][]byte, stid sttypes.StreamID, err error)
	GetAccountRange(ctx context.Context, root common.Hash, origin common.Hash, limit common.Hash, bytes uint64, opts ...syncproto.Option) (accounts []*message.AccountData, proof [][]byte, stid sttypes.StreamID, err error)
	GetStorageRanges(ctx context.Context, root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64, opts ...syncproto.Option) (slots [][]*message.StorageData, proof [][]byte, stid sttypes.StreamID, err error)
	GetByteCodes(ctx context.Context, hs []common.Hash, bytes uint64, opts ...syncproto.Option) (codes [][]byte, stid sttypes.StreamID, err error)
	GetTrieNodes(ctx context.Context, root common.Hash, paths []*message.TrieNodePathSet, bytes uint64, opts ...syncproto.Option) (nodes [][]byte, stid sttypes.StreamID, err error)

	RemoveStream(stID sttypes.StreamID) // If a stream delivers invalid data, remove the stream
	StreamFailed(stID sttypes.StreamID, reason string)
	SubscribeAddStreamEvent(ch chan<- streammanager.EvtStreamAdded) event.Subscription
	NumStreams() int
}

type blockChain interface {
	engine.ChainReader
	Engine() engine.Engine

	InsertChain(chain types.Blocks, verifyHeaders bool) (int, error)
	WriteCommitSig(blockNum uint64, lastCommits []byte) error
}
