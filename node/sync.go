package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

func newSyncerForIncoming(
	stream libp2p_network.Stream,
	host *p2p.Host,
	current chan chainHeights,
) (*syncingHandler, error) {
	rw := bufio.NewReadWriter(
		bufio.NewReader(stream), bufio.NewWriter(stream),
	)
	return &syncingHandler{
		stream, host, rw, current,
	}, nil

}

func newSyncerToPeerID(
	peerID libp2p_peer.ID,
	host *p2p.Host,
	current chan chainHeights,
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

	// TODO probably need to expose the raw stream as well
	return &syncingHandler{stream, host, rw, current}, nil

}

func askEveryoneInHMYProtocolTheirHeight(host *p2p.Host) error {
	//
	return nil
}

func syncFromHMYPeersIfNeeded(
	host *p2p.Host,
	currentBeacon, currentShard height,
) error {

	conns, err := host.CoreAPI.Swarm().Peers(context.Background())
	if err != nil {
		return err
	}

	for _, neighbor := range conns {
		id := neighbor.ID()
		protocols, err := host.IPFSNode.PeerHost.Peerstore().SupportsProtocols(
			id, p2p.Protocol,
		)

		if err != nil {
			return err
		}

		if len(protocols) == 0 {
			continue
		}

		handler, err := newSyncerToPeerID(id, host, nil)

		if err != nil {
			return err
		}

		heights, err := handler.askHeight()

		if err != nil {
			return err
		}

		fmt.Println("received this as reply->", heights)
	}
	// everyonesHeaderHashs := askEveryoneInHMYProtocolTheirHeight(conns)

	return nil
}

type syncingHandler struct {
	rawStream libp2p_network.Stream
	host      *p2p.Host
	rw        *bufio.ReadWriter
	current   chan chainHeights
}

var (
	requestBlockHeight []byte
	requestBlockHeader []byte
	requestBlock       []byte
)

func init() {
	allMessage := make([][]byte, 3)

	for i, m := range [...]msg_pb.MessageType{
		msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT,
		msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEADER,
		msg_pb.MessageType_SYNC_REQUEST_BLOCK,
	} {
		message := &msg_pb.Message{
			ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
			Type:        m,
		}
		data, err := constructHarmonyP2PMsgForClientRoles(message)
		if err != nil {
			panic("die ->" + err.Error())
		}
		allMessage[i] = data
	}

	requestBlockHeight = allMessage[0]
	requestBlockHeader = allMessage[1]
	requestBlock = allMessage[2]
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

func (sync *syncingHandler) receiveHeightResponse() (*chainHeights, error) {
	resp, err := sync.readHarmonyMessage()
	if err != nil {
		return nil, err
	}

	if r := resp.GetServiceType(); r != msg_pb.ServiceType_CLIENT_SUPPORT {
		return nil, errors.Wrapf(
			errNotClientSupport, "wrongly received %v", r,
		)
	}

	if r := resp.GetType(); r != msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT {
		return nil, errors.Wrapf(
			errWrongReply, "wrongly received %v", r,
		)
	}

	peerHeights := resp.GetSyncBlockHeight()
	return &chainHeights{
		beacon: height{
			blockNum:  peerHeights.GetBeaconHeight(),
			blockHash: common.BytesToHash(peerHeights.GetBeaconHash()),
		},
		shard: height{
			blockNum:  peerHeights.GetShardHeight(),
			blockHash: common.BytesToHash(peerHeights.GetShardHash()),
		},
	}, nil

}

func (sync *syncingHandler) askHeight() (*chainHeights, error) {
	if _, err := sync.rw.Write(requestBlockHeight); err != nil {
		return nil, err
	}
	if err := sync.rw.Flush(); err != nil {
		return nil, err
	}

	return sync.receiveHeightResponse()
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

	if err := sync.rw.Flush(); err != nil {
		return err
	}

	return nil

}

func (sync *syncingHandler) replyWithCurrentBlockHeight() error {

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT,
		Request: &msg_pb.Message_SyncBlockHeight{
			SyncBlockHeight: &msg_pb.SyncBlockHeight{
				BeaconHeight: 1,
				BeaconHash:   []byte{},
				ShardHeight:  2,
				ShardHash:    []byte{},
			},
		},
	}

	data, err := constructHarmonyP2PMsgForClientRoles(message)
	if err != nil {
		return err
	}

	return sync.sendBytesWithFlush(data)
}

func (sync *syncingHandler) handleMessage(msg *msg_pb.Message) error {
	if msg.GetServiceType() != msg_pb.ServiceType_CLIENT_SUPPORT {
		return errNotClientSupport
	}

	if msg.GetType() == msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT {
		fmt.Printf("\x1b[32m%s\x1b[0m> ", msg.String())
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

// HandleBlockSyncing ..
func (node *Node) HandleBlockSyncing() error {
	t := time.NewTicker(time.Second * 10)
	defer t.Stop()

	select {
	case blockRange := <-node.Consensus.SyncNeeded:
		_ = blockRange
	case <-t.C:
		if err := syncFromHMYPeersIfNeeded(
			node.host,
			height{
				node.Beaconchain().CurrentHeader().Number().Uint64(),
				node.Beaconchain().CurrentHeader().Hash(),
			},
			height{
				node.Blockchain().CurrentHeader().Number().Uint64(),
				node.Blockchain().CurrentHeader().Hash(),
			},
		); err != nil {
			return err
		}

	}

	return nil
}

// HandleIncomingHMYProtocolStreams ..
func (node *Node) HandleIncomingHMYProtocolStreams() error {
	errs := make(chan error)
	info := make(chan chainHeights)
	incoming := 0

	for stream := range node.host.IncomingStream {
		handler, err := newSyncerForIncoming(
			stream, node.host, info,
		)
		if err != nil {
			return err
		}

		incoming++
		fmt.Println(
			"incoming so far",
			incoming,
			node.host.OwnPeer.ConsensusPubKey.SerializeToHexStr(),
		)
		errs <- handler.processIncoming()

	}

	for err := range errs {
		utils.Logger().Info().Err(err).
			Msg("incoming stream handling had some problem")
	}

	return nil
}
