package node

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	ipfs_interface "github.com/ipfs/interface-go-ipfs-core"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-msgio"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func protocolPeerHeights(
	ctx context.Context,
	conns []ipfs_interface.ConnectionInfo,
	host *p2p.Host,
	node *Node,
) (map[libp2p_peer.ID]*msg_pb.Message, error) {
	hmyPeers := make(chan libp2p_peer.ID)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(hmyPeers)

		for _, neighbor := range conns {
			id := neighbor.ID()
			protocols, err := host.IPFSNode.PeerHost.Peerstore().SupportsProtocols(
				id, p2p.Protocol,
			)
			if err != nil {
				return err
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			case hmyPeers <- id:
			}
		}

		return nil
	})

	type peerResp struct {
		id  libp2p_peer.ID
		msg *msg_pb.Message
	}

	collect := make(chan *peerResp)
	const nWorkers = 10
	workers := int32(nWorkers)
	for i := 0; i < nWorkers; i++ {
		g.Go(func() error {
			defer func() {
				// Last one out closes shop
				if atomic.AddInt32(&workers, -1) == 0 {
					close(collect)
				}
			}()

			for id := range hmyPeers {
				msgSender, err := node.messageSenderForPeer(ctx, id)
				if err != nil {
					return err
				}
				if rpmes, err := msgSender.SendRequest(ctx, &msg_pb.Message{
					ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
					Type:        msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT,
				}); err != nil {
					return err
				} else {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case collect <- &peerResp{id, rpmes}:
					}
				}
			}
			return nil
		})
	}

	reduce := map[libp2p_peer.ID]*msg_pb.Message{}
	g.Go(func() error {
		for resp := range collect {
			reduce[resp.id] = resp.msg
		}
		return nil
	})

	return reduce, g.Wait()
}

type t struct {
	count  int
	haveIt []libp2p_peer.ID
}

type hashCount struct {
	count       int
	hash        common.Hash
	peersWithIt []libp2p_peer.ID
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

func commonHash(collect map[libp2p_peer.ID]*msg_pb.Message) mostCommonHash {

	beaconCounters, shardCounters :=
		map[common.Hash]t{}, map[common.Hash]t{}

	for peerID, c := range collect {
		height := c.GetSyncBlockHeight()
		shardHash := common.BytesToHash(height.GetShardHash())
		beaconHash := common.BytesToHash(height.GetBeaconHash())

		currentS := shardCounters[shardHash]
		currentS.count++
		currentS.haveIt = append(currentS.haveIt, peerID)
		shardCounters[shardHash] = currentS

		currentB := beaconCounters[beaconHash]
		currentB.count++
		currentB.haveIt = append(currentB.haveIt, peerID)
		beaconCounters[beaconHash] = currentB
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
		beacon: hashCount{b[0].count, b[0].hash, b[0].haveIt},
		shard:  hashCount{s[0].count, s[0].hash, s[0].haveIt},
	}
}

func syncFromHMYPeersIfNeeded(
	ctx context.Context, host *p2p.Host, node *Node,
) error {
	conns, err := host.CoreAPI.Swarm().Peers(context.Background())

	if err != nil {
		return err
	}

	// NOTE keeping it below 5 because checking all conns can eat lots of resources
	collect, err := protocolPeerHeights(ctx, conns[:5], host, node)
	if err != nil {
		return err
	}

	if len(collect) == 0 {
		return nil
	}

	_ = commonHash(collect)

	utils.Logger().Info().
		Int("protocol-peers", len(collect)).
		Msg("hmy protocol neighborse")

	return nil
}

// HandleBlockSyncing ..
func (node *Node) HandleBlockSyncing() error {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()

	for {
		select {
		case blockRange := <-node.Consensus.SyncNeeded:
			_ = blockRange
		case <-t.C:
			if err := func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return syncFromHMYPeersIfNeeded(ctx, node.host, node)
			}(); err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				return err
			}
		}
	}

	return nil
}

func (node *Node) handleNewMessage(s libp2p_network.Stream) error {
	r := msgio.NewVarintReaderSize(s, libp2p_network.MessageSizeMax)
	mPeer := s.Conn().RemotePeer()
	timer := time.AfterFunc(dhtStreamIdleTimeout, func() { s.Reset() })
	defer timer.Stop()

	for {
		var req msg_pb.Message
		msgbytes, err := r.ReadMsg()

		if err != nil {
			defer r.ReleaseMsg(msgbytes)
			if err == io.EOF {
				return nil
			}
			// This string test is necessary because there isn't a single stream reset error
			// instance	in use.
			if err.Error() != "stream reset" {
				utils.Logger().Info().Err(err).Msgf("error reading message")
			}

			return err
		}
		if err := protobuf.Unmarshal(msgbytes, &req); err != nil {
			return err
		}

		r.ReleaseMsg(msgbytes)
		timer.Reset(dhtStreamIdleTimeout)

		handler := node.syncHandlerForMsgType(req.GetType())

		if handler == nil {
			utils.Logger().Warn().
				Msgf("can't handle received message", "from", mPeer, "type", req.GetType())
			return errors.New("cant receive this message")
		}

		resp, err := handler(context.Background(), mPeer, &req)
		if err != nil {
			return err
		}
		if resp == nil {
			continue
		}
		if err := writeMsg(s, resp); err != nil {
			return err
		}
	}

	return nil

}

