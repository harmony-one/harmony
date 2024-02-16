package sync

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

// GetBlocksByNumber do getBlocksByNumberRequest through sync stream protocol.
// Return the block as result, target stream id, and error
func (p *Protocol) GetBlocksByNumber(ctx context.Context, bns []uint64, opts ...Option) (blocks []*types.Block, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getBlocksByNumber")
	defer p.doMetricPostClientRequest("getBlocksByNumber", err, timer)

	if len(bns) == 0 {
		err = fmt.Errorf("zero block numbers requested")
		return
	}
	if len(bns) > GetBlocksByNumAmountCap {
		err = fmt.Errorf("number of blocks exceed cap of %v", GetBlocksByNumAmountCap)
		return
	}

	req := newGetBlocksByNumberRequest(bns)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		// At this point, error can be context canceled, context timed out, or waiting queue
		// is already full.
		return
	}

	// Parse and return blocks
	blocks, err = req.getBlocksFromResponse(resp)
	return
}

func (p *Protocol) GetRawBlocksByNumber(ctx context.Context, bns []uint64, opts ...Option) (blockBytes [][]byte, sigBytes [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getBlocksByNumber")
	defer p.doMetricPostClientRequest("getBlocksByNumber", err, timer)

	if len(bns) == 0 {
		err = fmt.Errorf("zero block numbers requested")
		return
	}
	if len(bns) > GetBlocksByNumAmountCap {
		err = fmt.Errorf("number of blocks exceed cap of %v", GetBlocksByNumAmountCap)
		return
	}
	req := newGetBlocksByNumberRequest(bns)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		// At this point, error can be context canceled, context timed out, or waiting queue
		// is already full.
		return
	}

	// Parse and return blocks
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		err = errors.New("not sync response")
		return
	}
	blockBytes, sigBytes, err = req.parseBlockBytesAndSigs(sResp)
	return
}

// GetCurrentBlockNumber get the current block number from remote node
func (p *Protocol) GetCurrentBlockNumber(ctx context.Context, opts ...Option) (bn uint64, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getBlockNumber")
	defer p.doMetricPostClientRequest("getBlockNumber", err, timer)

	req := newGetBlockNumberRequest()

	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return 0, stid, err
	}

	bn, err = req.getNumberFromResponse(resp)
	return
}

// GetBlockHashes do getBlockHashesRequest through sync stream protocol.
// Return the hash of the given block number. If a block is unknown, the hash will be emptyHash.
func (p *Protocol) GetBlockHashes(ctx context.Context, bns []uint64, opts ...Option) (hashes []common.Hash, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getBlockHashes")
	defer p.doMetricPostClientRequest("getBlockHashes", err, timer)

	if len(bns) == 0 {
		err = fmt.Errorf("zero block numbers requested")
		return
	}
	if len(bns) > GetBlockHashesAmountCap {
		err = fmt.Errorf("number of requested numbers exceed limit")
		return
	}

	req := newGetBlockHashesRequest(bns)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	hashes, err = req.getHashesFromResponse(resp)
	return
}

// GetBlocksByHashes do getBlocksByHashesRequest through sync stream protocol.
func (p *Protocol) GetBlocksByHashes(ctx context.Context, hs []common.Hash, opts ...Option) (blocks []*types.Block, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getBlocksByHashes")
	defer p.doMetricPostClientRequest("getBlocksByHashes", err, timer)

	if len(hs) == 0 {
		err = fmt.Errorf("zero block hashes requested")
		return
	}
	if len(hs) > GetBlocksByHashesAmountCap {
		err = fmt.Errorf("number of requested hashes exceed limit")
		return
	}
	req := newGetBlocksByHashesRequest(hs)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	blocks, err = req.getBlocksFromResponse(resp)
	return
}

// GetReceipts do getReceiptsRequest through sync stream protocol.
// Return the receipts as result, target stream id, and error
func (p *Protocol) GetReceipts(ctx context.Context, hs []common.Hash, opts ...Option) (receipts []types.Receipts, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getReceipts")
	defer p.doMetricPostClientRequest("getReceipts", err, timer)

	if len(hs) == 0 {
		err = fmt.Errorf("zero receipt hashes requested")
		return
	}
	if len(hs) > GetReceiptsCap {
		err = fmt.Errorf("number of requested hashes exceed limit")
		return
	}
	req := newGetReceiptsRequest(hs)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	receipts, err = req.getReceiptsFromResponse(resp)
	return
}

