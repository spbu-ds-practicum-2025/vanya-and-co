// Code generated manually
package authpb

import (
	context "context"

	grpc "google.golang.org/grpc"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

var _ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
var _ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)

type WhoAmIRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
	Token         string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
}

func (x *WhoAmIRequest) Reset() {
	*x = WhoAmIRequest{}
}

func (x *WhoAmIRequest) String() string {
	return "WhoAmIRequest"
}

func (*WhoAmIRequest) ProtoMessage() {}

func (x *WhoAmIRequest) ProtoReflect() protoreflect.Message {
	return nil
}

type WhoAmIResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
	Username      string `protobuf:"bytes,1,opt,name=username,proto3" json:"username,omitempty"`
}

func (x *WhoAmIResponse) Reset() {
	*x = WhoAmIResponse{}
}

func (x *WhoAmIResponse) String() string {
	return "WhoAmIResponse"
}

func (*WhoAmIResponse) ProtoMessage() {}

func (x *WhoAmIResponse) ProtoReflect() protoreflect.Message {
	return nil
}

// client API
type AuthClient interface {
	WhoAmI(ctx context.Context, in *WhoAmIRequest, opts ...grpc.CallOption) (*WhoAmIResponse, error)
}

type authClient struct {
	cc grpc.ClientConnInterface
}

func NewAuthClient(cc grpc.ClientConnInterface) AuthClient {
	return &authClient{cc}
}

func (c *authClient) WhoAmI(ctx context.Context, in *WhoAmIRequest, opts ...grpc.CallOption) (*WhoAmIResponse, error) {
	out := new(WhoAmIResponse)
	if err := c.cc.Invoke(ctx, "/auth.Auth/WhoAmI", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

// server API
type AuthServer interface {
	WhoAmI(context.Context, *WhoAmIRequest) (*WhoAmIResponse, error)
}

func RegisterAuthServer(s *grpc.Server, srv AuthServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "auth.Auth",
		HandlerType: (*AuthServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "WhoAmI",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					in := new(WhoAmIRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return srv.(AuthServer).WhoAmI(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/auth.Auth/WhoAmI",
					}
					handler := func(ctx context.Context, req interface{}) (interface{}, error) {
						return srv.(AuthServer).WhoAmI(ctx, req.(*WhoAmIRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "auth.proto",
	}, srv)
}