func (node *Node) handleNewStream(s libp2p_network.Stream) {
	defer s.Reset()
	if err := node.handleNewMessage(s); err != nil {
		utils.Logger().Warn().Err(err).Msg("stream had possible issue")
		return
	}
	_ = s.Close()
}

// HandleIncomingHMYProtocolStreams ..
func (node *Node) HandleIncomingHMYProtocolStreams() {
	node.host.IPFSNode.PeerHost.SetStreamHandler(
		p2p.Protocol, node.handleNewStream,
	)
}

func (node *Node) messageSenderForPeer(
	ctx context.Context, p libp2p_peer.ID,
) (*messageSender, error) {

	node.sender.Lock()
	ms, ok := node.sender.strmap[p]
	if ok {
		node.sender.Unlock()
		return ms, nil
	}
	ms = &messageSender{p: p, host: node.host}
	node.sender.strmap[p] = ms
	node.sender.Unlock()

	if err := ms.prepOrInvalidate(ctx); err != nil {
		node.sender.Lock()
		defer node.sender.Unlock()

		if msCur, ok := node.sender.strmap[p]; ok {
			// Changed. Use the new one, old one is invalid and
			// not in the map so we can just throw it away.
			if ms != msCur {
				return msCur, nil
			}
			// Not changed, remove the now invalid stream from the
			// map.
			delete(node.sender.strmap, p)
		}
		// Invalid but not in map. Must have been removed by a disconnect.
		return nil, err
	}
	// All ready to go.
	return ms, nil

}

type syncHandler func(
	context.Context, libp2p_peer.ID, *msg_pb.Message,
) (*msg_pb.Message, error)

func (node *Node) syncRespBlockHeightHandler(
	ctx context.Context, peer libp2p_peer.ID, msg *msg_pb.Message,
) (*msg_pb.Message, error) {

	beaconHeader := node.Beaconchain().CurrentHeader()
	shardHeader := node.Blockchain().CurrentHeader()

	return &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEIGHT,
		Request: &msg_pb.Message_SyncBlockHeight{
			SyncBlockHeight: &msg_pb.SyncBlockHeight{
				BeaconHeight: beaconHeader.Number().Uint64(),
				BeaconHash:   beaconHeader.Hash().Bytes(),
				ShardHeight:  shardHeader.Number().Uint64(),
				ShardHash:    shardHeader.Hash().Bytes(),
			},
		},
	}, nil
}

func (node *Node) syncRespBlockHeaderHandler(
	ctx context.Context, peer libp2p_peer.ID, msg *msg_pb.Message,
) (*msg_pb.Message, error) {
	start, end :=
		msg.GetSyncBlockHeader().GetHeightStart(),
		msg.GetSyncBlockHeader().GetHeightEnd()
	latest := node.Blockchain().CurrentHeader().Number().Uint64()

	if start > latest {
		return nil, nil
	}

	if end > latest {
		end = latest
	}

	headers := make([]*block.Header, start-end)

	for i := start; i < end; i++ {
		headers[i] = node.Blockchain().GetHeaderByNumber(i)
	}

	headersData, err := rlp.EncodeToBytes(headers)

	if err != nil {
		return nil, err
	}

	return &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_RESPONSE_BLOCK_HEADER,
		Request: &msg_pb.Message_SyncBlockHeader{
			SyncBlockHeader: &msg_pb.SyncBlockHeader{
				HeightStart: start,
				HeightEnd:   end,
				HeaderRlp:   headersData,
			},
		},
	}, nil
}

func (node *Node) syncRespBlockHandler(
	ctx context.Context, peer libp2p_peer.ID, msg *msg_pb.Message,
) (*msg_pb.Message, error) {
	start, end :=
		msg.GetSyncBlock().GetHeightStart(), msg.GetSyncBlock().GetHeightEnd()
	latest := node.Blockchain().CurrentHeader().Number().Uint64()

	if start > latest {
		return nil, nil
	}

	if end > latest {
		end = latest
	}

	blocks := make([]*types.Block, start-end)

	for i := start; i < end; i++ {
		blocks[i] = node.Blockchain().GetBlockByNumber(i)
	}

	blocksData, err := rlp.EncodeToBytes(blocks)

	if err != nil {
		return nil, err
	}

	return &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CLIENT_SUPPORT,
		Type:        msg_pb.MessageType_SYNC_RESPONSE_BLOCK,
		Request: &msg_pb.Message_SyncBlock{
			SyncBlock: &msg_pb.SyncBlock{
				HeightStart: start,
				HeightEnd:   end,
				BlockRlp:    blocksData,
			},
		},
	}, nil
}

func (node *Node) syncHandlerForMsgType(t msg_pb.MessageType) syncHandler {
	switch t {

	case msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEIGHT:
		return node.syncRespBlockHeightHandler
	case msg_pb.MessageType_SYNC_REQUEST_BLOCK_HEADER:
		return node.syncRespBlockHeaderHandler
	case msg_pb.MessageType_SYNC_REQUEST_BLOCK:
		return node.syncRespBlockHandler
	}

	return nil
}
