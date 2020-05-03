package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func newSyncerForIncoming(
	stream libp2p_network.Stream,
	host *p2p.Host,
	current chainHeights,
) (*syncingHandler, error) {
	rw := bufio.NewReadWriter(
		bufio.NewReader(stream), bufio.NewWriter(stream),
	)
	return &syncingHandler{
		stream, host, rw, &current,
	}, nil

}

func newSyncerToPeerID(
	peerID libp2p_peer.ID,
	host *p2p.Host,
) (*syncingHandler, error) {

	stream, err := host.IPFSNode.PeerHost.NewStream(
		context.Background(),
		peerID,
		p2p.Protocol,
	)

	if err != nil {
		return nil, err
	}

	rw := bufio.NewReadWriter(
		bufio.NewReader(stream), bufio.NewWriter(stream),
	)

	return &syncingHandler{stream, host, rw, nil}, nil
}

func protocolPeerHeights(
	conns []ipfs_interface.ConnectionInfo, host *p2p.Host,
) ([]*whoHasIt, error) {
	var collect []*whoHasIt

	for _, neighbor := range conns {

		id := neighbor.ID()
		protocols, err := host.IPFSNode.PeerHost.Peerstore().SupportsProtocols(
			id, p2p.Protocol,
		)

		if err != nil {
			return nil, err
		}

		seen := false
		for _, protocol := range protocols {
			if seen = protocol == p2p.Protocol; seen {
				break
			}
		}

		if !seen {
			continue
		}

		open, err := neighbor.Streams()
		if err != nil {
			return nil, err
		}

		for _, already := range open {
			if already == p2p.Protocol {
				continue
			} else {
				fmt.Println("never connected so continuing")
			}
		}

		handler, err := newSyncerToPeerID(id, host)

		if err != nil {
			return nil, err
		}

		heights, err := handler.askHeight()
		if err != nil {
			return nil, err
		}

		collect = append(collect, &whoHasIt{
			chainHeights: heights, peerID: id,
		})

		if err := handler.finished(); err != nil {
			return nil, err
		}

	}

	utils.Logger().Info().
		Int("connected-harmony-protocol-peers", len(collect)).
		Msg("finished asking heights for state syncing")

	return collect, nil

}

type t struct {
	count  int
	haveIt []libp2p_peer.ID
}

type hashCount struct {
	count int
	hash  common.Hash
}

type mostCommonHash struct {
	beacon hashCount
	shard  hashCount
}

func (m *mostCommonHash) MarshalJSON() ([]byte, error) {
	type c struct {
		Count int    `json:"count"`
		Hash  string `json:"hash"`
	}
	return json.Marshal(struct {
		Beacon c `json:"beacon-chain"`
		Shard  c `json:"shard-chain"`
	}{
		c{m.beacon.count, m.beacon.hash.Hex()},
		c{m.shard.count, m.shard.hash.Hex()},
	})
}

func (m *mostCommonHash) String() string {
	s, _ := json.Marshal(m)
	return string(s)
}

func commonHash(collect []*whoHasIt) mostCommonHash {
	beaconCounters, shardCounters :=
		map[common.Hash]t{}, map[common.Hash]t{}

	for _, c := range collect {

		currentS := shardCounters[c.shard.blockHash]
		currentS.count++
		currentS.haveIt = append(currentS.haveIt, c.peerID)
		shardCounters[c.shard.blockHash] = currentS

		currentB := beaconCounters[c.beacon.blockHash]
		currentB.count++
		currentB.haveIt = append(currentB.haveIt, c.peerID)
		beaconCounters[c.beacon.blockHash] = currentB

	}

	type withHash struct {
		t
		hash common.Hash
	}

	b, s := []withHash{}, []withHash{}

	for h, value := range beaconCounters {
		b = append(b, withHash{t: value, hash: h})
	}

	for h, value := range shardCounters {
		s = append(s, withHash{t: value, hash: h})
	}

	sort.SliceStable(b, func(i, j int) bool {
		return b[i].count > s[j].count
	})

	sort.SliceStable(s, func(i, j int) bool {
		return s[i].count > s[j].count
	})

	return mostCommonHash{
		beacon: hashCount{b[0].count, b[0].hash},
		shard:  hashCount{s[0].count, s[0].hash},
	}
}

func syncFromHMYPeersIfNeeded(host *p2p.Host, current chainHeights) error {
	conns, err := host.CoreAPI.Swarm().Peers(context.Background())
	if err != nil {
		return err
	}

	collect, err := protocolPeerHeights(conns, host)
	if err != nil {
		return err
	}

	most := commonHash(collect)

	fmt.Println("most common", most.String())

	utils.Logger().Info().
		Int("protocol-peers", len(collect)).
		Msg("hmy protocol neighborse")

	return nil
}

