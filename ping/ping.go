// Package ping provides a minimal built-in Ping RPC suitable for health
// checks and demos. It uses [grpc.ServiceDesc] registration so that no
// protobuf code generation is required.
//
// Because the request/response types are plain Go structs (not generated
// protobuf messages), the package registers a thin codec wrapper that
// JSON-encodes Ping types while delegating all other messages to the
// standard proto codec. Import this package (or call [Register]) to
// activate the codec automatically.
package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"google.golang.org/grpc"
	grpcEncoding "google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // ensure default proto codec is registered first
	"google.golang.org/protobuf/proto"
)

// PingRequest is the input for the Ping method.
type PingRequest struct {
	Message string `json:"message"`
}

// PingResponse is the output of the Ping method.
type PingResponse struct {
	Message        string `json:"message"`
	ServerTimeUnix int64  `json:"server_time_unix"`
}

// pingMsg is a marker interface satisfied by PingRequest and PingResponse.
type pingMsg interface {
	isPingMsg()
}

func (*PingRequest) isPingMsg()  {}
func (*PingResponse) isPingMsg() {}

// Handler is the interface that a Ping service implementation must satisfy.
type Handler interface {
	Ping(ctx context.Context, req *PingRequest) (*PingResponse, error)
}

// DefaultHandler returns a Handler that echoes the request message and
// attaches the current server time.
func DefaultHandler() Handler { return defaultHandler{} }

type defaultHandler struct{}

func (defaultHandler) Ping(_ context.Context, req *PingRequest) (*PingResponse, error) {
	return &PingResponse{
		Message:        req.Message,
		ServerTimeUnix: time.Now().Unix(),
	}, nil
}

// funMessages is the pool of fun responses used when FunMode is enabled.
var funMessages = []string{
	"Squirrel power!",
	"Nom nom nom acorns!",
	"Tail flick activated!",
	"Scurry mode engaged!",
	"Nuts about this request!",
}

// FunHandler returns a Handler that, when FunMode is enabled, occasionally
// (1 in 5 chance) replaces the echoed message with a fun response chosen
// from an internal list. When the random check does not trigger, the
// request message is echoed normally.
//
// src may be nil; in that case a time-seeded source is used.
func FunHandler(src rand.Source) Handler {
	if src == nil {
		src = rand.NewSource(time.Now().UnixNano())
	}
	return funHandler{rng: rand.New(src)}
}

type funHandler struct {
	rng *rand.Rand
}

func (h funHandler) Ping(_ context.Context, req *PingRequest) (*PingResponse, error) {
	msg := req.Message
	if h.rng.Intn(5) == 0 {
		msg = funMessages[h.rng.Intn(len(funMessages))]
	}
	return &PingResponse{
		Message:        msg,
		ServerTimeUnix: time.Now().Unix(),
	}, nil
}

// ServiceDesc is the grpc.ServiceDesc for the rawr.Ping service.
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: "rawr.Ping",
	HandlerType: (*Handler)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    pingHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "rawr/ping.proto",
}

func pingHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(PingRequest)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(Handler).Ping(ctx, req)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rawr.Ping/Ping",
	}
	handler := func(ctx context.Context, r any) (any, error) {
		return srv.(Handler).Ping(ctx, r.(*PingRequest))
	}
	return interceptor(ctx, req, info, handler)
}

// Register registers a Ping service implementation on the given gRPC server.
func Register(s *grpc.Server, h Handler) {
	s.RegisterService(&ServiceDesc, h)
}

// ---------- codec wrapper ----------

func init() {
	// Replace the default proto codec with a thin wrapper that JSON-encodes
	// ping types and delegates all other (protobuf) messages to proto.Marshal.
	grpcEncoding.RegisterCodec(pingCodec{})
}

// pingCodec wraps the default proto codec. It handles PingRequest and
// PingResponse via JSON, and delegates all other types to proto.Marshal/Unmarshal.
type pingCodec struct{}

func (pingCodec) Name() string { return "proto" }

func (pingCodec) Marshal(v any) ([]byte, error) {
	if _, ok := v.(pingMsg); ok {
		return json.Marshal(v)
	}
	if m, ok := v.(proto.Message); ok {
		return proto.Marshal(m)
	}
	return nil, fmt.Errorf("ping codec: unsupported message type %T", v)
}

func (pingCodec) Unmarshal(data []byte, v any) error {
	if _, ok := v.(pingMsg); ok {
		return json.Unmarshal(data, v)
	}
	if m, ok := v.(proto.Message); ok {
		return proto.Unmarshal(data, m)
	}
	return fmt.Errorf("ping codec: unsupported message type %T", v)
}
