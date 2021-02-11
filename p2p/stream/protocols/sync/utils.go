package sync

import (
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/p2p/stream/common/requestmanager"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

var (
	errUnknownReqType = errors.New("unknown request")
)

// syncResponse is the sync protocol response which implements sttypes.Response
type syncResponse struct {
	pb *syncpb.Response
}

// ReqID return the request ID of the response
func (resp *syncResponse) ReqID() uint64 {
	return resp.pb.ReqId
}

// GetProtobufMsg return the raw protobuf message
func (resp *syncResponse) GetProtobufMsg() protobuf.Message {
	return resp.pb
}

func (resp *syncResponse) String() string {
	return fmt.Sprintf("[SyncResponse %v]", resp.pb.String())
}

// EpochStateResult is the result for GetEpochStateQuery
type EpochStateResult struct {
	Header *block.Header
	State  *shard.State
}

func epochStateResultFromResponse(resp sttypes.Response) (*EpochStateResult, error) {
	sResp, ok := resp.(*syncResponse)
	if !ok || sResp == nil {
		return nil, errors.New("not sync response")
	}
	if errResp := sResp.pb.GetErrorResponse(); errResp != nil {
		return nil, errors.New(errResp.Error)
	}
	gesResp := sResp.pb.GetGetEpochStateResponse()
	if gesResp == nil {
		return nil, errors.New("not GetEpochStateResponse")
	}
	var (
		headerBytes = gesResp.HeaderBytes
		ssBytes     = gesResp.ShardState

		header *block.Header
		ss     *shard.State
	)
	if len(headerBytes) > 0 {
		if err := rlp.DecodeBytes(headerBytes, &header); err != nil {
			return nil, err
		}
	}
	if len(ssBytes) > 0 {
		// here shard state is not encoded with legacy rules
		if err := rlp.DecodeBytes(ssBytes, &ss); err != nil {
			return nil, err
		}
	}
	return &EpochStateResult{
		Header: header,
		State:  ss,
	}, nil
}

func (res *EpochStateResult) toMessage(rid uint64) (*syncpb.Message, error) {
	headerBytes, err := rlp.EncodeToBytes(res.Header)
	if err != nil {
		return nil, err
	}
	// Shard state is not wrapped here, means no legacy shard state encoding rule as
	// in shard.EncodeWrapper.
	ssBytes, err := rlp.EncodeToBytes(res.State)
	if err != nil {
		return nil, err
	}
	return syncpb.MakeGetEpochStateResponseMessage(rid, headerBytes, ssBytes), nil
}

// Option is the additional option to do requests.
// Currently, two options are supported:
//  1. WithHighPriority - do the request in high priority.
//  2. WithBlacklist - do the request without the given stream ids as blacklist
//  3. WithWhitelist - do the request only with the given stream ids
type Option = requestmanager.RequestOption

var (
	// WithHighPriority instruct the request manager to do the request with high
	// priority
	WithHighPriority = requestmanager.WithHighPriority
	// WithBlacklist instruct the request manager not to assign the request to the
	// given streamID
	WithBlacklist = requestmanager.WithBlacklist
	// WithWhitelist instruct the request manager only to assign the request to the
	// given streamID
	WithWhitelist = requestmanager.WithWhitelist
)