type syncingHandler struct {
	rawStream libp2p_network.Stream
	host      *p2p.Host
	rw        *bufio.ReadWriter
	current   *chainHeights
}

var requestBlockHeight []byte

func init() {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT,
	}
	data, err := constructHarmonyP2PMsgForClientRoles(message)
	if err != nil {
		panic("die ->" + err.Error())
	}
	requestBlockHeight = data
}

func constructHarmonyP2PMsgForClientRoles(
	message *msg_pb.Message,
) ([]byte, error) {
	msg, err := protobuf.Marshal(message)
	if err != nil {
		return nil, err
	}
	byteBuffer := bytes.NewBuffer([]byte{byte(proto_node.Client)})
	byteBuffer.Write(msg)
	return p2p.ConstructMessage(byteBuffer.Bytes()), nil
}

func checkValidSyncResp(resp *msg_pb.Message) error {
	if r := resp.GetServiceType(); r != msg_pb.ServiceType_CLIENT_SUPPORT {
		return errors.Wrapf(
			errNotClientSupport, "wrongly received %v", r,
		)
	}

	if r := resp.GetType(); r != msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT &&
		r != msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEADER &&
		r != msg_pb.MessageType_SYNC_RESPONSE_BLOCK {
		return errors.Wrapf(errWrongReply, "wrongly received %v", r)
	}
	return nil
}

func (sync *syncingHandler) finished() error {
	if err := sync.rw.Flush(); err != nil {
		return err
	}

	return sync.rawStream.Close()
}

func (sync *syncingHandler) receiveResponse(
	heightReply chan chainHeights,
	headersReply chan []*block.Header,
	blocksReply chan []*types.Block,
) error {

	resp, err := sync.readHarmonyMessage()
	if err != nil {
		return err
	}

	if err := checkValidSyncResp(resp); err != nil {
		return err
	}

	switch resp.GetType() {
	case msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT:
		peerHeights := resp.GetSyncBlockHeight()
		heightReply <- chainHeights{
			beacon: height{
				blockNum:  peerHeights.GetBeaconHeight(),
				blockHash: common.BytesToHash(peerHeights.GetBeaconHash()),
			},
			shard: height{
				blockNum:  peerHeights.GetShardHeight(),
				blockHash: common.BytesToHash(peerHeights.GetShardHash()),
			},
		}

	case msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEADER:
		data := resp.GetSyncBlockHeader().GetHeaderRlp()
		var replyHeaders []*block.Header
		if err := rlp.DecodeBytes(data, replyHeaders); err != nil {
			return err
		}
		headersReply <- replyHeaders

	case msg_pb.MessageType_SYNC_RESPONSE_BLOCK:
		data := resp.GetSyncBlock().GetBlockRlp()
		var replyBlocks []*types.Block
		if err := rlp.DecodeBytes(data, replyBlocks); err != nil {
			return err
		}
		blocksReply <- replyBlocks
	}

	return nil

}

func (sync *syncingHandler) askHeight() (*chainHeights, error) {

	if err := sync.sendBytesWithFlush(
		requestBlockHeight,
	); err != nil {
		return nil, err
	}

	heightReply, headersReply, blocksReply := newReplyChannels()

	if err := sync.receiveResponse(
		heightReply, headersReply, blocksReply,
	); err != nil {
		return nil, err
	}

	c := <-heightReply
	return &c, nil
}

func (sync *syncingHandler) sendHarmonyMessageAsBytes(message *msg_pb.Message) error {
	msg, err := constructHarmonyP2PMsgForClientRoles(message)
	if err != nil {
		return err
	}
	return sync.sendBytesWithFlush(msg)
}

func (sync *syncingHandler) askBlockHeader(
	r consensus.Range,
) ([]*block.Header, error) {

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEADER,
		Request: &msg_pb.Message_SyncBlockHeader{
			SyncBlockHeader: &msg_pb.SyncBlockHeader{
				HeightStart: r.Start,
				HeightEnd:   r.End,
			},
		},
	}

	if err := sync.sendHarmonyMessageAsBytes(message); err != nil {
		return nil, err
	}

	heightReply, headersReply, blocksReply := newReplyChannels()

	if err := sync.receiveResponse(
		heightReply, headersReply, blocksReply,
	); err != nil {
		return nil, err
	}

	select {
	case r := <-headersReply:
		return r, nil
	default:
		return nil, errors.New("did not receive headers reply")
	}

}

func newReplyChannels() (
	chan chainHeights, chan []*block.Header, chan []*types.Block,
) {
	return make(chan chainHeights, 1),
		make(chan []*block.Header, 1),
		make(chan []*types.Block, 1)
}

