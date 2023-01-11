// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: msg.proto

package message

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Message struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to ReqOrResp:
	//
	//	*Message_Req
	//	*Message_Resp
	ReqOrResp isMessage_ReqOrResp `protobuf_oneof:"req_or_resp"`
}

func (x *Message) Reset() {
	*x = Message{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Message.ProtoReflect.Descriptor instead.
func (*Message) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{0}
}

func (m *Message) GetReqOrResp() isMessage_ReqOrResp {
	if m != nil {
		return m.ReqOrResp
	}
	return nil
}

func (x *Message) GetReq() *Request {
	if x, ok := x.GetReqOrResp().(*Message_Req); ok {
		return x.Req
	}
	return nil
}

func (x *Message) GetResp() *Response {
	if x, ok := x.GetReqOrResp().(*Message_Resp); ok {
		return x.Resp
	}
	return nil
}

type isMessage_ReqOrResp interface {
	isMessage_ReqOrResp()
}

type Message_Req struct {
	Req *Request `protobuf:"bytes,1,opt,name=req,proto3,oneof"`
}

type Message_Resp struct {
	Resp *Response `protobuf:"bytes,2,opt,name=resp,proto3,oneof"`
}

func (*Message_Req) isMessage_ReqOrResp() {}

func (*Message_Resp) isMessage_ReqOrResp() {}

type Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ReqId uint64 `protobuf:"varint,1,opt,name=req_id,json=reqId,proto3" json:"req_id,omitempty"`
	// Types that are assignable to Request:
	//
	//	*Request_GetBlockNumberRequest
	//	*Request_GetBlockHashesRequest
	//	*Request_GetBlocksByNumRequest
	//	*Request_GetBlocksByHashesRequest
	Request isRequest_Request `protobuf_oneof:"request"`
}

func (x *Request) Reset() {
	*x = Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{1}
}

func (x *Request) GetReqId() uint64 {
	if x != nil {
		return x.ReqId
	}
	return 0
}

func (m *Request) GetRequest() isRequest_Request {
	if m != nil {
		return m.Request
	}
	return nil
}

func (x *Request) GetGetBlockNumberRequest() *GetBlockNumberRequest {
	if x, ok := x.GetRequest().(*Request_GetBlockNumberRequest); ok {
		return x.GetBlockNumberRequest
	}
	return nil
}

func (x *Request) GetGetBlockHashesRequest() *GetBlockHashesRequest {
	if x, ok := x.GetRequest().(*Request_GetBlockHashesRequest); ok {
		return x.GetBlockHashesRequest
	}
	return nil
}

func (x *Request) GetGetBlocksByNumRequest() *GetBlocksByNumRequest {
	if x, ok := x.GetRequest().(*Request_GetBlocksByNumRequest); ok {
		return x.GetBlocksByNumRequest
	}
	return nil
}

func (x *Request) GetGetBlocksByHashesRequest() *GetBlocksByHashesRequest {
	if x, ok := x.GetRequest().(*Request_GetBlocksByHashesRequest); ok {
		return x.GetBlocksByHashesRequest
	}
	return nil
}

type isRequest_Request interface {
	isRequest_Request()
}

type Request_GetBlockNumberRequest struct {
	GetBlockNumberRequest *GetBlockNumberRequest `protobuf:"bytes,2,opt,name=get_block_number_request,json=getBlockNumberRequest,proto3,oneof"`
}

type Request_GetBlockHashesRequest struct {
	GetBlockHashesRequest *GetBlockHashesRequest `protobuf:"bytes,3,opt,name=get_block_hashes_request,json=getBlockHashesRequest,proto3,oneof"`
}

type Request_GetBlocksByNumRequest struct {
	GetBlocksByNumRequest *GetBlocksByNumRequest `protobuf:"bytes,4,opt,name=get_blocks_by_num_request,json=getBlocksByNumRequest,proto3,oneof"`
}

type Request_GetBlocksByHashesRequest struct {
	GetBlocksByHashesRequest *GetBlocksByHashesRequest `protobuf:"bytes,5,opt,name=get_blocks_by_hashes_request,json=getBlocksByHashesRequest,proto3,oneof"`
}

