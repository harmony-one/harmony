package sync

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// syncStream is the structure for a stream running sync protocol.
type syncStream struct {
	// Basic stream
	*sttypes.BaseStream

	protocol *Protocol
	chain    chainHelper

	// pipeline channels
	reqC  chan *syncpb.Request
	respC chan *syncpb.Response

	// close related fields. Concurrent call of close is possible.
	closeC    chan struct{}
	closeStat uint32

	logger zerolog.Logger
}

// wrapStream wraps the raw libp2p stream to syncStream
func (p *Protocol) wrapStream(raw libp2p_network.Stream) *syncStream {
	bs := sttypes.NewBaseStream(raw)
	logger := p.logger.With().
		Str("ID", string(bs.ID())).
		Str("Remote Protocol", string(bs.ProtoID())).
		Logger()

	return &syncStream{
		BaseStream: bs,
		protocol:   p,
		chain:      newChainHelper(p.chain, p.schedule),
		reqC:       make(chan *syncpb.Request, 100),
		respC:      make(chan *syncpb.Response, 100),
		closeC:     make(chan struct{}),
		closeStat:  0,
		logger:     logger,
	}
}

func (st *syncStream) run() {
	st.logger.Info().Str("StreamID", string(st.ID())).Msg("running sync protocol on stream")
	defer st.logger.Info().Str("StreamID", string(st.ID())).Msg("end running sync protocol on stream")

	go st.handleReqLoop()
	go st.handleRespLoop()
	st.readMsgLoop()
}

// readMsgLoop is the loop
func (st *syncStream) readMsgLoop() {
	for {
		msg, err := st.readMsg()
		if err != nil {
			if err := st.Close(); err != nil {
				st.logger.Err(err).Msg("failed to close sync stream")
			}
			return
		}
		st.deliverMsg(msg)
	}
}

// deliverMsg process the delivered message and forward to the corresponding channel
func (st *syncStream) deliverMsg(msg protobuf.Message) {
	syncMsg := msg.(*syncpb.Message)
	if syncMsg == nil {
		st.logger.Info().Str("message", msg.String()).Msg("received unexpected sync message")
		return
	}
	if req := syncMsg.GetReq(); req != nil {
		go func() {
			select {
			case st.reqC <- req:
			case <-time.After(1 * time.Minute):
				st.logger.Warn().Str("request", req.String()).
					Msg("request handler severely jammed, message dropped")
			}
		}()
	}
	if resp := syncMsg.GetResp(); resp != nil {
		go func() {
			select {
			case st.respC <- resp:
			case <-time.After(1 * time.Minute):
				st.logger.Warn().Str("response", resp.String()).
					Msg("response handler severely jammed, message dropped")
			}
		}()
	}
	return
}

func (st *syncStream) handleReqLoop() {
	for {
		select {
		case req := <-st.reqC:
			st.protocol.rl.LimitRequest(st.ID())
			err := st.handleReq(req)

			if err != nil {
				st.logger.Info().Err(err).Str("request", req.String()).
					Msg("handle request error. Closing stream")
				if err := st.Close(); err != nil {
					st.logger.Err(err).Msg("failed to close sync stream")
				}
				return
			}

		case <-st.closeC:
			return
		}
	}
}

func (st *syncStream) handleRespLoop() {
	for {
		select {
		case resp := <-st.respC:
			st.handleResp(resp)

		case <-st.closeC:
			return
		}
	}
}

// Close stops the stream handling and closes the underlying stream
func (st *syncStream) Close() error {
	notClosed := atomic.CompareAndSwapUint32(&st.closeStat, 0, 1)
	if !notClosed {
		// Already closed by another goroutine. Directly return
		return nil
	}
	if err := st.protocol.sm.RemoveStream(st.ID()); err != nil {
		st.logger.Err(err).Str("stream ID", string(st.ID())).
			Msg("failed to remove sync stream on close")
	}
	close(st.closeC)
	return st.BaseStream.Close()
}

// CloseOnExit reset the stream on exiting node
func (st *syncStream) CloseOnExit() error {
	notClosed := atomic.CompareAndSwapUint32(&st.closeStat, 0, 1)
	if !notClosed {
		// Already closed by another goroutine. Directly return
		return nil
	}
	close(st.closeC)
	return st.BaseStream.CloseOnExit()
}

func (st *syncStream) handleReq(req *syncpb.Request) error {
	if gnReq := req.GetGetBlockNumberRequest(); gnReq != nil {
		return st.handleGetBlockNumberRequest(req.ReqId)
	}
	if ghReq := req.GetGetBlockHashesRequest(); ghReq != nil {
		return st.handleGetBlockHashesRequest(req.ReqId, ghReq)
	}
	if bnReq := req.GetGetBlocksByNumRequest(); bnReq != nil {
		return st.handleGetBlocksByNumRequest(req.ReqId, bnReq)
	}
	if bhReq := req.GetGetBlocksByHashesRequest(); bhReq != nil {
		return st.handleGetBlocksByHashesRequest(req.ReqId, bhReq)
	}
	// unsupported request type
	return st.handleUnknownRequest(req.ReqId)
}

func (st *syncStream) handleGetBlockNumberRequest(rid uint64) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getBlockNumber",
	}).Inc()

	resp := st.computeBlockNumberResp(rid)
	if err := st.writeMsg(resp); err != nil {
		return errors.Wrap(err, "[GetBlockNumber]: writeMsg")
	}
	return nil
}