func (sync *syncingHandler) askBlock(
	r consensus.Range,
) ([]*types.Block, error) {

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_REQUEST_BLOCK,
		Request: &msg_pb.Message_SyncBlock{
			SyncBlock: &msg_pb.SyncBlock{
				HeightStart: r.Start,
				HeightEnd:   r.End,
			},
		},
	}

	if err := sync.sendHarmonyMessageAsBytes(message); err != nil {
		return nil, err
	}

	heightReply, headersReply, blocksReply := newReplyChannels()

	if err := sync.receiveResponse(
		heightReply, headersReply, blocksReply,
	); err != nil {
		return nil, err
	}

	select {
	case blocks := <-blocksReply:
		return blocks, nil
	default:
		return nil, errors.New("did not receive blocks reply")
	}

}

var (
	errNotClientSupport   = errors.New("not a client support message")
	errNotAHarmonyMessage = errors.New(
		"incoming message from stream did not begin with 0x11",
	)
	errWrongReply = errors.New("wrong reply from other side of stream connection")
)

func (sync *syncingHandler) sendBytesWithFlush(data []byte) error {
	if _, err := sync.rw.Write(data); err != nil {
		return err
	}
	return sync.rw.Flush()
}

func (sync *syncingHandler) replyWithCurrentBlockHeight() error {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT,
		Request: &msg_pb.Message_SyncBlockHeight{
			SyncBlockHeight: &msg_pb.SyncBlockHeight{
				BeaconHeight: sync.current.beacon.blockNum,
				BeaconHash:   sync.current.beacon.blockHash.Bytes(),
				ShardHeight:  sync.current.shard.blockNum,
				ShardHash:    sync.current.shard.blockHash.Bytes(),
			},
		},
	}

	return sync.sendHarmonyMessageAsBytes(message)
}

func (sync *syncingHandler) handleMessage(msg *msg_pb.Message) error {
	if msg.GetServiceType() != msg_pb.ServiceType_CLIENT_SUPPORT {
		return errNotClientSupport
	}

	if msg.GetType() == msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT {
		return sync.replyWithCurrentBlockHeight()
	}

	return nil
}

func (sync *syncingHandler) readHarmonyMessage() (*msg_pb.Message, error) {

	buf, err := sync.rw.Peek(1)
	if err != nil {
		return nil, err
	}

	if buf[0] == 0x11 {
		_, err := sync.rw.ReadByte()
		if err != nil {
			return nil, err
		}

		var msgBuf msg_pb.Message
		contentSizeBuf := make([]byte, 4)

		if _, err := io.ReadFull(sync.rw, contentSizeBuf); err != nil {
			return nil, err
		}

		sized := binary.BigEndian.Uint32(contentSizeBuf)
		payload := make([]byte, sized)

		if _, err := io.ReadFull(sync.rw, payload); err != nil {
			return nil, err
		}

		// TODO double check that that one byte is indeed "client"
		// That one extra middle offset byte --
		if err := protobuf.Unmarshal(payload[1:], &msgBuf); err != nil {
			return nil, err
		}
		return &msgBuf, nil
	}

	return nil, errNotAHarmonyMessage
}

func (sync *syncingHandler) processIncoming() error {
	// this must be our message
	msg, err := sync.readHarmonyMessage()
	if err != nil {
		return err
	}

	return sync.handleMessage(msg)
}

type height struct {
	blockNum  uint64
	blockHash common.Hash
}

type chainHeights struct {
	beacon height
	shard  height
}

type whoHasIt struct {
	*chainHeights
	peerID libp2p_peer.ID
}

// HandleBlockSyncing ..
func (node *Node) HandleBlockSyncing() error {
	var g errgroup.Group
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()

	g.Go(func() error {
		i := 0

		for {
			select {
			case blockRange := <-node.Consensus.SyncNeeded:
				_ = blockRange
			case <-t.C:
				fmt.Println("tick went off", i)
				i++
				if err := syncFromHMYPeersIfNeeded(
					node.host, node.heightNow(),
				); err != nil {
					return err
				}
			}
		}

	})

	return g.Wait()
}

func (node *Node) heightNow() chainHeights {
	currentBeacon := node.Blockchain().CurrentBlock()
	currentShard := node.Beaconchain().CurrentBlock()

	return chainHeights{
		beacon: height{
			blockNum:  currentBeacon.Number().Uint64(),
			blockHash: currentBeacon.Hash(),
		},
		shard: height{
			blockNum:  currentShard.Number().Uint64(),
			blockHash: currentShard.Hash(),
		},
	}
}

// HandleIncomingHMYProtocolStreams ..
func (node *Node) HandleIncomingHMYProtocolStreams() error {
	var g errgroup.Group
	for stream := range node.host.IncomingStream {
		s := stream
		g.Go(func() error {

			handler, err := newSyncerForIncoming(
				s, node.host, node.heightNow(),
			)

			if err != nil {
				return err
			}

			if err := handler.processIncoming(); err != nil {
				return err
			}

			// if err := handler.finished(); err != nil {
			// 	return err
			// }

			return nil
		})
	}

	return g.Wait()
}
