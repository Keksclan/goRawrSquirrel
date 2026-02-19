// Package main demonstrates goRawrSquirrel's global rate limiting by starting
// a real gRPC server, calling a method repeatedly, and printing when the rate
// limit triggers.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	gs "github.com/Keksclan/goRawrSquirrel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// echoService is a minimal gRPC service whose only method returns successfully.
type echoService struct{}

// serviceDesc describes a minimal gRPC service with a single unary method
// registered via the generic handler path so we can invoke it without
// generated protobuf stubs.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: "example.EchoService",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Echo",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(echoMsg)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor != nil {
					return interceptor(ctx, in, &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/example.EchoService/Echo",
					}, func(_ context.Context, _ any) (any, error) {
						return in, nil
					})
				}
				return in, nil
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "example.proto",
}

// echoMsg is a trivial message type that satisfies the proto codec by
// implementing ProtoMessage, Reset, and String.
type echoMsg struct{}

func (echoMsg) ProtoMessage()            {}
func (echoMsg) Reset()                   {}
func (echoMsg) String() string           { return "" }
func (echoMsg) Marshal() ([]byte, error) { return []byte{}, nil }
func (*echoMsg) Unmarshal([]byte) error  { return nil }

func main() {
	// Create a goRawrSquirrel server with recovery and a global rate limit of
	// 2 requests per second with a burst of 3.
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithRateLimitGlobal(2, 3),
	)

	// Register our echo service.
	srv.GRPC().RegisterService(&serviceDesc, &echoService{})

	// Listen on a random available port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	// Start serving in the background.
	go func() {
		if err := srv.GRPC().Serve(lis); err != nil {
			// Server stopped.
		}
	}()
	defer srv.GRPC().Stop()

	// Dial the server.
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Fire 6 requests rapidly — the first 3 (burst) should succeed, the rest
	// should be rejected with codes.ResourceExhausted.
	for i := range 6 {
		var reply echoMsg
		err = conn.Invoke(context.Background(), "/example.EchoService/Echo", &echoMsg{}, &reply)
		st, _ := status.FromError(err)

		if st.Code() == codes.ResourceExhausted {
			fmt.Printf("request %d: rate limited (%s)\n", i+1, st.Message())
		} else if st.Code() == codes.OK {
			fmt.Printf("request %d: OK\n", i+1)
		} else {
			fmt.Printf("request %d: unexpected code %v\n", i+1, st.Code())
		}
	}
}
