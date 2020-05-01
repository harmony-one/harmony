package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
)

func newSyncer(
	host *p2p.Host,
	peerID libp2p_peer.ID,
) (*syncingHandler, error) {

	stateSyncStream, err := host.IPFSNode.PeerHost.NewStream(
		context.Background(),
		peerID,
		p2p.Protocol,
	)

	if err != nil {
		return nil, err
	}

	rw := bufio.NewReadWriter(
		bufio.NewReader(stateSyncStream), bufio.NewWriter(stateSyncStream),
	)

	// TODO probably need to expose the raw stream as well
	return &syncingHandler{
		peerID, host, rw,
	}, nil

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
			fmt.Println("here the protocols", protocols)
			return nil
		}

		handler, err := newSyncer(host, id)

		if err != nil {
			return err
		}

		beaconC, shardC, err := handler.askHeight()

		if err != nil {
			return err
		}

		fmt.Println("chain heights", beaconC, shardC)
	}
	// everyonesHeaderHashs := askEveryoneInHMYProtocolTheirHeight(conns)

	return nil
}

type syncingHandler struct {
	handlingFor libp2p_peer.ID
	host        *p2p.Host
	rw          *bufio.ReadWriter
}

func (sync *syncingHandler) askHeight() (*height, *height, error) {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_BLOCK_HEIGHT,
	}

	msg, err := protobuf.Marshal(message)

	if err != nil {
		return nil, nil, err
	}

	byteBuffer := bytes.NewBuffer([]byte{byte(proto_node.Client)})
	byteBuffer.Write(msg)
	syncingMessage := p2p.ConstructMessage(byteBuffer.Bytes())

	if _, err := sync.rw.Write(syncingMessage); err != nil {
		return nil, nil, err
	}

	if err := sync.rw.Flush(); err != nil {
		return nil, nil, err
	}

	return nil, nil, nil

}

func (sync *syncingHandler) processIncoming(s libp2p_network.Stream) error {
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	fmt.Println("something incoming->", rw)
	for {

		time.Sleep(time.Second * 2)
		// this must be our message
		buf, err := rw.Peek(1)
		if err != nil {
			return err
		}

		if buf[0] == 0x11 {

			_, err := rw.ReadByte()
			if err != nil {
				return err
			}

			var msgBuf msg_pb.Message
			contentSizeBuf := make([]byte, 4)

			if _, err := io.ReadFull(rw, contentSizeBuf); err != nil {
				return err
			}

			sized := binary.BigEndian.Uint32(contentSizeBuf)
			fmt.Println("sized", sized)

			payload := make([]byte, sized)

			if _, err := io.ReadFull(rw, payload); err != nil {
				return err
			}

			fmt.Println("what does the payload look like", hex.EncodeToString(payload))
			// 0x1100000005020803100e
			// 0x          020803100e

			if err := protobuf.Unmarshal(payload[1:], &msgBuf); err != nil {
				fmt.Println(err.Error())
			}
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", msgBuf.String())
		}
	}

	return nil
}

var (
	askBlockHeight = &downloader_pb.DownloaderRequest{
		Type: downloader_pb.DownloaderRequest_BLOCKHEIGHT,
	}
)

// NOTE maybe better named handle incoming request
// switch request.GetType() {
// case downloader_pb.DownloaderRequest_BLOCKHASH:
// case downloader_pb.DownloaderRequest_BLOCK:
// case downloader_pb.DownloaderRequest_NEWBLOCK:
// case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
// case downloader_pb.DownloaderRequest_REGISTER:
// case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
// case downloader_pb.DownloaderRequest_UNKNOWN:
// case downloader_pb.DownloaderRequest_BLOCKHEADER:
// response := &downloader_pb.DownloaderResponse{}
// blk := <-s.currentBlockHeight
// response.BlockHeight = blk.NumberU64()
// return response, nil

// block downloading consumer side pipeline

// allBlockHeightsAndHashOfNeighbors <- askEveryoneInHMYProtocolTheirHeight()
// for each block := range downloadedBlocksFromPeersWithMostCommonHash <- previouslyDownloaded()
//

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

	for stream := range node.host.IncomingStream {
		handler, err := newSyncer(
			node.host,
			stream.Conn().RemotePeer(),
		)
		if err != nil {
			return err
		}

		go func() {
			errs <- handler.processIncoming(stream)
		}()

	}

	go func() {
		for err := range errs {
			utils.Logger().Info().Err(err).
				Msg("incoming stream handling had some problem")
		}
	}()

	return nil
}
