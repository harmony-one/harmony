// Code generated by protoc-gen-go. DO NOT EDIT.
// source: downloader.proto

package downloader

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type DownloaderRequest_RequestType int32

const (
	DownloaderRequest_HEADER          DownloaderRequest_RequestType = 0
	DownloaderRequest_BLOCK           DownloaderRequest_RequestType = 1
	DownloaderRequest_NEWBLOCK        DownloaderRequest_RequestType = 2
	DownloaderRequest_BLOCKHEIGHT     DownloaderRequest_RequestType = 3
	DownloaderRequest_REGISTER        DownloaderRequest_RequestType = 4
	DownloaderRequest_REGISTERTIMEOUT DownloaderRequest_RequestType = 5
	DownloaderRequest_UNKNOWN         DownloaderRequest_RequestType = 6
)

var DownloaderRequest_RequestType_name = map[int32]string{
	0: "HEADER",
	1: "BLOCK",
	2: "NEWBLOCK",
	3: "BLOCKHEIGHT",
	4: "REGISTER",
	5: "REGISTERTIMEOUT",
	6: "UNKNOWN",
}

var DownloaderRequest_RequestType_value = map[string]int32{
	"HEADER":          0,
	"BLOCK":           1,
	"NEWBLOCK":        2,
	"BLOCKHEIGHT":     3,
	"REGISTER":        4,
	"REGISTERTIMEOUT": 5,
	"UNKNOWN":         6,
}

func (x DownloaderRequest_RequestType) String() string {
	return proto.EnumName(DownloaderRequest_RequestType_name, int32(x))
}

func (DownloaderRequest_RequestType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_6a99ec95c7ab1ff1, []int{0, 0}
}

type DownloaderResponse_RegisterResponseType int32

const (
	DownloaderResponse_SUCCESS DownloaderResponse_RegisterResponseType = 0
	DownloaderResponse_FAIL    DownloaderResponse_RegisterResponseType = 1
	DownloaderResponse_INSYNC  DownloaderResponse_RegisterResponseType = 2
)

var DownloaderResponse_RegisterResponseType_name = map[int32]string{
	0: "SUCCESS",
	1: "FAIL",
	2: "INSYNC",
}

var DownloaderResponse_RegisterResponseType_value = map[string]int32{
	"SUCCESS": 0,
	"FAIL":    1,
	"INSYNC":  2,
}

func (x DownloaderResponse_RegisterResponseType) String() string {
	return proto.EnumName(DownloaderResponse_RegisterResponseType_name, int32(x))
}

func (DownloaderResponse_RegisterResponseType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_6a99ec95c7ab1ff1, []int{1, 0}
}

