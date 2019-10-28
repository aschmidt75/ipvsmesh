// Code generated by protoc-gen-go. DO NOT EDIT.
// source: cli.proto

package localinterface

import (
	context "context"
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
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

type Empty struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Empty) Reset()         { *m = Empty{} }
func (m *Empty) String() string { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()    {}
func (*Empty) Descriptor() ([]byte, []int) {
	return fileDescriptor_81159ba547ea6f30, []int{0}
}

func (m *Empty) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Empty.Unmarshal(m, b)
}
func (m *Empty) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Empty.Marshal(b, m, deterministic)
}
func (m *Empty) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Empty.Merge(m, src)
}
func (m *Empty) XXX_Size() int {
	return xxx_messageInfo_Empty.Size(m)
}
func (m *Empty) XXX_DiscardUnknown() {
	xxx_messageInfo_Empty.DiscardUnknown(m)
}

var xxx_messageInfo_Empty proto.InternalMessageInfo

func init() {
	proto.RegisterType((*Empty)(nil), "localinterface.Empty")
}

func init() { proto.RegisterFile("cli.proto", fileDescriptor_81159ba547ea6f30) }

var fileDescriptor_81159ba547ea6f30 = []byte{
	// 101 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4c, 0xce, 0xc9, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0xcb, 0xc9, 0x4f, 0x4e, 0xcc, 0xc9, 0xcc, 0x2b, 0x49,
	0x2d, 0x4a, 0x4b, 0x4c, 0x4e, 0x55, 0x62, 0xe7, 0x62, 0x75, 0xcd, 0x2d, 0x28, 0xa9, 0x34, 0x72,
	0xe5, 0xe2, 0x75, 0x49, 0x4c, 0xcd, 0xcd, 0xcf, 0x0b, 0x4e, 0x2d, 0x2a, 0xcb, 0x4c, 0x4e, 0x15,
	0x32, 0xe1, 0x62, 0x09, 0x2e, 0xc9, 0x2f, 0x10, 0x12, 0xd5, 0x43, 0xd5, 0xa2, 0x07, 0x56, 0x2f,
	0x85, 0x5d, 0x38, 0x89, 0x0d, 0x6c, 0x8d, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0x3f, 0xcb, 0x23,
	0xca, 0x73, 0x00, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// DaemonServiceClient is the client API for DaemonService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type DaemonServiceClient interface {
	Stop(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
}

type daemonServiceClient struct {
	cc *grpc.ClientConn
}

func NewDaemonServiceClient(cc *grpc.ClientConn) DaemonServiceClient {
	return &daemonServiceClient{cc}
}

func (c *daemonServiceClient) Stop(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/localinterface.DaemonService/Stop", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DaemonServiceServer is the server API for DaemonService service.
type DaemonServiceServer interface {
	Stop(context.Context, *Empty) (*Empty, error)
}

func RegisterDaemonServiceServer(s *grpc.Server, srv DaemonServiceServer) {
	s.RegisterService(&_DaemonService_serviceDesc, srv)
}

func _DaemonService_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DaemonServiceServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/localinterface.DaemonService/Stop",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DaemonServiceServer).Stop(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _DaemonService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "localinterface.DaemonService",
	HandlerType: (*DaemonServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Stop",
			Handler:    _DaemonService_Stop_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "cli.proto",
}