func (*Request_GetBlockNumberRequest) isRequest_Request() {}

func (*Request_GetBlockHashesRequest) isRequest_Request() {}

func (*Request_GetBlocksByNumRequest) isRequest_Request() {}

func (*Request_GetBlocksByHashesRequest) isRequest_Request() {}

type GetBlockNumberRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetBlockNumberRequest) Reset() {
	*x = GetBlockNumberRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlockNumberRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlockNumberRequest) ProtoMessage() {}

func (x *GetBlockNumberRequest) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlockNumberRequest.ProtoReflect.Descriptor instead.
func (*GetBlockNumberRequest) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{2}
}

type GetBlockHashesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Nums []uint64 `protobuf:"varint,1,rep,packed,name=nums,proto3" json:"nums,omitempty"`
}

func (x *GetBlockHashesRequest) Reset() {
	*x = GetBlockHashesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlockHashesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlockHashesRequest) ProtoMessage() {}

func (x *GetBlockHashesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlockHashesRequest.ProtoReflect.Descriptor instead.
func (*GetBlockHashesRequest) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{3}
}

func (x *GetBlockHashesRequest) GetNums() []uint64 {
	if x != nil {
		return x.Nums
	}
	return nil
}

type GetBlocksByNumRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Nums []uint64 `protobuf:"varint,1,rep,packed,name=nums,proto3" json:"nums,omitempty"`
}

func (x *GetBlocksByNumRequest) Reset() {
	*x = GetBlocksByNumRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlocksByNumRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlocksByNumRequest) ProtoMessage() {}

func (x *GetBlocksByNumRequest) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlocksByNumRequest.ProtoReflect.Descriptor instead.
func (*GetBlocksByNumRequest) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{4}
}

func (x *GetBlocksByNumRequest) GetNums() []uint64 {
	if x != nil {
		return x.Nums
	}
	return nil
}

type GetBlocksByHashesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlockHashes [][]byte `protobuf:"bytes,1,rep,name=block_hashes,json=blockHashes,proto3" json:"block_hashes,omitempty"`
}

func (x *GetBlocksByHashesRequest) Reset() {
	*x = GetBlocksByHashesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlocksByHashesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlocksByHashesRequest) ProtoMessage() {}

func (x *GetBlocksByHashesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlocksByHashesRequest.ProtoReflect.Descriptor instead.
func (*GetBlocksByHashesRequest) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{5}
}

func (x *GetBlocksByHashesRequest) GetBlockHashes() [][]byte {
	if x != nil {
		return x.BlockHashes
	}
	return nil
}

type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ReqId uint64 `protobuf:"varint,1,opt,name=req_id,json=reqId,proto3" json:"req_id,omitempty"`
	// Types that are assignable to Response:
	//
	//	*Response_ErrorResponse
	//	*Response_GetBlockNumberResponse
	//	*Response_GetBlockHashesResponse
	//	*Response_GetBlocksByNumResponse
	//	*Response_GetBlocksByHashesResponse
	Response isResponse_Response `protobuf_oneof:"response"`
}

func (x *Response) Reset() {
	*x = Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Response) ProtoMessage() {}

func (x *Response) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Response.ProtoReflect.Descriptor instead.
func (*Response) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{6}
}

func (x *Response) GetReqId() uint64 {
	if x != nil {
		return x.ReqId
	}
	return 0
}

func (m *Response) GetResponse() isResponse_Response {
	if m != nil {
		return m.Response
	}
	return nil
}

func (x *Response) GetErrorResponse() *ErrorResponse {
	if x, ok := x.GetResponse().(*Response_ErrorResponse); ok {
		return x.ErrorResponse
	}
	return nil
}

func (x *Response) GetGetBlockNumberResponse() *GetBlockNumberResponse {
	if x, ok := x.GetResponse().(*Response_GetBlockNumberResponse); ok {
		return x.GetBlockNumberResponse
	}
	return nil
}