// DownloaderRequest is the generic download request.
type DownloaderRequest struct {
	// Request type.
	Type DownloaderRequest_RequestType `protobuf:"varint,1,opt,name=type,proto3,enum=downloader.DownloaderRequest_RequestType" json:"type,omitempty"`
	// The hashes of the blocks we want to download.
	Hashes               [][]byte `protobuf:"bytes,2,rep,name=hashes,proto3" json:"hashes,omitempty"`
	PeerHash             []byte   `protobuf:"bytes,3,opt,name=peerHash,proto3" json:"peerHash,omitempty"`
	BlockHash            []byte   `protobuf:"bytes,4,opt,name=blockHash,proto3" json:"blockHash,omitempty"`
	Ip                   string   `protobuf:"bytes,5,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 string   `protobuf:"bytes,6,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DownloaderRequest) Reset()         { *m = DownloaderRequest{} }
func (m *DownloaderRequest) String() string { return proto.CompactTextString(m) }
func (*DownloaderRequest) ProtoMessage()    {}
func (*DownloaderRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_6a99ec95c7ab1ff1, []int{0}
}

func (m *DownloaderRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DownloaderRequest.Unmarshal(m, b)
}
func (m *DownloaderRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DownloaderRequest.Marshal(b, m, deterministic)
}
func (m *DownloaderRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DownloaderRequest.Merge(m, src)
}
func (m *DownloaderRequest) XXX_Size() int {
	return xxx_messageInfo_DownloaderRequest.Size(m)
}
func (m *DownloaderRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DownloaderRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DownloaderRequest proto.InternalMessageInfo

func (m *DownloaderRequest) GetType() DownloaderRequest_RequestType {
	if m != nil {
		return m.Type
	}
	return DownloaderRequest_HEADER
}

func (m *DownloaderRequest) GetHashes() [][]byte {
	if m != nil {
		return m.Hashes
	}
	return nil
}

func (m *DownloaderRequest) GetPeerHash() []byte {
	if m != nil {
		return m.PeerHash
	}
	return nil
}

func (m *DownloaderRequest) GetBlockHash() []byte {
	if m != nil {
		return m.BlockHash
	}
	return nil
}

func (m *DownloaderRequest) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *DownloaderRequest) GetPort() string {
	if m != nil {
		return m.Port
	}
	return ""
}

// DownloaderResponse is the generic response of DownloaderRequest.
type DownloaderResponse struct {
	// payload of Block.
	Payload [][]byte `protobuf:"bytes,1,rep,name=payload,proto3" json:"payload,omitempty"`
	// response of registration request
	Type                 DownloaderResponse_RegisterResponseType `protobuf:"varint,2,opt,name=type,proto3,enum=downloader.DownloaderResponse_RegisterResponseType" json:"type,omitempty"`
	BlockHeight          uint64                                  `protobuf:"varint,3,opt,name=blockHeight,proto3" json:"blockHeight,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                                `json:"-"`
	XXX_unrecognized     []byte                                  `json:"-"`
	XXX_sizecache        int32                                   `json:"-"`
}

func (m *DownloaderResponse) Reset()         { *m = DownloaderResponse{} }
func (m *DownloaderResponse) String() string { return proto.CompactTextString(m) }
func (*DownloaderResponse) ProtoMessage()    {}
func (*DownloaderResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_6a99ec95c7ab1ff1, []int{1}
}

func (m *DownloaderResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DownloaderResponse.Unmarshal(m, b)
}
func (m *DownloaderResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DownloaderResponse.Marshal(b, m, deterministic)
}
func (m *DownloaderResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DownloaderResponse.Merge(m, src)
}
func (m *DownloaderResponse) XXX_Size() int {
	return xxx_messageInfo_DownloaderResponse.Size(m)
}
func (m *DownloaderResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_DownloaderResponse.DiscardUnknown(m)
}

var xxx_messageInfo_DownloaderResponse proto.InternalMessageInfo

func (m *DownloaderResponse) GetPayload() [][]byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *DownloaderResponse) GetType() DownloaderResponse_RegisterResponseType {
	if m != nil {
		return m.Type
	}
	return DownloaderResponse_SUCCESS
}

func (m *DownloaderResponse) GetBlockHeight() uint64 {
	if m != nil {
		return m.BlockHeight
	}
	return 0
}

func init() {
	proto.RegisterEnum("downloader.DownloaderRequest_RequestType", DownloaderRequest_RequestType_name, DownloaderRequest_RequestType_value)
	proto.RegisterEnum("downloader.DownloaderResponse_RegisterResponseType", DownloaderResponse_RegisterResponseType_name, DownloaderResponse_RegisterResponseType_value)
	proto.RegisterType((*DownloaderRequest)(nil), "downloader.DownloaderRequest")
	proto.RegisterType((*DownloaderResponse)(nil), "downloader.DownloaderResponse")
}

func init() { proto.RegisterFile("downloader.proto", fileDescriptor_6a99ec95c7ab1ff1) }

var fileDescriptor_6a99ec95c7ab1ff1 = []byte{
	// 388 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x92, 0xdf, 0x6f, 0x94, 0x40,
	0x10, 0xc7, 0x6f, 0x39, 0xa0, 0x77, 0xc3, 0xa5, 0x5d, 0x47, 0x63, 0x48, 0xa3, 0x86, 0xf0, 0x84,
	0x2f, 0x3c, 0xb4, 0x4f, 0x3e, 0xf8, 0x50, 0xe9, 0x7a, 0x90, 0x56, 0x2e, 0x2e, 0x9c, 0x8d, 0x8f,
	0xd4, 0x6e, 0x0a, 0xb1, 0x29, 0x2b, 0x4b, 0x63, 0xf8, 0xe3, 0xfc, 0x2f, 0xfc, 0x83, 0x0c, 0x7b,
	0x3f, 0x20, 0x51, 0xef, 0x69, 0xe7, 0xfb, 0x9d, 0x9d, 0xc9, 0xcc, 0x27, 0x03, 0xf4, 0xae, 0xfe,
	0xf9, 0xf8, 0x50, 0x17, 0x77, 0xa2, 0x09, 0x65, 0x53, 0xb7, 0x35, 0xc2, 0xe0, 0xf8, 0xbf, 0x0c,
	0x78, 0x76, 0xb9, 0x97, 0x5c, 0xfc, 0x78, 0x12, 0xaa, 0xc5, 0xf7, 0x60, 0xb6, 0x9d, 0x14, 0x2e,
	0xf1, 0x48, 0x70, 0x7c, 0xf6, 0x36, 0x1c, 0xb5, 0xf8, 0xeb, 0x73, 0xb8, 0x7d, 0xf3, 0x4e, 0x0a,
	0xae, 0xcb, 0xf0, 0x25, 0xd8, 0x65, 0xa1, 0x4a, 0xa1, 0x5c, 0xc3, 0x9b, 0x06, 0x0b, 0xbe, 0x55,
	0x78, 0x0a, 0x33, 0x29, 0x44, 0x13, 0x17, 0xaa, 0x74, 0xa7, 0x1e, 0x09, 0x16, 0x7c, 0xaf, 0xf1,
	0x15, 0xcc, 0x6f, 0x1f, 0xea, 0x6f, 0xdf, 0x75, 0xd2, 0xd4, 0xc9, 0xc1, 0xc0, 0x63, 0x30, 0x2a,
	0xe9, 0x5a, 0x1e, 0x09, 0xe6, 0xdc, 0xa8, 0x24, 0x22, 0x98, 0xb2, 0x6e, 0x5a, 0xd7, 0xd6, 0x8e,
	0x8e, 0x7d, 0x05, 0xce, 0x68, 0x14, 0x04, 0xb0, 0x63, 0x76, 0x71, 0xc9, 0x38, 0x9d, 0xe0, 0x1c,
	0xac, 0x0f, 0xd7, 0xab, 0xe8, 0x8a, 0x12, 0x5c, 0xc0, 0x2c, 0x65, 0x37, 0x1b, 0x65, 0xe0, 0x09,
	0x38, 0x3a, 0x8c, 0x59, 0xb2, 0x8c, 0x73, 0x3a, 0xed, 0xd3, 0x9c, 0x2d, 0x93, 0x2c, 0x67, 0x9c,
	0x9a, 0xf8, 0x1c, 0x4e, 0x76, 0x2a, 0x4f, 0x3e, 0xb1, 0xd5, 0x3a, 0xa7, 0x16, 0x3a, 0x70, 0xb4,
	0x4e, 0xaf, 0xd2, 0xd5, 0x4d, 0x4a, 0x6d, 0xff, 0x37, 0x01, 0x1c, 0x23, 0x51, 0xb2, 0x7e, 0x54,
	0x02, 0x5d, 0x38, 0x92, 0x45, 0xd7, 0x9b, 0x2e, 0xd1, 0x08, 0x76, 0x12, 0x97, 0x5b, 0xb4, 0x86,
	0x46, 0x7b, 0xfe, 0x3f, 0xb4, 0x9b, 0x3e, 0x21, 0x17, 0xf7, 0x95, 0x6a, 0x07, 0x63, 0x04, 0xd9,
	0x03, 0x67, 0xc3, 0x47, 0x54, 0xf7, 0x65, 0xab, 0x79, 0x9a, 0x7c, 0x6c, 0xf9, 0xef, 0xe0, 0xc5,
	0xbf, 0xea, 0xfb, 0x05, 0xb2, 0x75, 0x14, 0xb1, 0x2c, 0xa3, 0x13, 0x9c, 0x81, 0xf9, 0xf1, 0x22,
	0xb9, 0xa6, 0xa4, 0x07, 0x96, 0xa4, 0xd9, 0xd7, 0x34, 0xa2, 0xc6, 0xd9, 0x17, 0x80, 0x61, 0x1a,
	0x8c, 0xc1, 0xfa, 0xfc, 0x24, 0x9a, 0x0e, 0x5f, 0x1f, 0xbc, 0x84, 0xd3, 0x37, 0x87, 0xb7, 0xf1,
	0x27, 0xb7, 0xb6, 0xbe, 0xc0, 0xf3, 0x3f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x45, 0x42, 0x00, 0x51,
	0x95, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// DownloaderClient is the client API for Downloader service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type DownloaderClient interface {
	Query(ctx context.Context, in *DownloaderRequest, opts ...grpc.CallOption) (*DownloaderResponse, error)
}

type downloaderClient struct {
	cc *grpc.ClientConn
}

func NewDownloaderClient(cc *grpc.ClientConn) DownloaderClient {
	return &downloaderClient{cc}
}

func (c *downloaderClient) Query(ctx context.Context, in *DownloaderRequest, opts ...grpc.CallOption) (*DownloaderResponse, error) {
	out := new(DownloaderResponse)
	err := c.cc.Invoke(ctx, "/downloader.Downloader/Query", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DownloaderServer is the server API for Downloader service.
type DownloaderServer interface {
	Query(context.Context, *DownloaderRequest) (*DownloaderResponse, error)
}

func RegisterDownloaderServer(s *grpc.Server, srv DownloaderServer) {
	s.RegisterService(&_Downloader_serviceDesc, srv)
}

func _Downloader_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DownloaderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DownloaderServer).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/downloader.Downloader/Query",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DownloaderServer).Query(ctx, req.(*DownloaderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Downloader_serviceDesc = grpc.ServiceDesc{
	ServiceName: "downloader.Downloader",
	HandlerType: (*DownloaderServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Query",
			Handler:    _Downloader_Query_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "downloader.proto",
}