// GetNodeData do getNodeData through sync stream protocol.
// Return the state node data as result, target stream id, and error
func (p *Protocol) GetNodeData(ctx context.Context, hs []common.Hash, opts ...Option) (data [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getNodeData")
	defer p.doMetricPostClientRequest("getNodeData", err, timer)

	if len(hs) == 0 {
		err = fmt.Errorf("zero node data hashes requested")
		return
	}
	if len(hs) > GetNodeDataCap {
		err = fmt.Errorf("number of requested hashes exceed limit")
		return
	}
	req := newGetNodeDataRequest(hs)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	data, err = req.getNodeDataFromResponse(resp)
	return
}

// GetAccountRange do getAccountRange through sync stream protocol.
// returns the accounts along with proofs as result, target stream id, and error
func (p *Protocol) GetAccountRange(ctx context.Context, root common.Hash, origin common.Hash, limit common.Hash, bytes uint64, opts ...Option) (accounts []*message.AccountData, proof [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getAccountRange")
	defer p.doMetricPostClientRequest("getAccountRange", err, timer)

	if bytes == 0 {
		err = fmt.Errorf("zero account ranges bytes requested")
		return
	}
	if bytes > softResponseLimit {
		err = fmt.Errorf("requested bytes exceed limit")
		return
	}
	req := newGetAccountRangeRequest(root, origin, limit, bytes)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	accounts, proof, err = req.getAccountRangeFromResponse(resp)
	return
}

// GetStorageRanges do getStorageRanges through sync stream protocol.
// returns the slots along with proofs as result, target stream id, and error
func (p *Protocol) GetStorageRanges(ctx context.Context, root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64, opts ...Option) (slots [][]*message.StorageData, proof [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getStorageRanges")
	defer p.doMetricPostClientRequest("getStorageRanges", err, timer)

	if bytes == 0 {
		err = fmt.Errorf("zero storage ranges bytes requested")
		return
	}
	if bytes > softResponseLimit {
		err = fmt.Errorf("requested bytes exceed limit")
		return
	}
	if len(accounts) > GetStorageRangesRequestCap {
		err = fmt.Errorf("number of requested accounts exceed limit")
		return
	}
	req := newGetStorageRangesRequest(root, accounts, origin, limit, bytes)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	var storages []*message.StoragesData
	storages, proof, err = req.getStorageRangesFromResponse(resp)
	if err != nil {
		return
	}
	slots = make([][]*message.StorageData, 0)
	for _, storage := range storages {
		slots = append(slots, storage.Data)
	}
	return
}

// GetByteCodes do getByteCodes through sync stream protocol.
// returns the codes as result, target stream id, and error
func (p *Protocol) GetByteCodes(ctx context.Context, hs []common.Hash, bytes uint64, opts ...Option) (codes [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getByteCodes")
	defer p.doMetricPostClientRequest("getByteCodes", err, timer)

	if bytes == 0 {
		err = fmt.Errorf("zero bytecode bytes requested")
		return
	}
	if bytes > softResponseLimit {
		err = fmt.Errorf("requested bytes exceed limit")
		return
	}
	if len(hs) > GetByteCodesRequestCap {
		err = fmt.Errorf("number of requested hashes exceed limit")
		return
	}
	req := newGetByteCodesRequest(hs, bytes)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	codes, err = req.getByteCodesFromResponse(resp)
	return
}

// GetTrieNodes do getTrieNodes through sync stream protocol.
// returns the nodes as result, target stream id, and error
func (p *Protocol) GetTrieNodes(ctx context.Context, root common.Hash, paths []*message.TrieNodePathSet, bytes uint64, opts ...Option) (nodes [][]byte, stid sttypes.StreamID, err error) {
	timer := p.doMetricClientRequest("getTrieNodes")
	defer p.doMetricPostClientRequest("getTrieNodes", err, timer)

	if bytes == 0 {
		err = fmt.Errorf("zero trie nodes bytes requested")
		return
	}
	if bytes > softResponseLimit {
		err = fmt.Errorf("requested bytes exceed limit")
		return
	}
	if len(paths) > GetTrieNodesRequestCap {
		err = fmt.Errorf("number of requested paths exceed limit")
		return
	}
	req := newGetTrieNodesRequest(root, paths, bytes)
	resp, stid, err := p.rm.DoRequest(ctx, req, opts...)
	if err != nil {
		return
	}
	nodes, err = req.getTrieNodesFromResponse(resp)
	return
}

// getBlocksByNumberRequest is the request for get block by numbers which implements
// sttypes.Request interface
type getBlocksByNumberRequest struct {
	bns   []uint64
	pbReq *syncpb.Request
}

func newGetBlocksByNumberRequest(bns []uint64) *getBlocksByNumberRequest {
	pbReq := syncpb.MakeGetBlocksByNumRequest(bns)
	return &getBlocksByNumberRequest{
		bns:   bns,
		pbReq: pbReq,
	}
}

func (req *getBlocksByNumberRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getBlocksByNumberRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getBlocksByNumberRequest) String() string {
	ss := make([]string, 0, len(req.bns))
	for _, bn := range req.bns {
		ss = append(ss, strconv.Itoa(int(bn)))
	}
	bnsStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetBlockByNumber: %s]", bnsStr)
}

func (req *getBlocksByNumberRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getBlocksByNumberRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getBlocksByNumberRequest) getBlocksFromResponse(resp sttypes.Response) ([]*types.Block, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	blockBytes, sigs, err := req.parseBlockBytesAndSigs(sResp)
	if err != nil {
		return nil, err
	}
	blocks := make([]*types.Block, 0, len(blockBytes))
	for i, bb := range blockBytes {
		var block *types.Block
		if err := rlp.DecodeBytes(bb, &block); err != nil {
			return nil, errors.Wrap(err, "[GetBlocksByNumResponse]")
		}
		if block != nil {
			block.SetCurrentCommitSig(sigs[i])
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (req *getBlocksByNumberRequest) parseBlockBytesAndSigs(resp *syncResponse) ([][]byte, [][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, nil, errors.New(errResp.Error)
	}
	gbResp := resp.pb.GetGetBlocksByNumResponse()
	if gbResp == nil {
		return nil, nil, errors.New("response not GetBlockByNumber")
	}
	if len(gbResp.BlocksBytes) != len(gbResp.CommitSig) {
		return nil, nil, fmt.Errorf("commit sigs size not expected: %v / %v",
			len(gbResp.CommitSig), len(gbResp.BlocksBytes))
	}
	return gbResp.BlocksBytes, gbResp.CommitSig, nil
}

type getBlockNumberRequest struct {
	pbReq *syncpb.Request
}

func newGetBlockNumberRequest() *getBlockNumberRequest {
	pbReq := syncpb.MakeGetBlockNumberRequest()
	return &getBlockNumberRequest{
		pbReq: pbReq,
	}
}

func (req *getBlockNumberRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getBlockNumberRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getBlockNumberRequest) String() string {
	return fmt.Sprintf("REQUEST [GetBlockNumber]")
}

func (req *getBlockNumberRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getBlockNumberRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getBlockNumberRequest) getNumberFromResponse(resp sttypes.Response) (uint64, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return 0, errors.New("not sync response")
	}
	if errResp := sResp.pb.GetErrorResponse(); errResp != nil {
		return 0, errors.New(errResp.Error)
	}
	gnResp := sResp.pb.GetGetBlockNumberResponse()
	if gnResp == nil {
		return 0, errors.New("response not GetBlockNumber")
	}
	return gnResp.Number, nil
}

type getBlockHashesRequest struct {
	bns   []uint64
	pbReq *syncpb.Request
}

func newGetBlockHashesRequest(bns []uint64) *getBlockHashesRequest {
	pbReq := syncpb.MakeGetBlockHashesRequest(bns)
	return &getBlockHashesRequest{
		bns:   bns,
		pbReq: pbReq,
	}
}

func (req *getBlockHashesRequest) ReqID() uint64 {
	return req.pbReq.ReqId
}

func (req *getBlockHashesRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getBlockHashesRequest) String() string {
	ss := make([]string, 0, len(req.bns))
	for _, bn := range req.bns {
		ss = append(ss, strconv.Itoa(int(bn)))
	}
	bnsStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetBlockHashes: %s]", bnsStr)
}

func (req *getBlockHashesRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getBlockHashesRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getBlockHashesRequest) getHashesFromResponse(resp sttypes.Response) ([]common.Hash, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	if errResp := sResp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	bhResp := sResp.pb.GetGetBlockHashesResponse()
	if bhResp == nil {
		return nil, errors.New("response not GetBlockHashes")
	}
	hashBytes := bhResp.Hashes
	return bytesToHashes(hashBytes), nil
}

type getBlocksByHashesRequest struct {
	hashes []common.Hash
	pbReq  *syncpb.Request
}

func newGetBlocksByHashesRequest(hashes []common.Hash) *getBlocksByHashesRequest {
	pbReq := syncpb.MakeGetBlocksByHashesRequest(hashes)
	return &getBlocksByHashesRequest{
		hashes: hashes,
		pbReq:  pbReq,
	}
}

func (req *getBlocksByHashesRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getBlocksByHashesRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getBlocksByHashesRequest) String() string {
	hashStrs := make([]string, 0, len(req.hashes))
	for _, h := range req.hashes {
		hashStrs = append(hashStrs, fmt.Sprintf("%x", h[:]))
	}
	hStr := strings.Join(hashStrs, ", ")
	return fmt.Sprintf("REQUEST [GetBlocksByHashes: %v]", hStr)
}

func (req *getBlocksByHashesRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getBlocksByHashesRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getBlocksByHashesRequest) getBlocksFromResponse(resp sttypes.Response) ([]*types.Block, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	if errResp := sResp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	bhResp := sResp.pb.GetGetBlocksByHashesResponse()
	if bhResp == nil {
		return nil, errors.New("response not GetBlocksByHashes")
	}
	var (
		blockBytes = bhResp.BlocksBytes
		sigs       = bhResp.CommitSig
	)
	if len(blockBytes) != len(sigs) {
		return nil, fmt.Errorf("sig size not expected: %v / %v", len(sigs), len(blockBytes))
	}
	blocks := make([]*types.Block, 0, len(blockBytes))
	for i, bb := range blockBytes {
		var block *types.Block
		if err := rlp.DecodeBytes(bb, &block); err != nil {
			return nil, errors.Wrap(err, "[GetBlocksByHashesResponse]")
		}
		if block != nil {
			block.SetCurrentCommitSig(sigs[i])
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// getNodeDataRequest is the request for get node data which implements
// sttypes.Request interface
type getNodeDataRequest struct {
	hashes []common.Hash
	pbReq  *syncpb.Request
}

func newGetNodeDataRequest(hashes []common.Hash) *getNodeDataRequest {
	pbReq := syncpb.MakeGetNodeDataRequest(hashes)
	return &getNodeDataRequest{
		hashes: hashes,
		pbReq:  pbReq,
	}
}

func (req *getNodeDataRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getNodeDataRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getNodeDataRequest) String() string {
	ss := make([]string, 0, len(req.hashes))
	for _, h := range req.hashes {
		ss = append(ss, h.String())
	}
	hsStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetNodeData: %s]", hsStr)
}

func (req *getNodeDataRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getNodeDataRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getNodeDataRequest) getNodeDataFromResponse(resp sttypes.Response) ([][]byte, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	dataBytes, err := req.parseNodeDataBytes(sResp)
	if err != nil {
		return nil, err
	}
	return dataBytes, nil
}

func (req *getNodeDataRequest) parseNodeDataBytes(resp *syncResponse) ([][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	ndResp := resp.pb.GetGetNodeDataResponse()
	if ndResp == nil {
		return nil, errors.New("response not GetNodeData")
	}
	return ndResp.DataBytes, nil
}

// getReceiptsRequest is the request for get receipts which implements
// sttypes.Request interface
type getReceiptsRequest struct {
	hashes []common.Hash
	pbReq  *syncpb.Request
}

func newGetReceiptsRequest(hashes []common.Hash) *getReceiptsRequest {
	pbReq := syncpb.MakeGetReceiptsRequest(hashes)
	return &getReceiptsRequest{
		hashes: hashes,
		pbReq:  pbReq,
	}
}

func (req *getReceiptsRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getReceiptsRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getReceiptsRequest) String() string {
	ss := make([]string, 0, len(req.hashes))
	for _, h := range req.hashes {
		ss = append(ss, h.String())
	}
	hsStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetReceipts: %s]", hsStr)
}

func (req *getReceiptsRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getReceiptsRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getReceiptsRequest) getReceiptsFromResponse(resp sttypes.Response) ([]types.Receipts, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	receipts, err := req.parseGetReceiptsBytes(sResp)
	if err != nil {
		return nil, err
	}
	return receipts, nil
}

func (req *getReceiptsRequest) parseGetReceiptsBytes(resp *syncResponse) ([]types.Receipts, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	grResp := resp.pb.GetGetReceiptsResponse()
	if grResp == nil {
		return nil, errors.New("response not GetReceipts")
	}
	receipts := make([]types.Receipts, len(grResp.Receipts))
	for i, blockReceipts := range grResp.Receipts {
		for _, rcptBytes := range blockReceipts.ReceiptBytes {
			var receipt *types.Receipt
			if err := rlp.DecodeBytes(rcptBytes, &receipt); err != nil {
				return nil, errors.Wrap(err, "[GetReceiptsResponse]")
			}
			receipts[i] = append(receipts[i], receipt)
		}
	}
	return receipts, nil
}

// getAccountRangeRequest is the request for get account ranges which implements
// sttypes.Request interface
type getAccountRangeRequest struct {
	root   common.Hash
	origin common.Hash
	limit  common.Hash
	bytes  uint64
	pbReq  *syncpb.Request
}

func newGetAccountRangeRequest(root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) *getAccountRangeRequest {
	pbReq := syncpb.MakeGetAccountRangeRequest(root, origin, limit, bytes)
	return &getAccountRangeRequest{
		root:   root,
		origin: origin,
		limit:  limit,
		bytes:  bytes,
		pbReq:  pbReq,
	}
}

func (req *getAccountRangeRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getAccountRangeRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getAccountRangeRequest) String() string {
	ss := make([]string, 0, 4)
	ss = append(ss, req.root.String())
	ss = append(ss, req.origin.String())
	ss = append(ss, req.limit.String())
	ss = append(ss, fmt.Sprint(req.bytes))
	rqStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetAccountRange: %s]", rqStr)
}

func (req *getAccountRangeRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getAccountRangeRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getAccountRangeRequest) getAccountRangeFromResponse(resp sttypes.Response) ([]*message.AccountData, [][]byte, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, nil, errors.New("not sync response")
	}
	return req.parseGetAccountRangeResponse(sResp)
}

func (req *getAccountRangeRequest) parseGetAccountRangeResponse(resp *syncResponse) ([]*message.AccountData, [][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, nil, errors.New(errResp.Error)
	}
	grResp := resp.pb.GetGetAccountRangeResponse()
	if grResp == nil {
		return nil, nil, errors.New("response not GetAccountRange")
	}
	proofs := make([][]byte, 0)
	for _, proofBytes := range grResp.Proof {
		var proof []byte
		if err := rlp.DecodeBytes(proofBytes, &proof); err != nil {
			return nil, nil, errors.Wrap(err, "[GetAccountRangeResponse]")
		}
		proofs = append(proofs, proof)
	}
	return grResp.Accounts, proofs, nil
}

// getStorageRangesRequest is the request for get storage ranges which implements
// sttypes.Request interface
type getStorageRangesRequest struct {
	root     common.Hash
	accounts []common.Hash
	origin   common.Hash
	limit    common.Hash
	bytes    uint64
	pbReq    *syncpb.Request
}

func newGetStorageRangesRequest(root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) *getStorageRangesRequest {
	pbReq := syncpb.MakeGetStorageRangesRequest(root, accounts, origin, limit, bytes)
	return &getStorageRangesRequest{
		root:     root,
		accounts: accounts,
		origin:   origin,
		limit:    limit,
		bytes:    bytes,
		pbReq:    pbReq,
	}
}

func (req *getStorageRangesRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getStorageRangesRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getStorageRangesRequest) String() string {
	ss := make([]string, 0, 4)
	ss = append(ss, req.root.String())
	for _, acc := range req.accounts {
		ss = append(ss, acc.String())
	}
	ss = append(ss, req.origin.String())
	ss = append(ss, req.limit.String())
	ss = append(ss, fmt.Sprint(req.bytes))
	rqStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetStorageRanges: %s]", rqStr)
}

func (req *getStorageRangesRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getStorageRangesRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

// []*message.AccountData, []common.Hash
func (req *getStorageRangesRequest) getStorageRangesFromResponse(resp sttypes.Response) ([]*message.StoragesData, [][]byte, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, nil, errors.New("not sync response")
	}
	return req.parseGetStorageRangesResponse(sResp)
}

func (req *getStorageRangesRequest) parseGetStorageRangesResponse(resp *syncResponse) ([]*message.StoragesData, [][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, nil, errors.New(errResp.Error)
	}
	grResp := resp.pb.GetGetStorageRangesResponse()
	if grResp == nil {
		return nil, nil, errors.New("response not GetStorageRanges")
	}
	proofs := make([][]byte, 0)
	for _, proofBytes := range grResp.Proof {
		var proof []byte
		if err := rlp.DecodeBytes(proofBytes, &proof); err != nil {
			return nil, nil, errors.Wrap(err, "[GetStorageRangesResponse]")
		}
		proofs = append(proofs, proof)
	}
	return grResp.Slots, proofs, nil
}

// getByteCodesRequest is the request for get code bytes which implements
// sttypes.Request interface
type getByteCodesRequest struct {
	hashes []common.Hash
	bytes  uint64
	pbReq  *syncpb.Request
}

func newGetByteCodesRequest(hashes []common.Hash, bytes uint64) *getByteCodesRequest {
	pbReq := syncpb.MakeGetByteCodesRequest(hashes, bytes)
	return &getByteCodesRequest{
		hashes: hashes,
		bytes:  bytes,
		pbReq:  pbReq,
	}
}

func (req *getByteCodesRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getByteCodesRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getByteCodesRequest) String() string {
	ss := make([]string, 0, 4)
	for _, h := range req.hashes {
		ss = append(ss, h.String())
	}
	ss = append(ss, fmt.Sprint(req.bytes))
	rqStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetByteCodes: %s]", rqStr)
}

func (req *getByteCodesRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getByteCodesRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getByteCodesRequest) getByteCodesFromResponse(resp sttypes.Response) ([][]byte, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	return req.parseGetByteCodesResponse(sResp)
}

func (req *getByteCodesRequest) parseGetByteCodesResponse(resp *syncResponse) ([][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	grResp := resp.pb.GetGetByteCodesResponse()
	if grResp == nil {
		return nil, errors.New("response not GetByteCodes")
	}
	codes := make([][]byte, 0)
	for _, codeBytes := range grResp.Codes {
		var code []byte
		if err := rlp.DecodeBytes(codeBytes, &code); err != nil {
			return nil, errors.Wrap(err, "[GetByteCodesResponse]")
		}
		codes = append(codes, code)
	}
	return codes, nil
}

// getTrieNodesRequest is the request for get trie nodes which implements
// sttypes.Request interface
type getTrieNodesRequest struct {
	root  common.Hash
	paths []*message.TrieNodePathSet
	bytes uint64
	pbReq *syncpb.Request
}

func newGetTrieNodesRequest(root common.Hash, paths []*message.TrieNodePathSet, bytes uint64) *getTrieNodesRequest {
	pbReq := syncpb.MakeGetTrieNodesRequest(root, paths, bytes)
	return &getTrieNodesRequest{
		root:  root,
		paths: paths,
		bytes: bytes,
		pbReq: pbReq,
	}
}

func (req *getTrieNodesRequest) ReqID() uint64 {
	return req.pbReq.GetReqId()
}

func (req *getTrieNodesRequest) SetReqID(val uint64) {
	req.pbReq.ReqId = val
}

func (req *getTrieNodesRequest) String() string {
	ss := make([]string, 0, 4)
	ss = append(ss, req.root.String())
	for _, p := range req.paths {
		ss = append(ss, p.String())
	}
	ss = append(ss, fmt.Sprint(req.bytes))
	rqStr := strings.Join(ss, ",")
	return fmt.Sprintf("REQUEST [GetTrieNodes: %s]", rqStr)
}

func (req *getTrieNodesRequest) IsSupportedByProto(target sttypes.ProtoSpec) bool {
	return target.Version.GreaterThanOrEqual(MinVersion)
}

func (req *getTrieNodesRequest) Encode() ([]byte, error) {
	msg := syncpb.MakeMessageFromRequest(req.pbReq)
	return protobuf.Marshal(msg)
}

func (req *getTrieNodesRequest) getTrieNodesFromResponse(resp sttypes.Response) ([][]byte, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	return req.parseGetTrieNodesResponse(sResp)
}

func (req *getTrieNodesRequest) parseGetTrieNodesResponse(resp *syncResponse) ([][]byte, error) {
	if errResp := resp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	grResp := resp.pb.GetGetTrieNodesResponse()
	if grResp == nil {
		return nil, errors.New("response not GetTrieNodes")
	}
	nodes := make([][]byte, 0)
	for _, codeBytes := range grResp.Nodes {
		var code []byte
		if err := rlp.DecodeBytes(codeBytes, &code); err != nil {
			return nil, errors.Wrap(err, "[GetTrieNodesResponse]")
		}
		nodes = append(nodes, code)
	}
	return nodes, nil
}
