package sync

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	protobuf "google.golang.org/protobuf/proto"
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
		select {
		case <-st.closeC:
			return
		default:
			msg, err := st.readMsg()
			if err != nil {
				if err := st.Close("read msg failed", false); err != nil {
					st.logger.Err(err).Msg("failed to close sync stream")
				}
				return
			}
			if msg != nil {
				st.deliverMsg(msg)
			}
		}
	}
}

// deliverMsg process the delivered message and forward to the corresponding channel
func (st *syncStream) deliverMsg(msg protobuf.Message) {
	syncMsg := msg.(*syncpb.Message)
	if syncMsg == nil {
		st.logger.Info().Interface("message", msg).Msg("received unexpected sync message")
		return
	}
	if req := syncMsg.GetReq(); req != nil {
		go func() {
			select {
			case st.reqC <- req:
			case <-time.After(1 * time.Minute):
				st.logger.Warn().Str("request", req.String()).
					Msg("request handler severely jammed, message dropped")
			case <-st.closeC:
				return
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
			case <-st.closeC:
				return
			}
		}()
	}
	return
}

// handleReqLoop replies to incoming requests
func (st *syncStream) handleReqLoop() {
	for {
		select {
		case req := <-st.reqC:
			st.protocol.rl.LimitRequest(st.ID())
			err := st.handleReq(req)

			if err != nil {
				st.logger.Info().Err(err).Str("request", req.String()).
					Msg("handle request error. Closing stream")
				if err := st.Close("handle request error", false); err != nil {
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
func (st *syncStream) Close(reason string, criticalErr bool) error {
	notClosed := atomic.CompareAndSwapUint32(&st.closeStat, 0, 1)
	if !notClosed {
		// Already closed by another goroutine. Directly return
		return nil
	}
	if err := st.protocol.sm.RemoveStream(st.ID(), "force close: "+reason, criticalErr); err != nil {
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
	if err := st.BaseStream.Close(); err != nil {
		//TODO: log closure error
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
	if ndReq := req.GetGetNodeDataRequest(); ndReq != nil {
		return st.handleGetNodeDataRequest(req.ReqId, ndReq)
	}
	if rReq := req.GetGetReceiptsRequest(); rReq != nil {
		return st.handleGetReceiptsRequest(req.ReqId, rReq)
	}
	if ndReq := req.GetGetAccountRangeRequest(); ndReq != nil {
		return st.handleGetAccountRangeRequest(req.ReqId, ndReq)
	}
	if ndReq := req.GetGetStorageRangesRequest(); ndReq != nil {
		return st.handleGetStorageRangesRequest(req.ReqId, ndReq)
	}
	if ndReq := req.GetGetByteCodesRequest(); ndReq != nil {
		return st.handleGetByteCodesRequest(req.ReqId, ndReq)
	}
	if ndReq := req.GetGetTrieNodesRequest(); ndReq != nil {
		return st.handleGetTrieNodesRequest(req.ReqId, ndReq)
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

func (st *syncStream) handleGetNodeDataRequest(rid uint64, req *syncpb.GetNodeDataRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getNodeData",
	}).Inc()

	hashes := bytesToHashes(req.NodeHashes)
	resp, err := st.computeGetNodeData(rid, hashes)
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
	return errors.Wrap(err, "[GetNodeData]")
}

func (st *syncStream) handleGetReceiptsRequest(rid uint64, req *syncpb.GetReceiptsRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getReceipts",
	}).Inc()

	hashes := bytesToHashes(req.BlockHashes)
	resp, err := st.computeGetReceipts(rid, hashes)
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
	return errors.Wrap(err, "[GetReceipts]")
}

func (st *syncStream) handleGetAccountRangeRequest(rid uint64, req *syncpb.GetAccountRangeRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getAccountRangeRequest",
	}).Inc()

	root := common.BytesToHash(req.Root)
	origin := common.BytesToHash(req.Origin)
	limit := common.BytesToHash(req.Limit)
	resp, err := st.computeGetAccountRangeRequest(rid, root, origin, limit, req.Bytes)
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
	return errors.Wrap(err, "[GetAccountRange]")
}

func (st *syncStream) handleGetStorageRangesRequest(rid uint64, req *syncpb.GetStorageRangesRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getStorageRangesRequest",
	}).Inc()

	root := common.BytesToHash(req.Root)
	accounts := bytesToHashes(req.Accounts)
	origin := common.BytesToHash(req.Origin)
	limit := common.BytesToHash(req.Limit)
	resp, err := st.computeGetStorageRangesRequest(rid, root, accounts, origin, limit, req.Bytes)
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
	return errors.Wrap(err, "[GetStorageRanges]")
}

func (st *syncStream) handleGetByteCodesRequest(rid uint64, req *syncpb.GetByteCodesRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getByteCodesRequest",
	}).Inc()

	hashes := bytesToHashes(req.Hashes)
	resp, err := st.computeGetByteCodesRequest(rid, hashes, req.Bytes)
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
	return errors.Wrap(err, "[GetByteCodes]")
}

