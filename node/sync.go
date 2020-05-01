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
	current chan latestInfo,
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
	current chan latestInfo,
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

		beaconC, shardC, err := handler.askHeight()

		if err != nil {
			return err
		}

		fmt.Println(beaconC, shardC, err)
	}
	// everyonesHeaderHashs := askEveryoneInHMYProtocolTheirHeight(conns)

	return nil
}

type latestInfo struct {
	beacon *height
	shard  *height
}

type syncingHandler struct {
	rawStream libp2p_network.Stream
	host      *p2p.Host
	rw        *bufio.ReadWriter
	current   chan latestInfo
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
		msg, err := protobuf.Marshal(message)
		if err != nil {
			panic("die ->" + err.Error())
		}
		byteBuffer := bytes.NewBuffer([]byte{byte(proto_node.Client)})
		byteBuffer.Write(msg)
		allMessage[i] = p2p.ConstructMessage(byteBuffer.Bytes())
	}

	requestBlockHeight = allMessage[0]
	requestBlockHeader = allMessage[1]
	requestBlock = allMessage[2]
}

func (sync *syncingHandler) receiveHeightResponse() (*height, *height, error) {
	return nil, nil, nil
}

func (sync *syncingHandler) askHeight() (*height, *height, error) {
	if _, err := sync.rw.Write(requestBlockHeight); err != nil {
		return nil, nil, err
	}
	if err := sync.rw.Flush(); err != nil {
		return nil, nil, err
	}

	return sync.receiveHeightResponse()
}

var (
	errNotClientSupport   = errors.New("not a client support message")
	errNotAHarmonyMessage = errors.New(
		"incoming message from stream did not begin with 0x11",
	)
)

func (sync *syncingHandler) replyWithCurrentBlockHeight() error {
	fmt.Println("need to give reply")
	return nil
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

func (sync *syncingHandler) processIncoming() error {
	// this must be our message
	buf, err := sync.rw.Peek(1)
	if err != nil {
		return err
	}

	if buf[0] == 0x11 {
		_, err := sync.rw.ReadByte()
		if err != nil {
			return err
		}

		var msgBuf msg_pb.Message
		contentSizeBuf := make([]byte, 4)

		if _, err := io.ReadFull(sync.rw, contentSizeBuf); err != nil {
			return err
		}

		sized := binary.BigEndian.Uint32(contentSizeBuf)
		payload := make([]byte, sized)

		if _, err := io.ReadFull(sync.rw, payload); err != nil {
			return err
		}

		// That one extra middle offset byte --
		if err := protobuf.Unmarshal(payload[1:], &msgBuf); err != nil {
			return err
		}

		return sync.handleMessage(&msgBuf)
	}

	return errNotAHarmonyMessage
}

type height struct {
	blockNum  uint64
	blockHash common.Hash
}

// HandleBlockSyncing ..
func (node *Node) HandleBlockSyncing() error {
	t := time.NewTicker(time.Second * 30)
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
	info := make(chan latestInfo)
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
