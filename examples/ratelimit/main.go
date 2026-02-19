// Package main demonstrates goRawrSquirrel's IP blocking and per-group/global
// rate limiting by starting a real gRPC server with two RPC methods:
//
//   - /example.EchoService/Heavy — limited to 1 req/s (burst 1) via a policy group
//   - /example.EchoService/Light — uses the global limit (10 req/s, burst 5)
//
// It also shows how an IP blocker denies requests from disallowed addresses
// by simulating a blocked IP via a fake peer context.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// echoService is a minimal gRPC service.
type echoService struct{}

// echoMsg is a trivial message type that satisfies the proto codec by
// implementing ProtoMessage, Reset, and String.
type echoMsg struct{}

func (echoMsg) ProtoMessage()            {}
func (echoMsg) Reset()                   {}
func (echoMsg) String() string           { return "" }
func (echoMsg) Marshal() ([]byte, error) { return []byte{}, nil }
func (*echoMsg) Unmarshal([]byte) error  { return nil }

// makeHandler builds a grpc.MethodDesc handler for the given full method name.
func makeHandler(fullMethod string) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		in := new(echoMsg)
		if err := dec(in); err != nil {
			return nil, err
		}
		if interceptor != nil {
			return interceptor(ctx, in, &grpc.UnaryServerInfo{
				Server:     srv,
				FullMethod: fullMethod,
			}, func(_ context.Context, _ any) (any, error) {
				return in, nil
			})
		}
		return in, nil
	}
}

// serviceDesc describes a minimal gRPC service with two unary methods.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: "example.EchoService",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Heavy", Handler: makeHandler("/example.EchoService/Heavy")},
		{MethodName: "Light", Handler: makeHandler("/example.EchoService/Light")},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "example.proto",
}

func main() {
	// Define a policy group: /example.EchoService/Heavy is limited to
	// 1 request per second with a burst of 1.
	resolver := policy.NewResolver(
		policy.Group("heavy-ops").
			Exact("/example.EchoService/Heavy").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 1, Window: time.Second},
			}),
	)

	// Create an IP blocker that only allows the 127.0.0.0/8 range.
	// Any request from outside that range will be rejected with
	// codes.PermissionDenied before the rate limiter even runs.
	blocker, err := security.NewIPBlocker(security.Config{
		Mode:  security.AllowList,
		CIDRs: []string{"127.0.0.0/8"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create IP blocker: %v\n", err)
		os.Exit(1)
	}

	// Create a goRawrSquirrel server with recovery, IP blocking, the
	// resolver, and a generous global rate limit (10 rps, burst 5).
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithIPBlocker(blocker),
		gs.WithResolver(resolver),
		gs.WithRateLimitGlobal(10, 5),
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

	// --- Demonstrate IP blocking ---
	// Simulate a request from a blocked IP (10.0.0.50) by injecting a fake
	// peer into the outgoing context via a unary client interceptor.
	fmt.Println("=== Blocked IP simulation (10.0.0.50 → should be PermissionDenied) ===")
	blockedConn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(fakePeerInterceptor("10.0.0.50:9999")),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial (blocked): %v\n", err)
		os.Exit(1)
	}
	{
		var reply echoMsg
		err = blockedConn.Invoke(context.Background(), "/example.EchoService/Light", &echoMsg{}, &reply)
		st, _ := status.FromError(err)
		fmt.Printf("  result: %s (%s)\n", st.Code(), st.Message())
	}
	blockedConn.Close()

	fmt.Println("=== Heavy method (per-group limit: 1 rps, burst 1) ===")
	for i := range 4 {
		var reply echoMsg
		err = conn.Invoke(context.Background(), "/example.EchoService/Heavy", &echoMsg{}, &reply)
		st, _ := status.FromError(err)
		if st.Code() == codes.ResourceExhausted {
			fmt.Printf("  request %d: rate limited (%s)\n", i+1, st.Message())
		} else if st.Code() == codes.OK {
			fmt.Printf("  request %d: OK\n", i+1)
		} else {
			fmt.Printf("  request %d: unexpected code %v\n", i+1, st.Code())
		}
	}

	fmt.Println("=== Light method (global limit: 10 rps, burst 5) ===")
	for i := range 4 {
		var reply echoMsg
		err = conn.Invoke(context.Background(), "/example.EchoService/Light", &echoMsg{}, &reply)
		st, _ := status.FromError(err)
		if st.Code() == codes.ResourceExhausted {
			fmt.Printf("  request %d: rate limited (%s)\n", i+1, st.Message())
		} else if st.Code() == codes.OK {
			fmt.Printf("  request %d: OK\n", i+1)
		} else {
			fmt.Printf("  request %d: unexpected code %v\n", i+1, st.Code())
		}
	}
}

// fakePeerAddr implements net.Addr to simulate an arbitrary client IP.
type fakePeerAddr struct{ addr string }

func (f fakePeerAddr) Network() string { return "tcp" }
func (f fakePeerAddr) String() string  { return f.addr }

// fakePeerInterceptor returns a unary client interceptor that injects a fake
// peer address into the context. This is only useful for demonstrations; in
// production the peer address is set by the transport layer.
func fakePeerInterceptor(addr string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = peer.NewContext(ctx, &peer.Peer{
			Addr: fakePeerAddr{addr: addr},
		})
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
