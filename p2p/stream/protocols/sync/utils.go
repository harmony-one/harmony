package sync

import (
	"fmt"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/p2p/stream/common/requestmanager"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
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