func (st *syncStream) handleGetTrieNodesRequest(rid uint64, req *syncpb.GetTrieNodesRequest) error {
	serverRequestCounterVec.With(prometheus.Labels{
		"topic":        string(st.ProtoID()),
		"request_type": "getTrieNodesRequest",
	}).Inc()

	root := common.BytesToHash(req.Root)
	resp, err := st.computeGetTrieNodesRequest(rid, root, req.Paths, req.Bytes)
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
	return errors.Wrap(err, "[GetTrieNodes]")
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
	if b == nil || len(b) == 0 {
		return nil, nil
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

func (st *syncStream) computeGetNodeData(rid uint64, hs []common.Hash) (*syncpb.Message, error) {
	if len(hs) > GetNodeDataCap {
		err := fmt.Errorf("GetNodeData amount exceed cap: %v > %v", len(hs), GetNodeDataCap)
		return nil, err
	}
	data, err := st.chain.getNodeData(hs)
	if err != nil {
		return nil, err
	}

	return syncpb.MakeGetNodeDataResponseMessage(rid, data), nil
}

func (st *syncStream) computeGetReceipts(rid uint64, hs []common.Hash) (*syncpb.Message, error) {
	if len(hs) > GetReceiptsCap {
		err := fmt.Errorf("GetReceipts amount exceed cap: %v > %v", len(hs), GetReceiptsCap)
		return nil, err
	}
	receipts, err := st.chain.getReceipts(hs)
	if err != nil {
		return nil, err
	}
	var normalizedReceipts = make(map[uint64]*syncpb.Receipts, len(receipts))
	for i, blkReceipts := range receipts {
		normalizedReceipts[uint64(i)] = &syncpb.Receipts{
			ReceiptBytes: make([][]byte, 0),
		}
		for _, receipt := range blkReceipts {
			receiptBytes, err := rlp.EncodeToBytes(receipt)
			if err != nil {
				return nil, err
			}
			normalizedReceipts[uint64(i)].ReceiptBytes = append(normalizedReceipts[uint64(i)].ReceiptBytes, receiptBytes)
		}
	}
	return syncpb.MakeGetReceiptsResponseMessage(rid, normalizedReceipts), nil
}

func (st *syncStream) computeGetAccountRangeRequest(rid uint64, root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) (*syncpb.Message, error) {
	if bytes == 0 {
		return nil, fmt.Errorf("zero account ranges bytes requested")
	}
	if bytes > softResponseLimit {
		return nil, fmt.Errorf("requested bytes exceed limit")
	}
	accounts, proof, err := st.chain.getAccountRange(root, origin, limit, bytes)
	if err != nil {
		return nil, err
	}
	return syncpb.MakeGetAccountRangeResponseMessage(rid, accounts, proof), nil
}

func (st *syncStream) computeGetStorageRangesRequest(rid uint64, root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) (*syncpb.Message, error) {
	if bytes == 0 {
		return nil, fmt.Errorf("zero storage ranges bytes requested")
	}
	if bytes > softResponseLimit {
		return nil, fmt.Errorf("requested bytes exceed limit")
	}
	if len(accounts) > GetStorageRangesRequestCap {
		err := fmt.Errorf("GetStorageRangesRequest amount exceed cap: %v > %v", len(accounts), GetStorageRangesRequestCap)
		return nil, err
	}
	slots, proofs, err := st.chain.getStorageRanges(root, accounts, origin, limit, bytes)
	if err != nil {
		return nil, err
	}
	return syncpb.MakeGetStorageRangesResponseMessage(rid, slots, proofs), nil
}

func (st *syncStream) computeGetByteCodesRequest(rid uint64, hs []common.Hash, bytes uint64) (*syncpb.Message, error) {
	if bytes == 0 {
		return nil, fmt.Errorf("zero byte code bytes requested")
	}
	if bytes > softResponseLimit {
		return nil, fmt.Errorf("requested bytes exceed limit")
	}
	if len(hs) > GetByteCodesRequestCap {
		err := fmt.Errorf("GetByteCodesRequest amount exceed cap: %v > %v", len(hs), GetByteCodesRequestCap)
		return nil, err
	}
	codes, err := st.chain.getByteCodes(hs, bytes)
	if err != nil {
		return nil, err
	}
	return syncpb.MakeGetByteCodesResponseMessage(rid, codes), nil
}

func (st *syncStream) computeGetTrieNodesRequest(rid uint64, root common.Hash, paths []*message.TrieNodePathSet, bytes uint64) (*syncpb.Message, error) {
	if bytes == 0 {
		return nil, fmt.Errorf("zero trie node bytes requested")
	}
	if bytes > softResponseLimit {
		return nil, fmt.Errorf("requested bytes exceed limit")
	}
	if len(paths) > GetTrieNodesRequestCap {
		err := fmt.Errorf("GetTrieNodesRequest amount exceed cap: %v > %v", len(paths), GetTrieNodesRequestCap)
		return nil, err
	}
	nodes, err := st.chain.getTrieNodes(root, paths, bytes, time.Now())
	if err != nil {
		return nil, err
	}
	return syncpb.MakeGetTrieNodesResponseMessage(rid, nodes), nil
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