func (x *Response) GetGetBlockHashesResponse() *GetBlockHashesResponse {
	if x, ok := x.GetResponse().(*Response_GetBlockHashesResponse); ok {
		return x.GetBlockHashesResponse
	}
	return nil
}

func (x *Response) GetGetBlocksByNumResponse() *GetBlocksByNumResponse {
	if x, ok := x.GetResponse().(*Response_GetBlocksByNumResponse); ok {
		return x.GetBlocksByNumResponse
	}
	return nil
}

func (x *Response) GetGetBlocksByHashesResponse() *GetBlocksByHashesResponse {
	if x, ok := x.GetResponse().(*Response_GetBlocksByHashesResponse); ok {
		return x.GetBlocksByHashesResponse
	}
	return nil
}

type isResponse_Response interface {
	isResponse_Response()
}

type Response_ErrorResponse struct {
	ErrorResponse *ErrorResponse `protobuf:"bytes,2,opt,name=error_response,json=errorResponse,proto3,oneof"`
}

type Response_GetBlockNumberResponse struct {
	GetBlockNumberResponse *GetBlockNumberResponse `protobuf:"bytes,3,opt,name=get_block_number_response,json=getBlockNumberResponse,proto3,oneof"`
}

type Response_GetBlockHashesResponse struct {
	GetBlockHashesResponse *GetBlockHashesResponse `protobuf:"bytes,4,opt,name=get_block_hashes_response,json=getBlockHashesResponse,proto3,oneof"`
}

type Response_GetBlocksByNumResponse struct {
	GetBlocksByNumResponse *GetBlocksByNumResponse `protobuf:"bytes,5,opt,name=get_blocks_by_num_response,json=getBlocksByNumResponse,proto3,oneof"`
}

type Response_GetBlocksByHashesResponse struct {
	GetBlocksByHashesResponse *GetBlocksByHashesResponse `protobuf:"bytes,6,opt,name=get_blocks_by_hashes_response,json=getBlocksByHashesResponse,proto3,oneof"`
}

func (*Response_ErrorResponse) isResponse_Response() {}

func (*Response_GetBlockNumberResponse) isResponse_Response() {}

func (*Response_GetBlockHashesResponse) isResponse_Response() {}

func (*Response_GetBlocksByNumResponse) isResponse_Response() {}

func (*Response_GetBlocksByHashesResponse) isResponse_Response() {}

type ErrorResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Error string `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *ErrorResponse) Reset() {
	*x = ErrorResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ErrorResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrorResponse) ProtoMessage() {}

func (x *ErrorResponse) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorResponse.ProtoReflect.Descriptor instead.
func (*ErrorResponse) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{7}
}

func (x *ErrorResponse) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type GetBlockNumberResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Number uint64 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
}

func (x *GetBlockNumberResponse) Reset() {
	*x = GetBlockNumberResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlockNumberResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlockNumberResponse) ProtoMessage() {}

func (x *GetBlockNumberResponse) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlockNumberResponse.ProtoReflect.Descriptor instead.
func (*GetBlockNumberResponse) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{8}
}

func (x *GetBlockNumberResponse) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

type GetBlockHashesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hashes [][]byte `protobuf:"bytes,1,rep,name=hashes,proto3" json:"hashes,omitempty"`
}

func (x *GetBlockHashesResponse) Reset() {
	*x = GetBlockHashesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlockHashesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlockHashesResponse) ProtoMessage() {}

func (x *GetBlockHashesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlockHashesResponse.ProtoReflect.Descriptor instead.
func (*GetBlockHashesResponse) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{9}
}

func (x *GetBlockHashesResponse) GetHashes() [][]byte {
	if x != nil {
		return x.Hashes
	}
	return nil
}

type GetBlocksByNumResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlocksBytes [][]byte `protobuf:"bytes,1,rep,name=blocks_bytes,json=blocksBytes,proto3" json:"blocks_bytes,omitempty"`
	CommitSig   [][]byte `protobuf:"bytes,2,rep,name=commit_sig,json=commitSig,proto3" json:"commit_sig,omitempty"`
}

func (x *GetBlocksByNumResponse) Reset() {
	*x = GetBlocksByNumResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlocksByNumResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlocksByNumResponse) ProtoMessage() {}

func (x *GetBlocksByNumResponse) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlocksByNumResponse.ProtoReflect.Descriptor instead.
func (*GetBlocksByNumResponse) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{10}
}

func (x *GetBlocksByNumResponse) GetBlocksBytes() [][]byte {
	if x != nil {
		return x.BlocksBytes
	}
	return nil
}

func (x *GetBlocksByNumResponse) GetCommitSig() [][]byte {
	if x != nil {
		return x.CommitSig
	}
	return nil
}

type GetBlocksByHashesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlocksBytes [][]byte `protobuf:"bytes,1,rep,name=blocks_bytes,json=blocksBytes,proto3" json:"blocks_bytes,omitempty"`
	CommitSig   [][]byte `protobuf:"bytes,2,rep,name=commit_sig,json=commitSig,proto3" json:"commit_sig,omitempty"`
}

func (x *GetBlocksByHashesResponse) Reset() {
	*x = GetBlocksByHashesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_msg_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBlocksByHashesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBlocksByHashesResponse) ProtoMessage() {}

func (x *GetBlocksByHashesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBlocksByHashesResponse.ProtoReflect.Descriptor instead.
func (*GetBlocksByHashesResponse) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{11}
}

func (x *GetBlocksByHashesResponse) GetBlocksBytes() [][]byte {
	if x != nil {
		return x.BlocksBytes
	}
	return nil
}

func (x *GetBlocksByHashesResponse) GetCommitSig() [][]byte {
	if x != nil {
		return x.CommitSig
	}
	return nil
}

var File_msg_proto protoreflect.FileDescriptor

var file_msg_proto_rawDesc = []byte{
	0x0a, 0x09, 0x6d, 0x73, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1b, 0x68, 0x61, 0x72,
	0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63,
	0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x8f, 0x01, 0x0a, 0x07, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x38, 0x0a, 0x03, 0x72, 0x65, 0x71, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x24, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00, 0x52, 0x03, 0x72, 0x65, 0x71, 0x12, 0x3b,
	0x0a, 0x04, 0x72, 0x65, 0x73, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x68,
	0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79,
	0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x48, 0x00, 0x52, 0x04, 0x72, 0x65, 0x73, 0x70, 0x42, 0x0d, 0x0a, 0x0b, 0x72,
	0x65, 0x71, 0x5f, 0x6f, 0x72, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x22, 0xf2, 0x03, 0x0a, 0x07, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x15, 0x0a, 0x06, 0x72, 0x65, 0x71, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x72, 0x65, 0x71, 0x49, 0x64, 0x12, 0x6d, 0x0a,
	0x18, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x6e, 0x75, 0x6d, 0x62, 0x65,
	0x72, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x32, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47, 0x65,
	0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x48, 0x00, 0x52, 0x15, 0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e,
	0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x6d, 0x0a, 0x18,
	0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x65, 0x73,
	0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32,
	0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e,
	0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47, 0x65, 0x74,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x48, 0x00, 0x52, 0x15, 0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61,
	0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x6e, 0x0a, 0x19, 0x67,
	0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x5f, 0x62, 0x79, 0x5f, 0x6e, 0x75, 0x6d,
	0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32,
	0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e,
	0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47, 0x65, 0x74,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x4e, 0x75, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x48, 0x00, 0x52, 0x15, 0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42,
	0x79, 0x4e, 0x75, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x77, 0x0a, 0x1c, 0x67,
	0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x5f, 0x62, 0x79, 0x5f, 0x68, 0x61, 0x73,
	0x68, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x35, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e,
	0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65,
	0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00, 0x52, 0x18, 0x67, 0x65, 0x74, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x42, 0x09, 0x0a, 0x07, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22,
	0x17, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65,
	0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x2f, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x16, 0x0a, 0x04, 0x6e, 0x75, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x04, 0x42,
	0x02, 0x10, 0x01, 0x52, 0x04, 0x6e, 0x75, 0x6d, 0x73, 0x22, 0x2f, 0x0a, 0x15, 0x47, 0x65, 0x74,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x4e, 0x75, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x16, 0x0a, 0x04, 0x6e, 0x75, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x04,
	0x42, 0x02, 0x10, 0x01, 0x52, 0x04, 0x6e, 0x75, 0x6d, 0x73, 0x22, 0x3d, 0x0a, 0x18, 0x47, 0x65,
	0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f,
	0x68, 0x61, 0x73, 0x68, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x0b, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x22, 0xd5, 0x04, 0x0a, 0x08, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x15, 0x0a, 0x06, 0x72, 0x65, 0x71, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x72, 0x65, 0x71, 0x49, 0x64, 0x12, 0x53, 0x0a,
	0x0e, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x2e, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x48, 0x00, 0x52, 0x0d, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x70, 0x0a, 0x19, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f,
	0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x00, 0x52, 0x16, 0x67, 0x65,
	0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x70, 0x0a, 0x19, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e,
	0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61,
	0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x00, 0x52, 0x16,
	0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x71, 0x0a, 0x1a, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x5f, 0x62, 0x79, 0x5f, 0x6e, 0x75, 0x6d, 0x5f, 0x72, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x72,
	0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63,
	0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63,
	0x6b, 0x73, 0x42, 0x79, 0x4e, 0x75, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48,
	0x00, 0x52, 0x16, 0x67, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x4e, 0x75,
	0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x7a, 0x0a, 0x1d, 0x67, 0x65, 0x74,
	0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x5f, 0x62, 0x79, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x65,
	0x73, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x36, 0x2e, 0x68, 0x61, 0x72, 0x6d, 0x6f, 0x6e, 0x79, 0x2e, 0x73, 0x74, 0x72, 0x65, 0x61,
	0x6d, 0x2e, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x47,
	0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x00, 0x52, 0x19, 0x67, 0x65, 0x74, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x0a, 0x0a, 0x08, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x25, 0x0a, 0x0d, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x22, 0x30, 0x0a, 0x16, 0x47, 0x65, 0x74, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x22, 0x30, 0x0a, 0x16, 0x47, 0x65,
	0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x61, 0x73, 0x68, 0x65, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0c, 0x52, 0x06, 0x68, 0x61, 0x73, 0x68, 0x65, 0x73, 0x22, 0x5a, 0x0a, 0x16,
	0x47, 0x65, 0x74, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x4e, 0x75, 0x6d, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73,
	0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x0b, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d,
	0x6d, 0x69, 0x74, 0x5f, 0x73, 0x69, 0x67, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x09, 0x63,
	0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x53, 0x69, 0x67, 0x22, 0x5d, 0x0a, 0x19, 0x47, 0x65, 0x74, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x42, 0x79, 0x48, 0x61, 0x73, 0x68, 0x65, 0x73, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x5f,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x0b, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x73, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d,
	0x69, 0x74, 0x5f, 0x73, 0x69, 0x67, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x09, 0x63, 0x6f,
	0x6d, 0x6d, 0x69, 0x74, 0x53, 0x69, 0x67, 0x42, 0x0c, 0x5a, 0x0a, 0x2e, 0x2f, 0x3b, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_msg_proto_rawDescOnce sync.Once
	file_msg_proto_rawDescData = file_msg_proto_rawDesc
)

func file_msg_proto_rawDescGZIP() []byte {
	file_msg_proto_rawDescOnce.Do(func() {
		file_msg_proto_rawDescData = protoimpl.X.CompressGZIP(file_msg_proto_rawDescData)
	})
	return file_msg_proto_rawDescData
}

var file_msg_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_msg_proto_goTypes = []interface{}{
	(*Message)(nil),                   // 0: harmony.stream.sync.message.Message
	(*Request)(nil),                   // 1: harmony.stream.sync.message.Request
	(*GetBlockNumberRequest)(nil),     // 2: harmony.stream.sync.message.GetBlockNumberRequest
	(*GetBlockHashesRequest)(nil),     // 3: harmony.stream.sync.message.GetBlockHashesRequest
	(*GetBlocksByNumRequest)(nil),     // 4: harmony.stream.sync.message.GetBlocksByNumRequest
	(*GetBlocksByHashesRequest)(nil),  // 5: harmony.stream.sync.message.GetBlocksByHashesRequest
	(*Response)(nil),                  // 6: harmony.stream.sync.message.Response
	(*ErrorResponse)(nil),             // 7: harmony.stream.sync.message.ErrorResponse
	(*GetBlockNumberResponse)(nil),    // 8: harmony.stream.sync.message.GetBlockNumberResponse
	(*GetBlockHashesResponse)(nil),    // 9: harmony.stream.sync.message.GetBlockHashesResponse
	(*GetBlocksByNumResponse)(nil),    // 10: harmony.stream.sync.message.GetBlocksByNumResponse
	(*GetBlocksByHashesResponse)(nil), // 11: harmony.stream.sync.message.GetBlocksByHashesResponse
}
var file_msg_proto_depIdxs = []int32{
	1,  // 0: harmony.stream.sync.message.Message.req:type_name -> harmony.stream.sync.message.Request
	6,  // 1: harmony.stream.sync.message.Message.resp:type_name -> harmony.stream.sync.message.Response
	2,  // 2: harmony.stream.sync.message.Request.get_block_number_request:type_name -> harmony.stream.sync.message.GetBlockNumberRequest
	3,  // 3: harmony.stream.sync.message.Request.get_block_hashes_request:type_name -> harmony.stream.sync.message.GetBlockHashesRequest
	4,  // 4: harmony.stream.sync.message.Request.get_blocks_by_num_request:type_name -> harmony.stream.sync.message.GetBlocksByNumRequest
	5,  // 5: harmony.stream.sync.message.Request.get_blocks_by_hashes_request:type_name -> harmony.stream.sync.message.GetBlocksByHashesRequest
	7,  // 6: harmony.stream.sync.message.Response.error_response:type_name -> harmony.stream.sync.message.ErrorResponse
	8,  // 7: harmony.stream.sync.message.Response.get_block_number_response:type_name -> harmony.stream.sync.message.GetBlockNumberResponse
	9,  // 8: harmony.stream.sync.message.Response.get_block_hashes_response:type_name -> harmony.stream.sync.message.GetBlockHashesResponse
	10, // 9: harmony.stream.sync.message.Response.get_blocks_by_num_response:type_name -> harmony.stream.sync.message.GetBlocksByNumResponse
	11, // 10: harmony.stream.sync.message.Response.get_blocks_by_hashes_response:type_name -> harmony.stream.sync.message.GetBlocksByHashesResponse
	11, // [11:11] is the sub-list for method output_type
	11, // [11:11] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_msg_proto_init() }
func file_msg_proto_init() {
	if File_msg_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_msg_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Message); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlockNumberRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlockHashesRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlocksByNumRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlocksByHashesRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ErrorResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlockNumberResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlockHashesResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlocksByNumResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_msg_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBlocksByHashesResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_msg_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Message_Req)(nil),
		(*Message_Resp)(nil),
	}
	file_msg_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*Request_GetBlockNumberRequest)(nil),
		(*Request_GetBlockHashesRequest)(nil),
		(*Request_GetBlocksByNumRequest)(nil),
		(*Request_GetBlocksByHashesRequest)(nil),
	}
	file_msg_proto_msgTypes[6].OneofWrappers = []interface{}{
		(*Response_ErrorResponse)(nil),
		(*Response_GetBlockNumberResponse)(nil),
		(*Response_GetBlockHashesResponse)(nil),
		(*Response_GetBlocksByNumResponse)(nil),
		(*Response_GetBlocksByHashesResponse)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_msg_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_msg_proto_goTypes,
		DependencyIndexes: file_msg_proto_depIdxs,
		MessageInfos:      file_msg_proto_msgTypes,
	}.Build()
	File_msg_proto = out.File
	file_msg_proto_rawDesc = nil
	file_msg_proto_goTypes = nil
	file_msg_proto_depIdxs = nil
}