func (st *syncStream) handleGetBlockHashesRequest(rid uint64, req *syncpb.GetBlockHashesRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getBlockHashes",
	}).Inc()

	resp, err := st.computeGetBlockHashesResp(rid, req.Nums)
	if err != nil {
		resp = syncpb.MakeErrorResponseMessage(rid, err)
	}
	if writeErr := st.writeMsg(resp); writeErr != nil {
		if err == nil {
			err = writeErr
		} else {
			err = fmt.Errorf("%v; [writeMsg] %v", err.Error(), writeErr)
		}
	}
	return errors.Wrap(err, "[GetBlockHashes]")
}

func (st *syncStream) handleGetBlocksByNumRequest(rid uint64, req *syncpb.GetBlocksByNumRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getBlocksByNumber",
	}).Inc()

	resp, err := st.computeRespFromBlockNumber(rid, req.Nums)
	if resp == nil && err != nil {
		resp = syncpb.MakeErrorResponseMessage(rid, err)
	}
	if writeErr := st.writeMsg(resp); writeErr != nil {
		if err == nil {
			err = writeErr
		} else {
			err = fmt.Errorf("%v; [writeMsg] %v", err.Error(), writeErr)
		}
	}
	return errors.Wrap(err, "[GetBlocksByNumber]")
}

func (st *syncStream) handleGetBlocksByHashesRequest(rid uint64, req *syncpb.GetBlocksByHashesRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getBlocksByHashes",
	}).Inc()

	hashes := bytesToHashes(req.BlockHashes)
	resp, err := st.computeRespFromBlockHashes(rid, hashes)
	if resp == nil && err != nil {
		resp = syncpb.MakeErrorResponseMessage(rid, err)
	}
	if writeErr := st.writeMsg(resp); writeErr != nil {
		if err == nil {
			err = writeErr
		} else {
			err = fmt.Errorf("%v; [writeMsg] %v", err.Error(), writeErr)
		}
	}
	return errors.Wrap(err, "[GetBlocksByHashes]")
}

func (st *syncStream) handleUnknownRequest(rid uint64) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "unknown",
	}).Inc()
	resp := syncpb.MakeErrorResponseMessage(rid, errUnknownReqType)
	return st.writeMsg(resp)
}

func (st *syncStream) handleResp(resp *syncpb.Response) {
	st.protocol.rm.DeliverResponse(st.ID(), &syncResponse{resp})
}

func (st *syncStream) readMsg() (*syncpb.Message, error) {
	b, err := st.ReadBytes()
	if err != nil {
		return nil, err
	}
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (st *syncStream) writeMsg(msg *syncpb.Message) error {
	b, err := protobuf.Marshal(msg)
	if err != nil {
		return err
	}
	return st.WriteBytes(b)
}

func (st *syncStream) computeBlockNumberResp(rid uint64) *syncpb.Message {
	bn := st.chain.getCurrentBlockNumber()
	return syncpb.MakeGetBlockNumberResponseMessage(rid, bn)
}

func (st *syncStream) computeGetBlockHashesResp(rid uint64, bns []uint64) (*syncpb.Message, error) {
	if len(bns) > GetBlockHashesAmountCap {
		err := fmt.Errorf("GetBlockHashes amount exceed cap: %v>%v", len(bns), GetBlockHashesAmountCap)
		return nil, err
	}
	hashes := st.chain.getBlockHashes(bns)
	return syncpb.MakeGetBlockHashesResponseMessage(rid, hashes), nil
}

func (st *syncStream) computeRespFromBlockNumber(rid uint64, bns []uint64) (*syncpb.Message, error) {
	if len(bns) > GetBlocksByNumAmountCap {
		err := fmt.Errorf("GetBlocksByNum amount exceed cap: %v>%v", len(bns), GetBlocksByNumAmountCap)
		return nil, err
	}
	blocks, err := st.chain.getBlocksByNumber(bns)
	if err != nil {
		return nil, err
	}

	var (
		blocksBytes = make([][]byte, 0, len(blocks))
		sigs        = make([][]byte, 0, len(blocks))
	)
	for _, block := range blocks {
		bb, err := rlp.EncodeToBytes(block)
		if err != nil {
			return nil, err
		}
		blocksBytes = append(blocksBytes, bb)

		var sig []byte
		if block != nil {
			sig = block.GetCurrentCommitSig()
		}
		sigs = append(sigs, sig)
	}
	return syncpb.MakeGetBlocksByNumResponseMessage(rid, blocksBytes, sigs), nil
}

func (st *syncStream) computeRespFromBlockHashes(rid uint64, hs []common.Hash) (*syncpb.Message, error) {
	if len(hs) > GetBlocksByHashesAmountCap {
		err := fmt.Errorf("GetBlockByHashes amount exceed cap: %v > %v", len(hs), GetBlocksByHashesAmountCap)
		return nil, err
	}
	blocks, err := st.chain.getBlocksByHashes(hs)
	if err != nil {
		return nil, err
	}

	var (
		blocksBytes = make([][]byte, 0, len(blocks))
		sigs        = make([][]byte, 0, len(blocks))
	)
	for _, block := range blocks {
		bb, err := rlp.EncodeToBytes(block)
		if err != nil {
			return nil, err
		}
		blocksBytes = append(blocksBytes, bb)

		var sig []byte
		if block != nil {
			sig = block.GetCurrentCommitSig()
		}
		sigs = append(sigs, sig)
	}
	return syncpb.MakeGetBlocksByHashesResponseMessage(rid, blocksBytes, sigs), nil
}

func bytesToHashes(bs [][]byte) []common.Hash {
	hs := make([]common.Hash, 0, len(bs))
	for _, b := range bs {
		var h common.Hash
		copy(h[:], b)
		hs = append(hs, h)
	}
	return hs
}
