// Package main demonstrates goRawrSquirrel's token-bucket rate limiting.
//
// It starts a gRPC server with a tight global rate limit (2 req/s, burst 2)
// and fires several requests in rapid succession, showing which ones succeed
// and which are rejected with codes.ResourceExhausted.
//
// Run:
//
//	go run ./examples/ratelimit-demo
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

// ── tiny proto-compatible message ──────────────────────────────────────

type msg struct{}

func (msg) ProtoMessage()            {}
func (msg) Reset()                   {}
func (msg) String() string           { return "" }
func (msg) Marshal() ([]byte, error) { return []byte{}, nil }
func (*msg) Unmarshal([]byte) error  { return nil }

// ── service definition ─────────────────────────────────────────────────

var serviceDesc = grpc.ServiceDesc{
	ServiceName: "demo.PingService",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(msg)
				if err := dec(in); err != nil {
					return nil, err
				}
				handler := func(_ context.Context, _ any) (any, error) {
					return &msg{}, nil
				}
				if interceptor != nil {
					return interceptor(ctx, in, &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/demo.PingService/Ping",
					}, handler)
				}
				return handler(ctx, in)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "demo.proto",
}

// ── main ───────────────────────────────────────────────────────────────

func main() {
	fmt.Println("ratelimit-demo: demonstrates goRawrSquirrel token-bucket rate limiting")
	fmt.Println()

	// 1. Build a server with a very tight global rate limit so the demo
	//    triggers throttling quickly: 2 req/s sustained, burst of 2.
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithRateLimitGlobal(2, 2),
	)
	srv.GRPC().RegisterService(&serviceDesc, struct{}{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	go func() { _ = srv.GRPC().Serve(lis) }()
	defer srv.GRPC().Stop()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Fire 6 requests back-to-back; some will succeed, the rest will be
	//    rejected with ResourceExhausted once the bucket is drained.
	fmt.Println("── sending 6 rapid requests (limit: 2 req/s, burst 2) ──")
	var accepted, rejected int
	for i := range 6 {
		var reply msg
		err = conn.Invoke(context.Background(), "/demo.PingService/Ping", &msg{}, &reply)
		st, _ := status.FromError(err)

		switch st.Code() {
		case codes.OK:
			fmt.Printf("  request %d: OK\n", i+1)
			accepted++
		case codes.ResourceExhausted:
			fmt.Printf("  request %d: rate limited (%s)\n", i+1, st.Message())
			rejected++
		default:
			fmt.Printf("  request %d: unexpected code %s\n", i+1, st.Code())
		}
	}

	fmt.Println()
	fmt.Printf("summary: %d accepted, %d rejected\n", accepted, rejected)

	if rejected == 0 {
		fmt.Fprintln(os.Stderr, "expected at least one request to be rate-limited")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ ratelimit-demo complete — excess requests correctly rejected with ResourceExhausted")
}
